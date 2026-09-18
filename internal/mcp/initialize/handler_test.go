package initialize

import (
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestHandlerMethod(t *testing.T) {
	t.Parallel()

	handler := NewHandler("flashgate", "0.1.0-dev")

	if handler.Method() != "initialize" {
		t.Fatalf("expected initialize, got %q", handler.Method())
	}
}

func TestHandlerReturnsInitializeResult(t *testing.T) {
	t.Parallel()

	handler := NewHandler("flashgate", "0.1.0-dev")

	result, rpcErr := handler.Handle(
		handlers.Context{},
		json.RawMessage(`{
			"protocolVersion":"2025-11-25",
			"capabilities":{},
			"clientInfo":{
				"name":"test-client",
				"version":"1.0.0"
			}
		}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("expected marshalable result, got %v", err)
	}

	var decoded struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools map[string]any `json:"tools"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}

	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}

	if decoded.ProtocolVersion != protocol.ProtocolVersion {
		t.Fatalf("expected protocol version %q, got %q", protocol.ProtocolVersion, decoded.ProtocolVersion)
	}

	if decoded.ServerInfo.Name != "flashgate" {
		t.Fatalf("expected server name flashgate, got %q", decoded.ServerInfo.Name)
	}

	if decoded.ServerInfo.Version != "0.1.0-dev" {
		t.Fatalf("expected version 0.1.0-dev, got %q", decoded.ServerInfo.Version)
	}

	if decoded.Capabilities.Tools == nil {
		t.Fatal("expected tools capability")
	}
}

func TestHandlerReturnsInvalidParamsForMalformedJSON(t *testing.T) {
	t.Parallel()

	handler := NewHandler("flashgate", "0.1.0-dev")

	result, rpcErr := handler.Handle(
		handlers.Context{},
		json.RawMessage(`{"protocolVersion":`),
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

func TestHandlerReturnsInvalidParamsForMissingProtocolVersion(t *testing.T) {
	t.Parallel()

	handler := NewHandler("flashgate", "0.1.0-dev")

	result, rpcErr := handler.Handle(
		handlers.Context{},
		json.RawMessage(`{
			"capabilities":{},
			"clientInfo":{
				"name":"test-client",
				"version":"1.0.0"
			}
		}`),
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

func TestHandlerNegotiatesUnsupportedClientProtocolVersionToSupportedRevision(t *testing.T) {
	t.Parallel()

	handler := NewHandler("flashgate", "0.1.0-dev")

	result, rpcErr := handler.Handle(
		handlers.Context{},
		json.RawMessage(`{
			"protocolVersion":"2026-07-28",
			"capabilities":{},
			"clientInfo":{
				"name":"test-client",
				"version":"1.0.0"
			}
		}`),
	)

	if rpcErr != nil {
		t.Fatalf("expected no error, got %v", rpcErr)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("expected marshalable result, got %v", err)
	}

	var decoded struct {
		ProtocolVersion string `json:"protocolVersion"`
	}

	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}

	if decoded.ProtocolVersion != protocol.ProtocolVersion {
		t.Fatalf("expected server protocol version %q, got %q", protocol.ProtocolVersion, decoded.ProtocolVersion)
	}
}
