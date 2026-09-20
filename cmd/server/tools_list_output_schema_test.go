package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	mcpserver "github.com/thomasweidner/flashgate-mcp/internal/mcp/server"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

var benchmarkToolsListWireSink []byte

func TestToolsListWireOutputSchemasAndPayloadSizes(t *testing.T) {
	expectedAnnotations := map[string]protocol.ToolAnnotations{
		"list_directory":   {ReadOnlyHint: true, IdempotentHint: true},
		"read_file":        {ReadOnlyHint: true, IdempotentHint: true},
		"get_path_info":    {ReadOnlyHint: true, IdempotentHint: true},
		"write_file":       {DestructiveHint: true},
		"create_directory": {IdempotentHint: true},
		"delete_path":      {DestructiveHint: true, IdempotentHint: true},
		"copy_path":        {DestructiveHint: true},
		"move_path":        {DestructiveHint: true, IdempotentHint: true},
	}
	tests := []struct {
		name                  string
		capabilities          toolCapabilities
		toolCount             int
		expectedResponseBytes int
		expectedResultBytes   int
	}{
		{"read-only", capabilitiesFromReadOnly(true), 3, 2446, 2411},
		{"default", toolCapabilities{filesystemWrite: true}, 8, 6492, 6457},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registry := createToolRegistry(noopFileSystem{}, 1024, tc.capabilities)
			request := bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\",\"params\":{}}\n")
			output := &bytes.Buffer{}
			server := mcpserver.New(request, output, createRouter("test-server", "test-version", registry))
			if err := server.Run(context.Background()); err != nil {
				t.Fatal(err)
			}
			if output.Len() != tc.expectedResponseBytes {
				t.Fatalf("response bytes=%d, want %d", output.Len(), tc.expectedResponseBytes)
			}
			var envelope struct {
				Result json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal(output.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid tools/list envelope: %v", err)
			}
			if len(envelope.Result) != tc.expectedResultBytes {
				t.Fatalf("result bytes=%d, want %d", len(envelope.Result), tc.expectedResultBytes)
			}

			var response struct {
				JSONRPC string          `json:"jsonrpc"`
				ID      json.RawMessage `json:"id"`
				Result  struct {
					Tools []protocol.Tool `json:"tools"`
				} `json:"result"`
			}
			if err := json.Unmarshal(output.Bytes(), &response); err != nil {
				t.Fatalf("invalid tools/list response: %v", err)
			}
			if len(response.Result.Tools) != tc.toolCount {
				t.Fatalf("got %d tools, want %d", len(response.Result.Tools), tc.toolCount)
			}
			var rawResult struct {
				Tools []map[string]json.RawMessage `json:"tools"`
			}
			if err := json.Unmarshal(envelope.Result, &rawResult); err != nil {
				t.Fatalf("invalid raw tools/list result: %v", err)
			}
			schemaCount := 0
			for index, tool := range response.Result.Tools {
				if tool.OutputSchema == nil {
					t.Fatalf("tools/list omitted outputSchema for %s", tool.Name)
				}
				if tool.OutputSchema["type"] != "object" {
					t.Fatalf("%s outputSchema root type=%#v", tool.Name, tool.OutputSchema["type"])
				}
				schemaCount++
				expected, ok := expectedAnnotations[tool.Name]
				if !ok {
					t.Fatalf("unexpected runtime tool %q has no annotation contract", tool.Name)
				}
				if tool.Annotations != expected {
					t.Fatalf("%s annotations=%#v, want %#v", tool.Name, tool.Annotations, expected)
				}
				var annotationMembers map[string]json.RawMessage
				if err := json.Unmarshal(rawResult.Tools[index]["annotations"], &annotationMembers); err != nil {
					t.Fatalf("%s annotations are missing or invalid: %v", tool.Name, err)
				}
				if len(annotationMembers) != 4 {
					t.Fatalf("%s annotations have %d members, want 4", tool.Name, len(annotationMembers))
				}
				for _, member := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
					if _, ok := annotationMembers[member]; !ok {
						t.Fatalf("%s annotations omit %s", tool.Name, member)
					}
				}
			}

			withSchemas := output.Len()
			for index := range response.Result.Tools {
				response.Result.Tools[index].OutputSchema = nil
			}
			historical, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			withoutSchemas := len(historical) + 1 // STDIO JSONL newline, matching the runtime response.
			delta := withSchemas - withoutSchemas
			percent := float64(delta) * 100 / float64(withoutSchemas)
			t.Logf("profile=%s without=%dB with=%dB delta=%dB change=%.2f%% tools=%d schemas=%d",
				tc.name, withoutSchemas, withSchemas, delta, percent, tc.toolCount, schemaCount)
		})
	}
}

func BenchmarkToolsListWireSerialization(b *testing.B) {
	tests := []struct {
		name         string
		capabilities toolCapabilities
	}{
		{"read-only", capabilitiesFromReadOnly(true)},
		{"default", toolCapabilities{filesystemWrite: true}},
	}

	for _, tc := range tests {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			registry := createToolRegistry(noopFileSystem{}, 1024, tc.capabilities)
			sampleOutput := &bytes.Buffer{}
			sampleServer := mcpserver.New(
				bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\",\"params\":{}}\n"),
				sampleOutput,
				createRouter("test-server", "test-version", registry),
			)
			if err := sampleServer.Run(context.Background()); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(sampleOutput.Len()))
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				output := &bytes.Buffer{}
				server := mcpserver.New(
					bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\",\"params\":{}}\n"),
					output,
					createRouter("test-server", "test-version", registry),
				)
				if err := server.Run(context.Background()); err != nil {
					b.Fatal(err)
				}
				benchmarkToolsListWireSink = append(benchmarkToolsListWireSink[:0], output.Bytes()...)
			}
			b.StopTimer()
			b.ReportMetric(float64(sampleOutput.Len()), "response-bytes")
		})
	}
}
