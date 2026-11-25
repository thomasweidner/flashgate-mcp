package tools

import (
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

// ListHandler handles the MCP tools/list method.
type ListHandler struct {
	registry *Registry
}

// NewListHandler creates a new tools/list handler.
func NewListHandler(registry *Registry) *ListHandler {
	return &ListHandler{
		registry: registry,
	}
}

// Method returns the MCP method name.
func (h *ListHandler) Method() string {
	return "tools/list"
}

// Handle handles the tools/list request.
func (h *ListHandler) Handle(ctx handlers.Context, rawParams json.RawMessage) (any, *protocol.Error) {
	if !isMissingNullOrObject(rawParams) {
		return nil, invalidParamsError()
	}

	if h.registry.IsCatalog() {
		view, _, err := h.registry.activeView(ctx)
		if err != nil {
			return nil, invalidParamsError()
		}
		return map[string]any{"tools": view.wireDefinitions()}, nil
	}
	registeredTools := h.registry.List()

	result := make([]protocol.Tool, 0, len(registeredTools))
	for _, tool := range registeredTools {
		result = append(result, tool.Definition())
	}

	return map[string]any{
		"tools": result,
	}, nil
}
