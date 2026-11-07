package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestMCPConformancePingRevisionIsolation(t *testing.T) {
	r := statelessRouter(t)
	responses := runRevisionWire(t, r,
		`{"jsonrpc":"2.0","id":"before","method":"ping"}`,
		strings.TrimSpace(mcpRequest("ping", `{}`, revision.Stateless)),
		`{"jsonrpc":"2.0","id":"after","method":"ping"}`,
	)
	assertPingResponse(t, responses[0], `"before"`, 0)
	assertPingResponse(t, responses[1], "1", protocol.ErrMethodNotFound)
	assertPingResponse(t, responses[2], `"after"`, 0)
}

func TestMCPConformancePingRegistryIndependence(t *testing.T) {
	registry := tools.NewRegistry()
	r := createRouter("flashgate", "test-version", registry)
	request := `{"jsonrpc":"2.0","id":7,"method":"ping"}` + "\n"
	assertPingResponse(t, mcpWire(t, r, request, 1)[0], "7", 0)

	fixture, _ := mcpRegistry(t, false)
	registry.Register(mcpFutureTool{Tool: fixture.List()[0]})
	assertPingResponse(t, mcpWire(t, r, request, 1)[0], "7", 0)
	result := mcpResult(t, r, "tools/list", `{}`, protocol.ProtocolVersion)
	var listed []protocol.Tool
	if err := json.Unmarshal(result["tools"], &listed); err != nil || len(listed) != 1 || listed[0].Name != "future_tool" {
		t.Fatalf("unexpected tool catalog: %s, %v", result["tools"], err)
	}
}

func TestMCPConformancePingSTDIO(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []struct {
		name, readOnly, debug string
	}{{"default", "false", "false"}, {"read-only", "true", "true"}} {
		t.Run(profile.name, func(t *testing.T) {
			_, root := mcpRegistry(t, profile.readOnly == "true")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestMCPConformanceProcessHelper$")
			command.WaitDelay = time.Second
			for _, entry := range command.Environ() {
				key := strings.ToUpper(strings.SplitN(entry, "=", 2)[0])
				if !strings.HasPrefix(key, "MCP_") && key != "FLASHGATE_MCP_CONFORMANCE_HELPER" {
					command.Env = append(command.Env, entry)
				}
			}
			command.Env = append(command.Env, "MCP_ROOT="+root, "MCP_READ_ONLY="+profile.readOnly,
				"MCP_DEBUG="+profile.debug, "MCP_MAX_JSONRPC_MESSAGE_BYTES=1024", "FLASHGATE_MCP_CONFORMANCE_HELPER=1")

			var input strings.Builder
			input.WriteString(`{"jsonrpc":"2.0","id":"pre","method":"ping"}` + "\n")
			input.WriteString(mcpRequest("initialize", `{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"ping-test","version":"1"}}`, protocol.ProtocolVersion))
			input.WriteString(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
			input.WriteString(`{"jsonrpc":"2.0","method":"ping","params":null}` + "\n")
			input.WriteString(`{"jsonrpc":"2.0","method":"ping"}` + "\n")
			cases := []struct {
				id, method, params string
				code               int
			}{
				{"9007199254740993", "ping", `{}`, 0},
				{`"meta"`, "ping", `{"future":42,"_meta":{"progressToken":"token","custom":{"nested":true}}}`, 0},
				{"-9", "ping", `{"_meta":{"progressToken":-0.0100e2}}`, 0},
				{"12", "ping", `{"_meta":{"progressToken":1e99999999999999999999}}`, 0},
				{"13", "ping", `{"_meta":{"progressToken":0e-99999999999999999999}}`, 0},
				{"14", "ping", `{"_meta":{"progressToken":10.00e-1}}`, 0},
				{"15", "ping", `null`, protocol.ErrInvalidParams},
				{"16", "ping", `[]`, protocol.ErrInvalidParams},
				{"17", "ping", `"bad"`, protocol.ErrInvalidParams},
				{"18", "ping", `{"_meta":null}`, protocol.ErrInvalidParams},
				{"19", "ping", `{"_meta":[]}`, protocol.ErrInvalidParams},
				{"20", "ping", `{"_meta":{"progressToken":null}}`, protocol.ErrInvalidParams},
				{"21", "ping", `{"_meta":{"progressToken":true}}`, protocol.ErrInvalidParams},
				{"22", "ping", `{"_meta":{"progressToken":1.000000000000000000001}}`, protocol.ErrInvalidParams},
				{"23", "ping", `{"_meta":{"progressToken":1e-99999999999999999999}}`, protocol.ErrInvalidParams},
				{"24", "unknown/ping", `{}`, protocol.ErrMethodNotFound},
				{"25", "ping", `{"extra":"` + strings.Repeat("x", 1024) + `"}`, protocol.ErrInvalidRequest},
				{`"isolated"`, "ping", `{}`, 0},
			}
			for _, test := range cases {
				input.WriteString(`{"jsonrpc":"2.0","id":` + test.id + `,"method":"` + test.method + `","params":` + test.params + "}\n")
			}
			command.Stdin = strings.NewReader(input.String())
			var stdout, stderr bytes.Buffer
			command.Stdout, command.Stderr = &stdout, &stderr
			if err := command.Run(); err != nil {
				t.Fatalf("STDIO process: %v, stderr=%q", err, stderr.String())
			}
			if profile.debug == "true" && !strings.Contains(stderr.String(), "notification ignored method=ping") {
				t.Fatalf("missing stderr diagnostic: %q", stderr.String())
			}
			if profile.debug == "false" && stderr.Len() != 0 {
				t.Fatalf("unexpected stderr: %q", stderr.String())
			}
			lines := bufio.NewScanner(&stdout)
			readResponse := func() map[string]json.RawMessage {
				t.Helper()
				var response map[string]json.RawMessage
				if !lines.Scan() || json.Unmarshal(lines.Bytes(), &response) != nil || string(response["jsonrpc"]) != `"2.0"` {
					t.Fatalf("expected JSON-RPC stdout line: %q", lines.Text())
				}
				return response
			}
			assertPingResponse(t, readResponse(), `"pre"`, 0)
			initial := readResponse()
			if initial["error"] != nil || !bytes.Contains(initial["result"], []byte(`"protocolVersion":"2025-11-25"`)) {
				t.Fatalf("initialize failed: %v", initial)
			}
			for _, test := range cases {
				id := test.id
				if test.code == protocol.ErrInvalidRequest {
					id = "null"
				}
				assertPingResponse(t, readResponse(), id, test.code)
			}
			if lines.Scan() || lines.Err() != nil {
				t.Fatalf("unexpected stdout after final response: %q, %v", lines.Text(), lines.Err())
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 1 || entries[0].Name() != "seed.txt" {
				t.Fatalf("ping changed filesystem entries: %v, %v", entries, err)
			}
			if seed, err := os.ReadFile(filepath.Join(root, "seed.txt")); err != nil || string(seed) != "seed" {
				t.Fatalf("ping changed seed file: %q, %v", seed, err)
			}
		})
	}
}

func assertPingResponse(t *testing.T, response map[string]json.RawMessage, id string, code int) {
	t.Helper()
	if len(response) != 3 || string(response["jsonrpc"]) != `"2.0"` || string(response["id"]) != id {
		t.Fatalf("unexpected envelope for ID %s: %v", id, response)
	}
	if code == 0 {
		if response["error"] != nil || string(response["result"]) != "{}" {
			t.Fatalf("ID %s: expected exact empty result, got %v", id, response)
		}
		return
	}
	var rpcError protocol.Error
	if response["result"] != nil || json.Unmarshal(response["error"], &rpcError) != nil || rpcError.Code != code || rpcError.Data != nil {
		t.Fatalf("ID %s: expected generic error %d, got %s", id, code, response["error"])
	}
}
