package handlers

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

// Context is passed through the request lifecycle.
type Context struct {
	Context         context.Context
	ProtocolVersion string
}

// Handler handles a JSON-RPC/MCP method.
type Handler interface {
	Method() string
	Handle(ctx Context, params json.RawMessage) (any, *protocol.Error)
}
