package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcpserver "github.com/thomasweidner/flashgate-mcp/internal/mcp/server"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestProtocolRevisionWire(t *testing.T) {
	cases := []struct {
		name           string
		request        string
		methodNotFound bool
	}{
		{
			name:    "supported initialize",
			request: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"client","version":"1"}}}`,
		},
		{
			name:    "unsupported proposal through initialize",
			request: `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2026-07-28","capabilities":{},"clientInfo":{"name":"client","version":"1"}}}`,
		},
		{
			name:           "server discover as first request",
			request:        `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{}}`,
			methodNotFound: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			registry := createToolRegistry(noopFileSystem{}, 1024, toolCapabilities{filesystemWrite: true})
			var stdout bytes.Buffer
			server := mcpserver.New(strings.NewReader(tc.request+"\n"), &stdout, createRouter("flashgate", "test-version", registry))
			if err := server.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(stdout.String(), "\n") {
				t.Fatalf("stdout must end with a JSON-RPC response line: %q", stdout.String())
			}
			lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
			if len(lines) != 1 {
				t.Fatalf("stdout must contain exactly one JSON-RPC response, got %q", stdout.String())
			}
			line := lines[0]
			var envelope struct {
				JSONRPC string                     `json:"jsonrpc"`
				ID      int                        `json:"id"`
				Result  map[string]json.RawMessage `json:"result"`
				Error   *protocol.Error            `json:"error"`
			}
			if err := json.Unmarshal([]byte(line), &envelope); err != nil || envelope.JSONRPC != "2.0" || envelope.ID != 1 {
				t.Fatalf("invalid wire envelope: %q: %v", line, err)
			}
			if tc.methodNotFound {
				if envelope.Error == nil || envelope.Error.Code != protocol.ErrMethodNotFound || envelope.Result != nil {
					t.Fatalf("server/discover must remain unregistered: %s", line)
				}
				return
			}
			if envelope.Error != nil || len(envelope.Result) != 3 {
				t.Fatalf("unexpected initialize response: %s", line)
			}
			var revision string
			if err := json.Unmarshal(envelope.Result["protocolVersion"], &revision); err != nil || revision != protocol.ProtocolVersion {
				t.Fatalf("unexpected negotiated revision: %s", line)
			}
			var capabilities map[string]json.RawMessage
			if err := json.Unmarshal(envelope.Result["capabilities"], &capabilities); err != nil || len(capabilities) != 1 || string(capabilities["tools"]) != "{}" {
				t.Fatalf("unexpected capabilities or extensions: %s", line)
			}
			var serverInfo map[string]string
			if err := json.Unmarshal(envelope.Result["serverInfo"], &serverInfo); err != nil || len(serverInfo) != 2 || serverInfo["name"] != "flashgate" || serverInfo["version"] != "test-version" {
				t.Fatalf("unexpected server info: %s", line)
			}
		})
	}
}
