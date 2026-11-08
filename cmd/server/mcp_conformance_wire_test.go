package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestMCPConformanceWireNegatives(t *testing.T) {
	t.Run("read-file-encoding", verifyReadFileEncoding)
	for _, candidate := range []bool{false, true} {
		registry, _ := mcpRegistry(t, false)
		r := mcpRouter(registry, candidate)
		version := protocol.ProtocolVersion
		if candidate {
			version = revision.Stateless
		}
		for _, test := range []struct {
			method string
			params string
			code   int
		}{
			{"unknown/method", `{}`, protocol.ErrMethodNotFound},
			{"resources/list", `{}`, protocol.ErrMethodNotFound},
			{"prompts/list", `{}`, protocol.ErrMethodNotFound},
			{"subscriptions/listen", `{}`, protocol.ErrMethodNotFound},
			{"tools/call", `{"name":"unknown","arguments":{}}`, protocol.ErrInvalidParams},
			{"tools/call", `{"name":"read_file","arguments":{"path":42}}`, 0},
			{"tools/call", `{"name":"read_file","arguments":{"path":"seed.txt","unexpected":true}}`, 0},
		} {
			response := mcpWire(t, r, mcpRequest(test.method, test.params, version), 1)[0]
			if test.code == 0 {
				var result map[string]json.RawMessage
				if response["error"] != nil || json.Unmarshal(response["result"], &result) != nil || string(result["isError"]) != "true" || result["structuredContent"] != nil {
					t.Fatalf("registered-tool argument error lost its CallToolResult contract: %s", response["result"])
				}
				mcpCheckRevisionResult(t, result, version, false)
				continue
			}
			var rpcError protocol.Error
			if err := json.Unmarshal(response["error"], &rpcError); err != nil || rpcError.Code != test.code {
				t.Fatalf("%s/%s: expected %d: %s", version, test.method, test.code, response["error"])
			}
		}
		mcpWire(t, r, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"+`{"jsonrpc":"2.0","method":"unknown/notification"}`+"\n", 0)
		for _, raw := range []string{`{`, `[]`, `{"jsonrpc":"1.0","id":1,"method":"tools/list"}`, `{"jsonrpc":"2.0","id":{},"method":"tools/list"}`} {
			response := mcpWire(t, r, raw+"\n", 1)[0]
			var rpcError protocol.Error
			if err := json.Unmarshal(response["error"], &rpcError); err != nil || (rpcError.Code != protocol.ErrParseError && rpcError.Code != protocol.ErrInvalidRequest) {
				t.Fatalf("malformed envelope accepted: %s", response["error"])
			}
		}
	}
}

func verifyReadFileEncoding(t *testing.T) {
	for _, version := range mcpConformanceVersions {
		for _, readOnly := range []bool{false, true} {
			registry, root := mcpRegistry(t, readOnly)
			r := mcpRouter(registry, version == revision.Stateless)
			valid := []byte("\ufeffGruß 💾\r\n<&>\x00\u2028")
			invalid := append([]byte("private-body-marker"), 0xff)
			for _, fixture := range []struct {
				file string
				data []byte
			}{
				{"utf8.txt", valid},
				{"empty.txt", nil},
				{"invalid.txt", invalid},
			} {
				if err := os.WriteFile(filepath.Join(root, fixture.file), fixture.data, 0o600); err != nil {
					t.Fatal(err)
				}
				result := mcpResult(t, r, "tools/call",
					`{"name":"read_file","arguments":{"path":"`+fixture.file+`"}}`, version)
				mcpCheckRevisionResult(t, result, version, false)
				envelope := mcpResultEnvelope(t, result)
				if fixture.file == "invalid.txt" {
					decoded, err := mcptest.DecodeCallToolResult(envelope)
					if err != nil || !decoded.IsError || decoded.HasStructuredContent {
						t.Fatalf("%s read-only=%t: invalid bytes were not rejected: %s, %v", version, readOnly, envelope, err)
					}
					body := decoded.TextContent.(map[string]any)
					if body["category"] != "invalid_text_encoding" || body["message"] != "file text is not valid UTF-8" {
						t.Fatalf("invalid encoding must return only its safe category and message: %s", envelope)
					}
					continue
				}
				decoded, err := mcptest.DecodeSplitCallToolResult(envelope)
				if err != nil || decoded.IsError || decoded.TextContent != string(fixture.data) ||
					string(result["structuredContent"]) != `{"size":`+strconv.Itoa(len(fixture.data))+`}` {
					t.Fatalf("%s read-only=%t: file text or raw size changed: %s, %v", version, readOnly, envelope, err)
				}
			}
		}
	}
}

func TestMCPConformanceLegacyNegotiation(t *testing.T) {
	registry, _ := mcpRegistry(t, false)
	r := mcpRouter(registry, false)
	for _, proposed := range []string{protocol.ProtocolVersion, revision.Stateless, "2030-01-01"} {
		result := mcpResult(t, r, "initialize", `{"protocolVersion":"`+proposed+`","capabilities":{},"clientInfo":{"name":"contract","version":"1"}}`, protocol.ProtocolVersion)
		if string(result["protocolVersion"]) != `"2025-11-25"` {
			t.Fatalf("invalid exact-revision negotiation: %s", result["protocolVersion"])
		}
		mcpCheckRevisionResult(t, result, protocol.ProtocolVersion, false)
	}
}

func TestMCPConformanceCrossRevisionStateless(t *testing.T) {
	registry, _ := mcpRegistry(t, true)
	r := mcpRouter(registry, true)
	// Ordinary requests succeed before discovery; an intervening legacy request
	// cannot enable, disable or supply metadata for the following request.
	requests := mcpRequest("tools/list", `{}`, revision.Stateless) +
		mcpRequest("initialize", `{"protocolVersion":"2025-11-25"}`, protocol.ProtocolVersion) +
		mcpRequest("tools/list", `{}`, protocol.ProtocolVersion) +
		mcpRequest("tools/list", `{}`, revision.Stateless)
	responses := mcpWire(t, r, requests, 4)
	for i, version := range []string{revision.Stateless, protocol.ProtocolVersion, protocol.ProtocolVersion, revision.Stateless} {
		var result map[string]json.RawMessage
		if err := json.Unmarshal(responses[i]["result"], &result); err != nil {
			t.Fatal(err)
		}
		mcpCheckRevisionResult(t, result, version, version == revision.Stateless)
	}
	// Self-reported client extensions and capabilities cannot expose write tools.
	request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_file","arguments":{"path":"blocked.txt","content":"x"},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{"tools":{},"extensions":{"io.modelcontextprotocol/tasks":{}}}}}}` + "\n"
	response := mcpWire(t, r, request, 1)[0]
	var rpcError protocol.Error
	if err := json.Unmarshal(response["error"], &rpcError); err != nil || rpcError.Code != protocol.ErrInvalidParams {
		t.Fatal("client metadata bypassed read-only profile")
	}
}

// The helper process executes the real startup/configuration and STDIO loop.
// The candidate is never injected into this path.
func TestMCPConformanceProcessHelper(t *testing.T) {
	if os.Getenv("FLASHGATE_MCP_CONFORMANCE_HELPER") != "1" {
		return
	}
	os.Exit(runMain(context.Background(), nil, os.Stdout, os.Stderr, run))
}

func TestMCPConformanceSTDIOProcess(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, readOnly := range []bool{false, true} {
		_, root := mcpRegistry(t, readOnly)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		command := exec.CommandContext(ctx, executable, "-test.run=^TestMCPConformanceProcessHelper$")
		command.WaitDelay = time.Second
		for _, entry := range os.Environ() {
			key := strings.ToUpper(strings.SplitN(entry, "=", 2)[0])
			if !strings.HasPrefix(key, "MCP_") && key != "FLASHGATE_MCP_CONFORMANCE_HELPER" {
				command.Env = append(command.Env, entry)
			}
		}
		command.Env = append(command.Env, "MCP_ROOT="+root, "MCP_READ_ONLY="+map[bool]string{true: "true", false: "false"}[readOnly], "FLASHGATE_MCP_CONFORMANCE_HELPER=1")
		command.Stdin = strings.NewReader(
			mcpRequest("initialize", `{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"contract","version":"1"}}`, protocol.ProtocolVersion) +
				`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
				mcpRequest("tools/list", `{}`, protocol.ProtocolVersion) +
				mcpRequest("tools/call", `{"name":"write_file","arguments":{"path":"new.txt","content":"x"}}`, protocol.ProtocolVersion) +
				mcpRequest("server/discover", `{}`, revision.Stateless) +
				mcpRequest("tools/list", `{}`, revision.Stateless))
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		cancel()
		if err != nil || stderr.Len() != 0 {
			t.Fatalf("STDIO process failed: %v; stderr=%q", err, stderr.String())
		}
		lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
		if len(lines) != 5 {
			t.Fatalf("STDIO purity/count: %q", stdout.String())
		}
		responses := make([]struct {
			JSONRPC string          `json:"jsonrpc"`
			Result  json.RawMessage `json:"result"`
			Error   *protocol.Error `json:"error"`
		}, len(lines))
		for i, line := range lines {
			if err := json.Unmarshal([]byte(line), &responses[i]); err != nil || responses[i].JSONRPC != "2.0" {
				t.Fatalf("nonprotocol stdout: %q", line)
			}
		}
		var list struct {
			Tools []protocol.Tool `json:"tools"`
		}
		if err := json.Unmarshal(responses[1].Result, &list); err != nil {
			t.Fatal(err)
		}
		if err := mcpInventory(list.Tools, mcpToolCases(), readOnly); err != nil {
			t.Fatal(err)
		}
		if readOnly {
			if responses[2].Error == nil || responses[2].Error.Code != protocol.ErrInvalidParams {
				t.Fatal("STDIO read-only write accepted")
			}
		} else if responses[2].Error != nil || !bytes.Contains(responses[2].Result, []byte(`"written":true`)) {
			t.Fatal("STDIO default write failed")
		}
		if responses[3].Error == nil || responses[3].Error.Code != protocol.ErrMethodNotFound || responses[4].Error == nil || responses[4].Error.Code != protocol.ErrUnsupportedProtocolVersion {
			t.Fatal("production candidate revision gate changed")
		}
	}
}
