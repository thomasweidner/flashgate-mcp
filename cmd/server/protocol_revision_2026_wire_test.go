package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/initialize"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	mcpserver "github.com/thomasweidner/flashgate-mcp/internal/mcp/server"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func statelessRouter(t *testing.T) *router.Router {
	t.Helper()
	filesystem, err := fs.NewLocalFileSystem(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registry := createToolRegistry(filesystem, 1024, toolCapabilities{filesystemWrite: true})
	r := router.NewWithPolicy("flashgate", "test-version", revision.NewPolicy(protocol.ProtocolVersion, revision.Stateless))
	r.Register(initialize.NewHandler("flashgate", "test-version"))
	r.Register(tools.NewListHandler(registry))
	r.Register(tools.NewCallHandler(registry))
	return r
}

func runRevisionWire(t *testing.T, r *router.Router, requests ...string) []map[string]json.RawMessage {
	t.Helper()
	var stdout bytes.Buffer
	s := mcpserver.New(strings.NewReader(strings.Join(requests, "\n")+"\n"), &stdout, r)
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != len(requests) {
		t.Fatalf("unexpected stdout lines: %q", stdout.String())
	}
	results := make([]map[string]json.RawMessage, 0, len(lines))
	for _, line := range lines {
		var response map[string]json.RawMessage
		if json.Unmarshal([]byte(line), &response) != nil || string(response["jsonrpc"]) != `"2.0"` {
			t.Fatalf("non-JSON-RPC stdout: %q", line)
		}
		results = append(results, response)
	}
	return results
}

func TestStatelessCandidateWire(t *testing.T) {
	meta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{"extensions":{"io.modelcontextprotocol/tasks":{}}}}`
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{` + meta + `}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{` + meta + `}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_directory","arguments":{},` + meta + `}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"../outside"},` + meta + `}}`,
		`{"jsonrpc":"2.0","id":5,"method":"subscriptions/listen","params":{` + meta + `}}`,
		`{"jsonrpc":"2.0","id":6,"method":"initialize","params":{"protocolVersion":"2025-11-25",` + meta + `}}`,
		`{"jsonrpc":"2.0","id":7,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"client","version":"1"}}}`,
	}
	responses := runRevisionWire(t, statelessRouter(t), requests...)
	for i := 0; i < 4; i++ {
		if responses[i]["error"] != nil {
			t.Fatalf("unexpected candidate error %d: %s", i, responses[i]["error"])
		}
		var result map[string]json.RawMessage
		if json.Unmarshal(responses[i]["result"], &result) != nil || string(result["resultType"]) != `"complete"` {
			t.Fatalf("missing complete result %d: %s", i, responses[i]["result"])
		}
		var resultMeta map[string]json.RawMessage
		if json.Unmarshal(result["_meta"], &resultMeta) != nil || resultMeta["io.modelcontextprotocol/serverInfo"] == nil {
			t.Fatalf("missing server identity %d: %s", i, responses[i]["result"])
		}
		if i <= 1 && (string(result["ttlMs"]) != "0" || string(result["cacheScope"]) != `"private"`) {
			t.Fatalf("missing conservative cache hints %d: %s", i, responses[i]["result"])
		}
		if i >= 2 && (result["ttlMs"] != nil || result["cacheScope"] != nil) {
			t.Fatalf("unexpected cache hints on tools/call: %s", responses[i]["result"])
		}
		if i == 0 {
			if len(result) != 6 || result["supportedVersions"] == nil || result["supportedProtocolVersions"] != nil || result["extensions"] != nil || result["capabilities"] == nil {
				t.Fatalf("nonconforming discovery result: %s", responses[i]["result"])
			}
			var versions []string
			if err := json.Unmarshal(result["supportedVersions"], &versions); err != nil || len(versions) != 1 || versions[0] != revision.Stateless {
				t.Fatalf("unexpected discovery supportedVersions: %s: %v", responses[i]["result"], err)
			}
			var capabilities map[string]json.RawMessage
			if err := json.Unmarshal(result["capabilities"], &capabilities); err != nil || len(capabilities) != 1 || string(capabilities["tools"]) != `{}` || capabilities["extensions"] != nil {
				t.Fatalf("unexpected discovery capabilities: %s: %v", responses[i]["result"], err)
			}
			var serverInfo map[string]string
			if err := json.Unmarshal(resultMeta["io.modelcontextprotocol/serverInfo"], &serverInfo); err != nil || len(serverInfo) != 2 || serverInfo["name"] != "flashgate" || serverInfo["version"] != "test-version" {
				t.Fatalf("unexpected discovery serverInfo: %s: %v", responses[i]["result"], err)
			}
		}
		if i == 3 && string(result["isError"]) != "true" {
			t.Fatalf("expected tool error result: %s", responses[i]["result"])
		}
	}
	for i := 4; i < 6; i++ {
		var rpcErr protocol.Error
		if json.Unmarshal(responses[i]["error"], &rpcErr) != nil || rpcErr.Code != protocol.ErrMethodNotFound {
			t.Fatalf("unexpected unavailable method %d: %s", i, responses[i]["error"])
		}
	}
	var legacyResult map[string]json.RawMessage
	if responses[6]["error"] != nil || json.Unmarshal(responses[6]["result"], &legacyResult) != nil || string(legacyResult["protocolVersion"]) != `"2025-11-25"` || legacyResult["resultType"] != nil || legacyResult["_meta"] != nil {
		t.Fatalf("legacy initialization regressed: %#v", responses[6])
	}
	listFirst := runRevisionWire(t, statelessRouter(t), requests[1])
	if listFirst[0]["error"] != nil || !bytes.Contains(listFirst[0]["result"], []byte(`"resultType":"complete"`)) {
		t.Fatalf("stateless tools/list required discovery history: %#v", listFirst[0])
	}
}

func TestStatelessMetadataAndProductionGate(t *testing.T) {
	validMeta := `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}`
	defaultRouter := createRouter("flashgate", "test-version", createToolRegistry(noopFileSystem{}, 1024, toolCapabilities{}))
	legacyResponses := runRevisionWire(t, defaultRouter,
		`{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{`+validMeta+`}}`,
		`{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"client","version":"1"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{`+validMeta+`}}`,
	)
	var legacyProbe protocol.Error
	if err := json.Unmarshal(legacyResponses[0]["error"], &legacyProbe); err != nil || legacyProbe.Code != protocol.ErrMethodNotFound {
		t.Fatalf("default discovery probe exposed modern era: %s: %v", legacyResponses[0]["error"], err)
	}
	var initialized map[string]json.RawMessage
	if legacyResponses[1]["error"] != nil || json.Unmarshal(legacyResponses[1]["result"], &initialized) != nil || string(initialized["protocolVersion"]) != `"2025-11-25"` {
		t.Fatalf("legacy initialize failed after discovery probe: %#v", legacyResponses[1])
	}
	var disabledModern protocol.Error
	if err := json.Unmarshal(legacyResponses[2]["error"], &disabledModern); err != nil || disabledModern.Code != protocol.ErrUnsupportedProtocolVersion {
		t.Fatalf("default policy activated modern tools/list: %s: %v", legacyResponses[2]["error"], err)
	}
	var disabledData struct {
		Requested string   `json:"requested"`
		Supported []string `json:"supported"`
	}
	if err := json.Unmarshal(disabledModern.Data, &disabledData); err != nil || disabledData.Requested != revision.Stateless || len(disabledData.Supported) != 0 {
		t.Fatalf("default policy advertised a stateless revision: %s: %v", disabledModern.Data, err)
	}

	cases := []struct {
		name      string
		params    string
		want      int
		requested string
	}{
		{"valid", `{` + validMeta + `}`, 0, ""},
		{"absent meta", `{}`, protocol.ErrInvalidParams, ""},
		{"absent version", `{"_meta":{"io.modelcontextprotocol/clientCapabilities":{}}}`, protocol.ErrInvalidParams, ""},
		{"non-string version", `{"_meta":{"io.modelcontextprotocol/protocolVersion":1,"io.modelcontextprotocol/clientCapabilities":{}}}`, protocol.ErrInvalidParams, ""},
		{"null version", `{"_meta":{"io.modelcontextprotocol/protocolVersion":null,"io.modelcontextprotocol/clientCapabilities":{}}}`, protocol.ErrInvalidParams, ""},
		{"absent capabilities", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}`, protocol.ErrInvalidParams, ""},
		{"wrong capabilities type", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":[]}}`, protocol.ErrInvalidParams, ""},
		{"malformed reserved metadata", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{},"io.modelcontextprotocol/clientInfo":[]}}`, protocol.ErrInvalidParams, ""},
		{"legacy revision", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2025-11-25","io.modelcontextprotocol/clientCapabilities":{}}}`, protocol.ErrUnsupportedProtocolVersion, "2025-11-25"},
		{"future", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"future","io.modelcontextprotocol/clientCapabilities":{}}}`, protocol.ErrUnsupportedProtocolVersion, "future"},
		{"draft", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"draft-x","io.modelcontextprotocol/clientCapabilities":{}}}`, protocol.ErrUnsupportedProtocolVersion, "draft-x"},
		{"invalid calendar date", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-99-99","io.modelcontextprotocol/clientCapabilities":{}}}`, protocol.ErrUnsupportedProtocolVersion, "2026-99-99"},
	}
	requests := make([]string, len(cases))
	for i, test := range cases {
		requests[i] = `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":` + test.params + `}`
	}
	responses := runRevisionWire(t, statelessRouter(t), requests...)
	for i, test := range cases {
		if test.want == 0 {
			if responses[i]["error"] != nil || !bytes.Contains(responses[i]["result"], []byte(`"supportedVersions":["2026-07-28"]`)) {
				t.Fatalf("valid candidate discovery failed: %#v", responses[i])
			}
			continue
		}
		var rpcErr protocol.Error
		if err := json.Unmarshal(responses[i]["error"], &rpcErr); err != nil || rpcErr.Code != test.want {
			t.Fatalf("%s: error %s, want %d: %v", test.name, responses[i]["error"], test.want, err)
		}
		if test.want == protocol.ErrUnsupportedProtocolVersion {
			var data struct {
				Requested string   `json:"requested"`
				Supported []string `json:"supported"`
			}
			if err := json.Unmarshal(rpcErr.Data, &data); err != nil || data.Requested != test.requested || len(data.Supported) != 1 || data.Supported[0] != revision.Stateless {
				t.Fatalf("%s: unsafe or incorrect unsupported-version data: %s: %v", test.name, rpcErr.Data, err)
			}
		}
	}
}
