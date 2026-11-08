package tools

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestCallHandlerCallsRegisteredTool(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	tool := &testTool{
		name:   "test_tool",
		result: createDirectoryResult{Path: "fixture", Created: true},
	}

	requireRegistration(t, registry, tool)

	handler := NewCallHandler(registry)

	result, rpcErr := handler.Handle(
		handlers.Context{},
		json.RawMessage(`{"name":"test_tool","arguments":{"path":"."}}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if tool.called != 1 {
		t.Fatalf("expected tool to be called once, got %d", tool.called)
	}

	if string(tool.arguments) != `{"path":"."}` {
		t.Fatalf("expected arguments to be forwarded unchanged, got %s", string(tool.arguments))
	}

	callResult, ok := result.(protocol.CallToolResult)
	if !ok {
		t.Fatalf("expected CallToolResult, got %#v", result)
	}
	assertWrappedToolResult(t, callResult, tool.result)
}

func TestCallHandlerReturnsInvalidParamsForUnknownTool(t *testing.T) {
	t.Parallel()

	handler := NewCallHandler(NewRegistry())

	result, rpcErr := handler.Handle(
		handlers.Context{},
		json.RawMessage(`{"name":"missing_tool","arguments":{}}`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %d", rpcErr.Code)
	}
}

func TestCallHandlerRejectsRemovedToolNames(t *testing.T) {
	for _, name := range []string{"list_files", "stat_path", "exists_path", "mkdir", "rename_path"} {
		result, rpcErr := NewCallHandler(NewRegistry()).Handle(
			handlers.Context{}, json.RawMessage(`{"name":"`+name+`","arguments":{}}`),
		)
		if result != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected removed tool %q to be rejected generically, result=%#v error=%#v", name, result, rpcErr)
		}
	}
}

func TestCallHandlerReturnsInvalidParamsForMalformedParams(t *testing.T) {
	t.Parallel()

	handler := NewCallHandler(NewRegistry())

	result, rpcErr := handler.Handle(
		handlers.Context{},
		json.RawMessage(`{"name":`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %d", rpcErr.Code)
	}
}

func TestCallHandlerReturnsInvalidParamsForMissingName(t *testing.T) {
	t.Parallel()

	handler := NewCallHandler(NewRegistry())

	result, rpcErr := handler.Handle(
		handlers.Context{},
		json.RawMessage(`{"arguments":{}}`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %d", rpcErr.Code)
	}
}

func TestCallHandlerReturnsInvalidParamsForInvalidParamsShape(t *testing.T) {
	t.Parallel()

	testCases := map[string]json.RawMessage{
		"missing params": nil,
		"null params":    json.RawMessage(`null`),
		"string params":  json.RawMessage(`"bad"`),
		"array params":   json.RawMessage(`[]`),
		"number params":  json.RawMessage(`1`),
		"bool params":    json.RawMessage(`true`),
		"non-string name": json.RawMessage(
			`{"name":123,"arguments":{}}`,
		),
		"empty name":           json.RawMessage(`{"name":"","arguments":{}}`),
		"non-object arguments": json.RawMessage(`{"name":"test_tool","arguments":"bad"}`),
	}

	for name, rawParams := range testCases {
		name := name
		rawParams := rawParams

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := NewCallHandler(NewRegistry())

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

func TestCallHandlerTreatsMissingAndNullArgumentsAsEmptyObject(t *testing.T) {
	t.Parallel()

	testCases := map[string]json.RawMessage{
		"missing arguments": json.RawMessage(`{"name":"test_tool"}`),
		"null arguments":    json.RawMessage(`{"name":"test_tool","arguments":null}`),
	}

	for name, rawParams := range testCases {
		name := name
		rawParams := rawParams

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			registry := NewRegistry()
			tool := &testTool{
				name:   "test_tool",
				result: createDirectoryResult{Path: "fixture", Created: true},
			}
			requireRegistration(t, registry, tool)

			handler := NewCallHandler(registry)

			result, rpcErr := handler.Handle(handlers.Context{}, rawParams)
			if rpcErr != nil {
				t.Fatalf("expected no error, got %v", rpcErr)
			}

			if result == nil {
				t.Fatal("expected result")
			}

			if string(tool.arguments) != `{}` {
				t.Fatalf("expected empty object arguments, got %s", string(tool.arguments))
			}
		})
	}
}

func TestCallHandlerReturnsToolError(t *testing.T) {
	registry := NewRegistry()
	requireRegistration(t, registry, &testTool{name: "test_tool", err: &toolExecutionError{category: categoryInvalidPath, message: "invalid path"}})
	result, rpcErr := NewCallHandler(registry).Handle(handlers.Context{}, json.RawMessage(`{"name":"test_tool","arguments":{}}`))
	if rpcErr != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", rpcErr)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := mcptest.DecodeCallToolResult(encoded)
	if err != nil || !decoded.IsError || decoded.HasStructuredContent {
		t.Fatalf("invalid error result: %s %v", encoded, err)
	}
	payload := decoded.TextContent.(map[string]any)
	if payload["category"] != "invalid_path" || payload["message"] != "invalid path" {
		t.Fatalf("unexpected payload: %s", encoded)
	}
}

func TestCallHandlerMethod(t *testing.T) {
	t.Parallel()

	handler := NewCallHandler(NewRegistry())

	if handler.Method() != "tools/call" {
		t.Fatalf("expected tools/call, got %q", handler.Method())
	}
}
func TestMetadataParityForFilesystemReceipts(t *testing.T) {
	tests := map[string]resultObject{
		"get_path_info existing": getPathInfoExistingResult{Path: "Folder With Spaces\\grüße.txt", Exists: true, Name: "grüße.txt", Size: 7},
		"get_path_info missing":  getPathInfoMissingResult{Path: "does-not-exist.txt", Exists: false},
		"write_file":             writeFileResult{Path: "output file.txt", Size: 4, Written: true},
		"create_directory":       createDirectoryResult{Path: "new directory", Created: true},
		"delete_path":            deletePathResult{Path: "old.txt", Deleted: true},
		"copy_path":              copyPathResult{Source: "source.txt", Target: "target.txt", Copied: true},
		"move_path":              movePathResult{Source: "old.txt", Target: "new.txt", Moved: true},
	}

	for name, domainResult := range tests {
		t.Run(name, func(t *testing.T) {
			wrapped, err := metadataContract(defaultVariant).wrap(NewToolResult(defaultVariant, domainResult))
			if err != nil {
				t.Fatal(err)
			}
			assertWrappedToolResult(t, wrapped, domainResult)
		})
	}
}

func TestWrapSuccessfulToolResultRejectsUnsafeValues(t *testing.T) {
	tests := []resultObject{
		nil,
		(*listDirectoryResult)(nil),
		listDirectoryResult{},
		untrustedResultObject{},
	}
	for _, value := range tests {
		result, err := metadataContract(defaultVariant).wrap(NewToolResult(defaultVariant, value))
		if err != errInvalidToolResult {
			t.Fatalf("expected invalid result for %#v, result=%#v error=%#v", value, result, err)
		}
	}
}

func TestCallHandlerReturnsSafeInternalErrorForUnsupportedObject(t *testing.T) {
	registry := NewRegistry()
	requireRegistration(t, registry, &testTool{name: "test_tool", result: untrustedResultObject{}})

	result, rpcErr := NewCallHandler(registry).Handle(
		handlers.Context{}, json.RawMessage(`{"name":"test_tool","arguments":{}}`),
	)
	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}
	if rpcErr == nil || rpcErr.Code != protocol.ErrInternalError || rpcErr.Message != "internal error" {
		t.Fatalf("expected safe Internal error, got %#v", rpcErr)
	}
}

func assertWrappedToolResult(t *testing.T, result protocol.CallToolResult, expected any) {
	t.Helper()
	if result.Content == nil || len(result.Content) != 1 {
		t.Fatalf("expected exactly one content block, got %#v", result.Content)
	}
	if _, ok := result.Content[0].(protocol.TextContent); !ok {
		t.Fatalf("expected text content, got %#v", result.Content[0])
	}
	if result.IsError {
		t.Fatal("successful result must not set isError")
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := mcptest.DecodeCallToolResult(encoded)
	if err != nil {
		t.Fatalf("strict CallToolResult validation failed: %v; JSON=%s", err, encoded)
	}
	if !decoded.HasStructuredContent || decoded.IsError {
		t.Fatalf("unexpected strict result: %#v", decoded)
	}

	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	var expectedValue any
	if err := json.Unmarshal(expectedJSON, &expectedValue); err != nil {
		t.Fatal(err)
	}
	actualJSON, err := json.Marshal(decoded.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var actualValue any
	if err := json.Unmarshal(actualJSON, &actualValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actualValue, expectedValue) {
		t.Fatalf("structured result differs: actual=%#v expected=%#v", actualValue, expectedValue)
	}
}
