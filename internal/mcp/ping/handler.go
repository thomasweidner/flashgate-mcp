// Package ping implements the initialized-revision MCP ping receiver.
package ping

import (
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

// Handler returns an empty result for a validated ping request.
type Handler struct{}

func (Handler) Method() string { return "ping" }

func (Handler) Handle(handlers.Context, json.RawMessage) (any, *protocol.Error) {
	return struct{}{}, nil
}
