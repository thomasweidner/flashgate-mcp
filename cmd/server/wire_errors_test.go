package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/benchmark"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
)

const wirePrivateMarker = "private-wire-fixture-cause"
const wireInvalidTextBytes = wirePrivateMarker + "\xff"

type mcpErrorCase struct {
	arguments    string
	reason       string
	setup        func(string) error
	backendError error
	tightLimits  bool
	copiedBytes  uint64
}

// The path-policy denial is already part of the conformance snapshots. These
// rows cover the other active BL-203 categories and require an explicit reason
// when a category is inapplicable.
var mcpAdditionalErrorCategories = []string{
	"invalid_arguments", "not_found", "already_exists", "access_denied",
	"unsupported_path_type", "unsupported_operation", "invalid_text_encoding", "limit_exceeded",
}

func mcpFilesystemErrors(validArguments string, cases map[string]mcpErrorCase) map[string]mcpErrorCase {
	cases["invalid_arguments"] = mcpErrorCase{arguments: `{"unexpected":true}`}
	cases["access_denied"] = mcpErrorCase{arguments: validArguments, backendError: os.ErrPermission}
	if _, ok := cases["unsupported_operation"]; !ok {
		cases["unsupported_operation"] = mcpErrorCase{reason: "The current filesystem adapter emits this category for cross-volume move only."}
	}
	if _, ok := cases["invalid_text_encoding"]; !ok {
		cases["invalid_text_encoding"] = mcpErrorCase{reason: "This operation does not decode on-disk file bytes as UTF-8."}
	}
	return cases
}

func mcpErrorCoverage(cases map[string]mcpErrorCase) error {
	for _, category := range mcpAdditionalErrorCategories {
		test, ok := cases[category]
		if !ok {
			return fmt.Errorf("missing error applicability for %s", category)
		}
		if test.arguments == "" {
			if strings.TrimSpace(test.reason) == "" || category == "access_denied" || category == "invalid_arguments" {
				return fmt.Errorf("missing applicable error fixture for %s", category)
			}
			if test.setup != nil || test.backendError != nil || test.tightLimits || test.copiedBytes != 0 {
				return fmt.Errorf("inapplicable error %s has executable fixture state", category)
			}
		} else if test.reason != "" || !json.Valid([]byte(test.arguments)) || test.arguments[0] != '{' {
			return fmt.Errorf("invalid error fixture for %s", category)
		}
	}
	if len(cases) != len(mcpAdditionalErrorCategories) {
		return fmt.Errorf("unknown or inactive error applicability category")
	}
	return nil
}

func wireDirectory(root string) error {
	return os.Mkdir(filepath.Join(root, "directory"), 0o700)
}

func wireNonemptyDirectory(root string) error {
	if err := wireDirectory(root); err != nil {
		return err
	}
	for _, name := range []string{"first.txt", "second.txt"} {
		if err := os.WriteFile(filepath.Join(root, "directory", name), nil, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func wireSecondFile(root string) error {
	return os.WriteFile(filepath.Join(root, "second.txt"), nil, 0o600)
}

func wireInvalidText(root string) error {
	return os.WriteFile(filepath.Join(root, "invalid.txt"), []byte(wireInvalidTextBytes), 0o600)
}

// Inject the OS failure at the filesystem interface so the real registered
// adapter classifies it. This keeps permission and cross-volume cases stable
// without changing ACLs, execution identity or host mount layout.
type wireFailingFilesystem struct{ err error }

func (f wireFailingFilesystem) List(string) ([]fs.Entry, error)    { return nil, f.err }
func (f wireFailingFilesystem) Read(string, int64) ([]byte, error) { return nil, f.err }
func (f wireFailingFilesystem) Stat(string) (fs.Metadata, error)   { return fs.Metadata{}, f.err }
func (f wireFailingFilesystem) Write(string, []byte, bool) error   { return f.err }
func (f wireFailingFilesystem) Mkdir(string) (bool, error)         { return false, f.err }
func (f wireFailingFilesystem) Delete(string, bool) error          { return f.err }
func (f wireFailingFilesystem) Copy(string, string, bool) error    { return f.err }
func (f wireFailingFilesystem) Move(string, string, bool) error    { return f.err }

func wireExpectedToolError(id benchmark.WireIdentity, category string, test mcpErrorCase, template json.RawMessage) (serverWireCase, error) {
	id.Variant, id.Fixture, id.Outcome = "error", category, "error"
	id.Contract = "result/" + category
	message := "filesystem error: " + strings.ReplaceAll(category, "_", " ")
	switch category {
	case "invalid_arguments":
		message = "invalid arguments"
	case "invalid_text_encoding":
		message = "file text is not valid UTF-8"
	}
	// Build the expectation independently of the production error constructor.
	text := fmt.Sprintf(`{"category":%q,"message":%q}`, category, message)
	want, err := replaceWireBody(template, map[string]any{
		"content": []map[string]string{{"type": "text", "text": text}},
		"isError": true,
	})
	if err != nil {
		return serverWireCase{}, err
	}
	copies := uint64(1)
	if test.copiedBytes > 0 {
		copies++
	}
	return serverWireCase{
		id: id, args: test.arguments, want: want, setup: test.setup, encode: 2,
		fields: append(revisionWireFields(want), wireField(benchmark.WireDomain, "content", "0", "text")),
		copies: copies, copyBytes: uint64(len(text)) + test.copiedBytes,
		tightLimits: test.tightLimits, backendError: test.backendError,
	}, nil
}

const wireInvalidParams = `{"code":-32602,"message":"invalid params"}`
const wireMethodNotFound = `{"code":-32601,"message":"method not found"}`
const wireInternalError = `{"code":-32603,"message":"internal error"}`

func wireRPCError(base benchmark.WireIdentity, route, name, args, want string) serverWireCase {
	row := wireControlCase(base, route, name, args, json.RawMessage(want))
	row.id.Outcome, row.body = "error", "error"
	return row
}

func wireRPCErrors(base benchmark.WireIdentity, cases map[string]mcpToolCase) []serverWireCase {
	ioFailure := wireRPCError(base, "tools/call", "internal-io", `{"name":"read_file","arguments":{"path":"seed.txt"}}`, wireInternalError)
	ioFailure.backendError = io.ErrUnexpectedEOF
	badResult := wireRPCError(base, "tools/call", "internal-serialization", `{"name":"read_file","arguments":{"path":"seed.txt"}}`, wireInternalError)
	badResult.registry = func(registry *tools.Registry) error {
		tool, ok := registry.Get("read_file")
		if !ok {
			return fmt.Errorf("missing read_file serialization fixture")
		}
		return registry.Register(wireInvalidResultTool{tool})
	}
	invalidParams := wireRPCError(base, "tools/call", "invalid-outer-params", `{"name":"read_file","arguments":[]}`, wireInvalidParams)
	missingName := wireRPCError(base, "tools/call", "missing-tool-name", `{"arguments":{}}`, wireInvalidParams)
	// The request validator writes directly without the request-scoped trace.
	invalidParams.encode, missingName.encode = 0, 0
	plan := []serverWireCase{
		wireRPCError(base, "tools/call", "unknown-tool", `{"name":"fixture_unregistered"}`, wireInvalidParams),
		invalidParams,
		missingName,
		ioFailure,
		badResult,
		wireRPCError(base, "resources/list", "unavailable-method", "{}", wireMethodNotFound),
	}

	parse := wireRPCError(base, "jsonrpc", "parse-error", "", `{"code":-32700,"message":"parse error"}`)
	parse.request, parse.responseID, parse.encode = "{\n", "null", 0
	invalid := wireRPCError(base, "jsonrpc", "invalid-request", "", `{"code":-32600,"message":"invalid request"}`)
	invalid.request, invalid.encode = "{\"jsonrpc\":\"1.0\",\"id\":1,\"method\":\"tools/list\"}\n", 0
	plan = append(plan, parse, invalid)

	unsupported := wireRPCError(base, "tools/list", "unsupported-revision", "", "")
	if base.Revision == revision.Stateless {
		unsupported.request = strings.Replace(mcpRequest("tools/list", "{}", revision.Stateless), revision.Stateless, "2030-01-01", 1)
		unsupported.want = json.RawMessage(`{"code":-32022,"message":"unsupported protocol version","data":{"requested":"2030-01-01","supported":["2026-07-28"]}}`)
		plan = append(plan, wireRPCError(base, "ping", "unavailable-method", "{}", wireMethodNotFound))
	} else {
		unsupported.request = mcpRequest("tools/list", "{}", revision.Stateless)
		unsupported.want = json.RawMessage(`{"code":-32022,"message":"unsupported protocol version","data":{"requested":"2026-07-28","supported":[]}}`)
		plan = append(plan, wireRPCError(base, "server/discover", "unavailable-method", "{}", wireMethodNotFound))
	}
	plan = append(plan, unsupported)

	for _, name := range mcpRemovedToolNames {
		plan = append(plan, wireRPCError(base, "tools/call", "removed-"+name,
			fmt.Sprintf(`{"name":%q,"arguments":{}}`, name), wireInvalidParams))
	}
	if base.Profile == "read-only" {
		for _, name := range slices.Sorted(maps.Keys(cases)) {
			if cases[name].readOnly {
				continue
			}
			plan = append(plan, wireRPCError(base, "tools/call", "hidden-"+name,
				fmt.Sprintf(`{"name":%q,"arguments":{}}`, name), wireInvalidParams))
		}
	}
	return plan
}

// Supply a typed tool result with a missing declared payload. The production
// result adapter must contain this defect at its serialization boundary.
type wireInvalidResultTool struct{ tools.Tool }

func (wireInvalidResultTool) Execute(context.Context, json.RawMessage) (tools.ToolResult, error) {
	return tools.NewPayloadResult("complete"), nil
}
