package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func resourceWire(t *testing.T, root, settings, params, id string) ([]byte, json.RawMessage) {
	t.Helper()
	return resourceWireVersion(t, root, settings, params, id, protocol.ProtocolVersion)
}

func resourceWireVersion(t *testing.T, root, settings, params, id, version string) ([]byte, json.RawMessage) {
	t.Helper()
	document := configDocument(t, root, "roots.work.access=['read','write']\n"+settings)
	source := writeConfig(t, document)
	deps := defaultBootstrapDependencies(source)
	if version == revision.Stateless {
		deps.newRouter = func(name, version string, registry *tools.Registry) *router.Router {
			r := router.NewWithPolicy(name, version, revision.NewPolicy(protocol.ProtocolVersion, revision.Stateless))
			r.Register(tools.NewListHandler(registry))
			r.Register(tools.NewCallHandler(registry))
			return r
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(params), &fields); err != nil {
			t.Fatal(err)
		}
		fields["_meta"] = json.RawMessage(`{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`)
		encoded, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		params = string(encoded)
	}
	input := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"method":"tools/call","params":%s}`+"\n", id, params)
	var out, diagnostics bytes.Buffer
	if err := runWithIO(context.Background(), strings.NewReader(input), &out, &diagnostics, deps); err != nil {
		t.Fatal(err)
	}
	if diagnostics.Len() != 0 || bytes.Count(out.Bytes(), []byte("\n")) != 1 {
		t.Fatal("incomplete STDIO frame", out.String(), diagnostics.String())
	}
	var response struct {
		Result json.RawMessage
		Error  *protocol.Error
	}
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return out.Bytes(), response.Result
}

func TestResourceStatelessCompletionBoundaries(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source"), []byte("<>\n\u2028"), 0600); err != nil {
		t.Fatal(err)
	}
	const params = `{"name":"read_file","arguments":{"root_id":"work","path":"source"}}`
	const id = `"<id>"`
	baseline, result := resourceWireVersion(t, root, "", params, id, revision.Stateless)
	if !bytes.Contains(result, []byte(`"resultType":"complete"`)) || bytes.Contains(result, []byte(`"isError":true`)) {
		t.Fatal(string(result))
	}
	for _, delta := range []int{0, -1} {
		out, r := resourceWireVersion(t, root, fmt.Sprintf("roots.work.max_result_size=%d\n", len(result)+delta), params, id, revision.Stateless)
		if delta == 0 {
			if !bytes.Equal(out, baseline) {
				t.Fatal("exact completed result denied")
			}
		} else if !bytes.Contains(r, []byte("limit_exceeded")) {
			t.Fatal("revision metadata escaped root cap", string(r))
		}
		out, r = resourceWireVersion(t, root, fmt.Sprintf("[protocol]\nmax_response_size=%d\n", len(baseline)+delta), params, id, revision.Stateless)
		if delta == 0 {
			if !bytes.Equal(out, baseline) {
				t.Fatal("exact completed frame denied")
			}
		} else if bytes.Equal(out, baseline) || bytes.Contains(r, []byte(`"size":`)) {
			t.Fatal("completed frame bypass")
		}
	}
	const write = `{"name":"write_file","arguments":{"root_id":"work","path":"target","content":"ok"}}`
	baseline, result = resourceWireVersion(t, root, "[profiles]\nactive='write'\n", write, id, revision.Stateless)
	if err := os.Remove(filepath.Join(root, "target")); err != nil {
		t.Fatal(err)
	}
	for _, settings := range []string{fmt.Sprintf("roots.work.max_result_size=%d\n", len(result)-1), fmt.Sprintf("[protocol]\nmax_response_size=%d\n", len(baseline)-1)} {
		out, _ := resourceWireVersion(t, root, settings+"[profiles]\nactive='write'\n", write, id, revision.Stateless)
		if bytes.Contains(out, []byte(`"written":true`)) {
			t.Fatal("oversize completed write receipt succeeded")
		}
		if _, err := os.Stat(filepath.Join(root, "target")); !os.IsNotExist(err) {
			t.Fatal("revision overhead denied after mutation", err)
		}
	}
}

func TestResourceResultAndFramedWireBoundaries(t *testing.T) {
	root := t.TempDir()
	content := []byte("<>\n\x00\u2028\"\\")
	if err := os.WriteFile(filepath.Join(root, "source"), content, 0600); err != nil {
		t.Fatal(err)
	}
	const params = `{"name":"read_file","arguments":{"root_id":"work","path":"source","maxBytes":1000000}}`
	const id = `"<id>"`
	baseline, result := resourceWire(t, root, "", params, id)
	if bytes.Contains(result, []byte(`"isError":true`)) {
		t.Fatal(string(result))
	}
	for _, delta := range []int{0, -1} {
		out, r := resourceWire(t, root, fmt.Sprintf("roots.work.max_result_size=%d\n", len(result)+delta), params, id)
		if delta == 0 {
			if !bytes.Equal(out, baseline) {
				t.Fatal("exact result changed wire")
			}
		} else if !bytes.Contains(r, []byte("limit_exceeded")) {
			t.Fatal("escaping bypass", string(r))
		}
		out, r = resourceWire(t, root, fmt.Sprintf("[protocol]\nmax_response_size=%d\n", len(baseline)+delta), params, id)
		if delta == 0 {
			if !bytes.Equal(out, baseline) {
				t.Fatal("exact frame denied", string(out))
			}
		} else if bytes.Equal(out, baseline) || bytes.Contains(r, []byte(`"size":`)) {
			t.Fatal("frame overflow returned payload")
		}
	}
}

func TestResourceWireDenialsHaveNoMutationAndNoStartupWrite(t *testing.T) {
	for _, field := range []string{"roots.work.max_result_size=1\n", "[protocol]\nmax_response_size=80\n"} {
		root := t.TempDir()
		out, _ := resourceWire(t, root, field+"[profiles]\nactive='write'\n", `{"name":"write_file","arguments":{"root_id":"work","path":"denied","content":"abc"}}`, "1")
		if bytes.Contains(out, []byte(`"written":true`)) {
			t.Fatal("denied write success")
		}
		if _, err := os.Stat(filepath.Join(root, "denied")); !os.IsNotExist(err) {
			t.Fatal("result denial changed files", err)
		}
	}
	root := t.TempDir()
	document := configDocument(t, root, "max_file_size='10MiB'\nmax_scan_size='1,234 B'\n")
	source := writeConfig(t, document)
	before, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if err := runWithIO(context.Background(), strings.NewReader(""), &out, &diagnostics, defaultBootstrapDependencies(source)); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(source)
	if err != nil || string(actual) != document || !before.ModTime().Equal(after.ModTime()) || !os.SameFile(before, after) {
		t.Fatal("startup rewrote size spelling")
	}
}
