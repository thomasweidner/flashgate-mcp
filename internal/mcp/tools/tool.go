package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

// Tool defines an executable MCP tool.
type Tool interface {
	Name() string
	Description() string
	InputSchema() any
	Definition() protocol.Tool
	Execute(ctx context.Context, arguments json.RawMessage) (any, error)
}
