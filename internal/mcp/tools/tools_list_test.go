package tools

import (
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestListHandlerReturnsRegisteredTools(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	requireRegistration(t, registry, &testTool{
		name:        "list_directory",
		title:       "List Directory",
		description: "Lists files and directories.",
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type": "string",
				},
			},
		},
	})
	requireRegistration(t, registry, &testTool{
		name:        "read_file",
		title:       "Read File",
		description: "Reads a file.",
		inputSchema: map[string]any{
			"type": "object",
		},
	})

	handler := NewListHandler(registry)

	result, rpcErr := handler.Handle(handlers.Context{}, nil)
	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("expected marshalable result, got %v", err)
	}

	var decoded struct {
		Tools []struct {
			Name        string         `json:"name"`
			Title       string         `json:"title"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}

	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid result JSON, got %v", err)
	}

	if len(decoded.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(decoded.Tools))
	}

	names := map[string]bool{}
	for _, tool := range decoded.Tools {
		names[tool.Name] = true

		if tool.Description == "" {
			t.Fatalf("expected description for tool %q", tool.Name)
		}
		if tool.Title == "" {
			t.Fatalf("expected title for tool %q", tool.Name)
		}

		if tool.InputSchema == nil {
			t.Fatalf("expected input schema for tool %q", tool.Name)
		}
	}

	if !names["list_directory"] {
		t.Fatal("expected list_directory tool")
	}

	if !names["read_file"] {
		t.Fatal("expected read_file tool")
	}
}

func TestListHandlerMethod(t *testing.T) {
	t.Parallel()

	handler := NewListHandler(NewRegistry())

	if handler.Method() != "tools/list" {
		t.Fatalf("expected tools/list, got %q", handler.Method())
	}
}

func TestListHandlerAcceptsMissingNullAndObjectParams(t *testing.T) {
	t.Parallel()

	testCases := map[string]json.RawMessage{
		"missing": nil,
		"null":    json.RawMessage(`null`),
		"object":  json.RawMessage(`{"extra":true}`),
	}

	for name, rawParams := range testCases {
		name := name
		rawParams := rawParams

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := NewListHandler(NewRegistry())

			result, rpcErr := handler.Handle(handlers.Context{}, rawParams)
			if rpcErr != nil {
				t.Fatalf("expected no error, got %v", rpcErr)
			}

			if result == nil {
				t.Fatal("expected result")
			}
		})
	}
}

func TestListHandlerReturnsInvalidParamsForInvalidParamsShape(t *testing.T) {
	t.Parallel()

	testCases := map[string]json.RawMessage{
		"string": json.RawMessage(`"bad"`),
		"array":  json.RawMessage(`[]`),
		"number": json.RawMessage(`1`),
		"bool":   json.RawMessage(`true`),
	}

	for name, rawParams := range testCases {
		name := name
		rawParams := rawParams

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := NewListHandler(NewRegistry())

			result, rpcErr := handler.Handle(handlers.Context{}, rawParams)
			if result != nil {
				t.Fatalf("expected nil result, got %#v", result)
			}

			if rpcErr == nil {
				t.Fatal("expected rpc error")
			}

			if rpcErr.Code != protocol.ErrInvalidParams {
				t.Fatalf("expected ErrInvalidParams, got %d", rpcErr.Code)
			}

			if rpcErr.Message != "invalid params" {
				t.Fatalf("expected generic invalid params message, got %q", rpcErr.Message)
			}
		})
	}
}
