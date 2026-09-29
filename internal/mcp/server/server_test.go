package server

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestServerRunHandlesValidRequest(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test/ok","params":{"value":"abc"}}` + "\n")
	output := &bytes.Buffer{}

	testRouter := router.New()
	testRouter.Register(&testHandler{
		method: "test/ok",
		result: map[string]any{
			"ok": true,
		},
	})

	server := New(input, output, testRouter)

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	response := decodeSingleResponse(t, output.String())

	if response.JSONRPC != protocol.JSONRPCVersion {
		t.Fatalf("expected JSON-RPC version %q, got %q", protocol.JSONRPCVersion, response.JSONRPC)
	}

	if string(response.ID) != "1" {
		t.Fatalf("expected id 1, got %s", string(response.ID))
	}

	if response.Error != nil {
		t.Fatalf("expected no error, got %#v", response.Error)
	}

	result, ok := response.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected result map, got %#v", response.Result)
	}

	if result["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", result["ok"])
	}
}

func TestRuntimeHandlesRequestWithoutTransport(t *testing.T) {
	t.Parallel()

	testRouter := router.New()
	testRouter.Register(&testHandler{
		method: "test/ok",
		result: map[string]any{"ok": true},
	})
	runtime := NewRuntime(testRouter, Options{})

	response := runtime.Handle(
		context.Background(),
		[]byte(`{"jsonrpc":"2.0","id":"runtime","method":"test/ok"}`),
	)
	if response == nil {
		t.Fatal("expected response")
	}
	if string(response.ID) != `"runtime"` || response.Error != nil {
		t.Fatalf("unexpected response: %#v", response)
	}

	notification := runtime.Handle(
		context.Background(),
		[]byte(`{"jsonrpc":"2.0","method":"test/ok"}`),
	)
	if notification != nil {
		t.Fatalf("expected no notification response, got %#v", notification)
	}
}

func TestServerRunReturnsMethodNotFound(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"missing/method"}` + "\n")
	output := &bytes.Buffer{}

	server := New(input, output, router.New())

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	response := decodeSingleResponse(t, output.String())

	if string(response.ID) != "1" {
		t.Fatalf("expected id 1, got %s", string(response.ID))
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}

	if response.Error.Code != protocol.ErrMethodNotFound {
		t.Fatalf("expected ErrMethodNotFound, got %d", response.Error.Code)
	}

	if response.Error.Message != "method not found" {
		t.Fatalf("expected generic method not found message, got %q", response.Error.Message)
	}
}

func TestServerRunReturnsParseErrorForInvalidJSON(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":` + "\n")
	output := &bytes.Buffer{}

	server := New(input, output, router.New())

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	response := decodeSingleResponse(t, output.String())

	if response.Error == nil {
		t.Fatal("expected parse error")
	}

	if response.Error.Code != protocol.ErrParseError {
		t.Fatalf("expected ErrParseError, got %d", response.Error.Code)
	}

	if response.Error.Message != "parse error" {
		t.Fatalf("expected parse error message, got %q", response.Error.Message)
	}

	if string(response.ID) != "null" {
		t.Fatalf("expected null id, got %s", string(response.ID))
	}
}

func TestServerRunReturnsInvalidRequestForMessageOverLimit(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"padding":"too-large"}}` + "\n")
	output := &bytes.Buffer{}
	server := NewWithOptions(input, output, router.New(), Options{MaxMessageBytes: 20})

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	response := decodeSingleResponse(t, output.String())

	if response.Error == nil {
		t.Fatal("expected error response")
	}

	if response.Error.Code != protocol.ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest, got %d", response.Error.Code)
	}

	if response.Error.Message != "invalid request" {
		t.Fatalf("expected invalid request, got %q", response.Error.Message)
	}

	if string(response.ID) != "null" {
		t.Fatalf("expected null id, got %s", string(response.ID))
	}
}

func TestServerRunReturnsInvalidParamsForToolArgumentsOverLimit(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_tool","arguments":{"content":"too-large"}}}` + "\n")
	output := &bytes.Buffer{}
	handler := &testHandler{
		method: "tools/call",
		result: map[string]any{
			"ok": true,
		},
	}
	testRouter := router.New()
	testRouter.Register(handler)
	server := NewWithOptions(input, output, testRouter, Options{MaxArgumentBytes: 10})

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	response := decodeSingleResponse(t, output.String())

	if response.Error == nil {
		t.Fatal("expected error response")
	}

	if response.Error.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %d", response.Error.Code)
	}

	if response.Error.Message != "invalid params" {
		t.Fatalf("expected invalid params, got %q", response.Error.Message)
	}

	if handler.called != 0 {
		t.Fatalf("expected handler not to be called, got %d", handler.called)
	}
}

func TestServerRunReturnsInternalErrorForResponseOverLimit(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test/large"}` + "\n")
	output := &bytes.Buffer{}
	testRouter := router.New()
	testRouter.Register(&testHandler{
		method: "test/large",
		result: map[string]any{
			"content": strings.Repeat("x", 100),
		},
	})
	server := NewWithOptions(input, output, testRouter, Options{MaxResponseBytes: 100})

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	response := decodeSingleResponse(t, output.String())

	if string(response.ID) != "1" {
		t.Fatalf("expected id 1, got %s", string(response.ID))
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}

	if response.Error.Code != protocol.ErrInternalError {
		t.Fatalf("expected ErrInternalError, got %d", response.Error.Code)
	}

	if response.Error.Message != "internal error" {
		t.Fatalf("expected internal error, got %q", response.Error.Message)
	}
}

func TestServerRunReturnsInvalidRequestForInvalidEnvelope(t *testing.T) {
	t.Parallel()

	testCases := map[string]string{
		"top-level array":   `[]`,
		"top-level string":  `"request"`,
		"top-level number":  `1`,
		"top-level null":    `null`,
		"empty object":      `{}`,
		"missing jsonrpc":   `{"id":1,"method":"tools/list"}`,
		"wrong jsonrpc":     `{"jsonrpc":"1.0","id":1,"method":"tools/list"}`,
		"missing method":    `{"jsonrpc":"2.0","id":1}`,
		"empty method":      `{"jsonrpc":"2.0","id":1,"method":""}`,
		"blank method":      `{"jsonrpc":"2.0","id":1,"method":"   "}`,
		"non-string method": `{"jsonrpc":"2.0","id":1,"method":123}`,
		"object id":         `{"jsonrpc":"2.0","id":{},"method":"tools/list"}`,
		"array id":          `{"jsonrpc":"2.0","id":[],"method":"tools/list"}`,
		"boolean id":        `{"jsonrpc":"2.0","id":true,"method":"tools/list"}`,
		"batch unsupported": `[{"jsonrpc":"2.0","id":1,"method":"tools/list"}]`,
	}

	for name, input := range testCases {
		name := name
		input := input

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			output := &bytes.Buffer{}
			server := New(strings.NewReader(input+"\n"), output, router.New())

			if err := server.Run(context.Background()); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			response := decodeSingleResponse(t, output.String())

			if response.Error == nil {
				t.Fatal("expected error response")
			}

			if response.Error.Code != protocol.ErrInvalidRequest {
				t.Fatalf("expected ErrInvalidRequest, got %d", response.Error.Code)
			}

			if response.Error.Message != "invalid request" {
				t.Fatalf("expected invalid request message, got %q", response.Error.Message)
			}

			if name == "object id" || name == "array id" || name == "boolean id" ||
				name == "top-level array" || name == "top-level string" ||
				name == "top-level number" || name == "top-level null" ||
				name == "empty object" || name == "batch unsupported" {
				if string(response.ID) != "null" {
					t.Fatalf("expected null id, got %s", string(response.ID))
				}
			}
		})
	}
}

func TestServerRunEchoesValidIDsForRequestErrors(t *testing.T) {
	t.Parallel()

	testCases := map[string]string{
		"string id": `"request-1"`,
		"number id": `42`,
		"null id":   `null`,
	}

	for name, id := range testCases {
		name := name
		id := id

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			input := `{"jsonrpc":"2.0","id":` + id + `,"method":"missing/method"}` + "\n"
			output := &bytes.Buffer{}
			server := New(strings.NewReader(input), output, router.New())

			if err := server.Run(context.Background()); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			response := decodeSingleResponse(t, output.String())

			if string(response.ID) != id {
				t.Fatalf("expected id %s, got %s", id, string(response.ID))
			}

			if response.Error == nil || response.Error.Code != protocol.ErrMethodNotFound {
				t.Fatalf("expected method not found error, got %#v", response.Error)
			}
		})
	}
}

func TestServerRunValidatesMethodParams(t *testing.T) {
	t.Parallel()

	testCases := map[string]string{
		"initialize missing params":           `{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		"initialize null params":              `{"jsonrpc":"2.0","id":1,"method":"initialize","params":null}`,
		"initialize missing protocol version": `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		"initialize non-string version":       `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":123}}`,
		"tools list string params":            `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":"bad"}`,
		"tools list array params":             `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":[]}`,
		"tools list number params":            `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":1}`,
		"tools list boolean params":           `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":true}`,
		"tools call missing params":           `{"jsonrpc":"2.0","id":1,"method":"tools/call"}`,
		"tools call null params":              `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":null}`,
		"tools call string params":            `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"bad"}`,
		"tools call array params":             `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":[]}`,
		"tools call number params":            `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":1}`,
		"tools call boolean params":           `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":true}`,
		"tools call missing name":             `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"arguments":{}}}`,
		"tools call empty name":               `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"","arguments":{}}}`,
		"tools call non-string name":          `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":123,"arguments":{}}}`,
		"tools call non-object arguments":     `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_tool","arguments":"bad"}}`,
	}

	for name, input := range testCases {
		name := name
		input := input

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			output := &bytes.Buffer{}
			server := New(strings.NewReader(input+"\n"), output, router.New())

			if err := server.Run(context.Background()); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			response := decodeSingleResponse(t, output.String())

			if response.Error == nil {
				t.Fatal("expected error response")
			}

			if response.Error.Code != protocol.ErrInvalidParams {
				t.Fatalf("expected ErrInvalidParams, got %d", response.Error.Code)
			}

			if response.Error.Message != "invalid params" {
				t.Fatalf("expected invalid params message, got %q", response.Error.Message)
			}
		})
	}
}

func TestServerRunAllowsValidToolsListParams(t *testing.T) {
	t.Parallel()

	testCases := map[string]string{
		"missing params": `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		"null params":    `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":null}`,
		"object params":  `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"extra":true}}`,
	}

	for name, input := range testCases {
		name := name
		input := input

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			output := &bytes.Buffer{}
			testRouter := router.New()
			testRouter.Register(&testHandler{
				method: "tools/list",
				result: map[string]any{
					"tools": []any{},
				},
			})
			server := New(strings.NewReader(input+"\n"), output, testRouter)

			if err := server.Run(context.Background()); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			response := decodeSingleResponse(t, output.String())

			if response.Error != nil {
				t.Fatalf("expected no error, got %#v", response.Error)
			}
		})
	}
}

func TestServerRunNormalizesMissingToolsCallArguments(t *testing.T) {
	t.Parallel()

	testCases := map[string]string{
		"missing arguments": `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_tool"}}`,
		"null arguments":    `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_tool","arguments":null}}`,
	}

	for name, input := range testCases {
		name := name
		input := input

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			output := &bytes.Buffer{}
			handler := &testHandler{
				method: "tools/call",
				result: map[string]any{
					"ok": true,
				},
			}
			testRouter := router.New()
			testRouter.Register(handler)
			server := New(strings.NewReader(input+"\n"), output, testRouter)

			if err := server.Run(context.Background()); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			response := decodeSingleResponse(t, output.String())

			if response.Error != nil {
				t.Fatalf("expected no error, got %#v", response.Error)
			}

			if !strings.Contains(string(handler.params), `"arguments":{}`) {
				t.Fatalf("expected normalized empty arguments object, got %s", string(handler.params))
			}
		})
	}
}

func TestServerRunDoesNotRespondToNotifications(t *testing.T) {
	t.Parallel()

	testCases := map[string]string{
		"initialized": `{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		"unknown":     `{"jsonrpc":"2.0","method":"unknown/notification","params":{}}`,
	}

	for name, input := range testCases {
		name := name
		input := input

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			output := &bytes.Buffer{}
			server := New(strings.NewReader(input+"\n"), output, router.New())

			if err := server.Run(context.Background()); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if output.Len() != 0 {
				t.Fatalf("expected no response for notification, got %q", output.String())
			}
		})
	}
}

func TestServerRunDoesNotExecuteToolCallNotification(t *testing.T) {
	t.Parallel()

	input := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"test_tool","arguments":{}}}` + "\n"
	output := &bytes.Buffer{}
	handler := &testHandler{
		method: "tools/call",
		result: map[string]any{
			"ok": true,
		},
	}
	testRouter := router.New()
	testRouter.Register(handler)
	server := New(strings.NewReader(input), output, testRouter)

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if output.Len() != 0 {
		t.Fatalf("expected no response for notification, got %q", output.String())
	}

	if handler.called != 0 {
		t.Fatalf("expected handler not to be called, got %d", handler.called)
	}
}

func TestServerRunReturnsInternalErrorForHandlerPanic(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test/panic"}` + "\n")
	output := &bytes.Buffer{}

	testRouter := router.New()
	testRouter.Register(&testHandler{
		method: "test/panic",
		panic:  true,
	})

	server := New(input, output, testRouter)

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	response := decodeSingleResponse(t, output.String())

	if string(response.ID) != "1" {
		t.Fatalf("expected id 1, got %s", string(response.ID))
	}

	if response.Error == nil {
		t.Fatal("expected error response")
	}

	if response.Error.Code != protocol.ErrInternalError {
		t.Fatalf("expected ErrInternalError, got %d", response.Error.Code)
	}

	if response.Error.Message != "internal error" {
		t.Fatalf("expected internal error message, got %q", response.Error.Message)
	}
}

func TestServerRunReturnsHandlerError(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test/error"}` + "\n")
	output := &bytes.Buffer{}

	testRouter := router.New()
	testRouter.Register(&testHandler{
		method: "test/error",
		err: &protocol.Error{
			Code:    protocol.ErrInvalidParams,
			Message: "invalid params",
		},
	})

	server := New(input, output, testRouter)

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	response := decodeSingleResponse(t, output.String())

	if response.Error == nil {
		t.Fatal("expected error response")
	}

	if response.Error.Code != protocol.ErrInvalidParams {
		t.Fatalf("expected ErrInvalidParams, got %d", response.Error.Code)
	}

	if response.Error.Message != "invalid params" {
		t.Fatalf("expected error message %q, got %q", "invalid params", response.Error.Message)
	}
}

func TestServerRunHandlesMultipleRequests(t *testing.T) {
	t.Parallel()

	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"test/one"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"test/two"}` + "\n",
	)
	output := &bytes.Buffer{}

	testRouter := router.New()
	testRouter.Register(&testHandler{
		method: "test/one",
		result: map[string]any{
			"name": "one",
		},
	})
	testRouter.Register(&testHandler{
		method: "test/two",
		result: map[string]any{
			"name": "two",
		},
	})

	server := New(input, output, testRouter)

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	responses := decodeResponses(t, output.String())

	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}

	if string(responses[0].ID) != "1" {
		t.Fatalf("expected first id 1, got %s", string(responses[0].ID))
	}

	if string(responses[1].ID) != "2" {
		t.Fatalf("expected second id 2, got %s", string(responses[1].ID))
	}
}

func TestServerRunReturnsNilOnEOF(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("")
	output := &bytes.Buffer{}

	server := New(input, output, router.New())

	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("expected nil on EOF, got %v", err)
	}

	if output.Len() != 0 {
		t.Fatalf("expected no output, got %q", output.String())
	}
}

type testHandler struct {
	method string
	result any
	err    *protocol.Error
	panic  bool
	called int
	params json.RawMessage
}

func (h *testHandler) Method() string {
	return h.method
}

func (h *testHandler) Handle(_ handlers.Context, params json.RawMessage) (any, *protocol.Error) {
	h.called++
	h.params = append(json.RawMessage(nil), params...)

	if h.panic {
		panic("test panic")
	}

	if h.err != nil {
		return nil, h.err
	}

	return h.result, nil
}

type decodedResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *protocol.Error `json:"error,omitempty"`
}

func decodeSingleResponse(t *testing.T, output string) decodedResponse {
	t.Helper()

	responses := decodeResponses(t, output)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d: %q", len(responses), output)
	}

	return responses[0]
}

func decodeResponses(t *testing.T, output string) []decodedResponse {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	responses := make([]decodedResponse, 0, len(lines))

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var response decodedResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("expected valid response JSON, got %v: %q", err, line)
		}

		responses = append(responses, response)
	}

	return responses
}
