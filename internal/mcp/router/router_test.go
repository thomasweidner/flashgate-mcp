package router

import (
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestDiscoverRespectsProtocolEra(t *testing.T) {
	legacy := NewWithPolicy("flashgate", "test", revision.ProductionPolicy())
	modern := NewWithPolicy("flashgate", "test", revision.NewPolicy(protocol.ProtocolVersion, revision.Stateless))
	for _, request := range []revision.Request{
		{Version: protocol.ProtocolVersion},
		{Version: revision.Stateless, StatelessEnvelope: true},
	} {
		_, rpcErr := legacy.DispatchRevision(request, "server/discover", handlers.Context{})
		if rpcErr == nil || rpcErr.Code != protocol.ErrMethodNotFound {
			t.Fatalf("legacy-only discovery probe exposed modern era: %#v", rpcErr)
		}
	}
	_, rpcErr := modern.DispatchRevision(revision.Request{Version: protocol.ProtocolVersion}, "server/discover", handlers.Context{})
	if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
		t.Fatalf("candidate discovery without modern metadata: %#v", rpcErr)
	}
	result, rpcErr := modern.DispatchRevision(revision.Request{Version: revision.Stateless, StatelessEnvelope: true}, "server/discover", handlers.Context{})
	if rpcErr != nil || result == nil {
		t.Fatalf("candidate discovery rejected complete metadata: %#v %#v", result, rpcErr)
	}
}

func TestRouterDispatchesRegisteredHandler(t *testing.T) {
	t.Parallel()

	router := New()
	handler := &testHandler{
		method: "test/method",
		result: map[string]any{
			"ok": true,
		},
	}

	router.Register(handler)

	result, rpcErr := router.Dispatch(
		"test/method",
		handlers.Context{},
		json.RawMessage(`{"value":"abc"}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected result map, got %#v", result)
	}

	if resultMap["ok"] != true {
		t.Fatalf("expected ok=true, got %#v", resultMap["ok"])
	}

	if handler.called != 1 {
		t.Fatalf("expected handler to be called once, got %d", handler.called)
	}

	if string(handler.params) != `{"value":"abc"}` {
		t.Fatalf("expected params to be forwarded unchanged, got %s", string(handler.params))
	}
}

func TestRouterReturnsMethodNotFoundForUnknownMethod(t *testing.T) {
	t.Parallel()

	router := New()

	result, rpcErr := router.Dispatch(
		"missing/method",
		handlers.Context{},
		nil,
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}

	if rpcErr.Code != protocol.ErrMethodNotFound {
		t.Fatalf("expected ErrMethodNotFound, got %d", rpcErr.Code)
	}
}

func TestRouterReturnsHandlerError(t *testing.T) {
	t.Parallel()

	router := New()
	expectedErr := &protocol.Error{
		Code:    protocol.ErrInvalidParams,
		Message: "invalid params",
	}

	router.Register(&testHandler{
		method: "test/error",
		err:    expectedErr,
	})

	result, rpcErr := router.Dispatch(
		"test/error",
		handlers.Context{},
		json.RawMessage(`{"invalid":true}`),
	)

	if result != nil {
		t.Fatalf("expected nil result, got %#v", result)
	}

	if rpcErr != expectedErr {
		t.Fatalf("expected handler error to be returned unchanged")
	}
}

func TestRouterRegisterOverridesExistingHandler(t *testing.T) {
	t.Parallel()

	router := New()

	first := &testHandler{
		method: "test/method",
		result: "first",
	}

	second := &testHandler{
		method: "test/method",
		result: "second",
	}

	router.Register(first)
	router.Register(second)

	result, rpcErr := router.Dispatch(
		"test/method",
		handlers.Context{},
		nil,
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	if result != "second" {
		t.Fatalf("expected second handler result, got %#v", result)
	}

	if first.called != 0 {
		t.Fatalf("expected first handler not to be called, got %d", first.called)
	}

	if second.called != 1 {
		t.Fatalf("expected second handler to be called once, got %d", second.called)
	}
}

type testHandler struct {
	method string
	result any
	err    *protocol.Error
	called int
	params json.RawMessage
}

func (h *testHandler) Method() string {
	return h.method
}

func (h *testHandler) Handle(_ handlers.Context, params json.RawMessage) (any, *protocol.Error) {
	h.called++
	h.params = append(json.RawMessage(nil), params...)

	if h.err != nil {
		return nil, h.err
	}

	return h.result, nil
}
