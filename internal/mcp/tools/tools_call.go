package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const toolsCallMethod = "tools/call"

// CallHandler handles MCP tools/call requests.
type CallHandler struct {
	registry *Registry
}

// NewCallHandler creates a new tools/call handler.
func NewCallHandler(registry *Registry) *CallHandler {
	return &CallHandler{
		registry: registry,
	}
}

// Method returns the JSON-RPC method handled by this handler.
func (h *CallHandler) Method() string {
	return toolsCallMethod
}

// Handle executes the requested tool.
func (h *CallHandler) Handle(ctx handlers.Context, rawParams json.RawMessage) (any, *protocol.Error) {
	params, rpcErr := validateCallParams(rawParams)
	if rpcErr != nil {
		return nil, rpcErr
	}

	tool, ok := h.registry.Get(params.Name)
	if !ok {
		return nil, invalidParamsError()
	}

	execCtx := ctx.Context
	if execCtx == nil {
		execCtx = context.Background()
	}

	result, rpcErr := tool.Execute(execCtx, params.Arguments)
	if rpcErr != nil {
		wrapped, err := wrapToolError(rpcErr)
		if err != nil {
			return nil, err
		}
		return wrapped, nil
	}

	wrapped, rpcErr := wrapSuccessfulToolResult(result)
	if rpcErr != nil {
		return nil, rpcErr
	}

	return wrapped, nil
}

func wrapToolError(toolErr *protocol.Error) (protocol.CallToolResult, *protocol.Error) {
	category := toolErr.Category
	message := toolErr.Message
	if category == "" {
		category = "internal_error"
		message = "internal error"
	}
	wrapped, err := protocol.NewCallToolErrorResult(category, message)
	if err != nil {
		return protocol.CallToolResult{}, internalToolResultError()
	}
	return wrapped, nil
}

func wrapSuccessfulToolResult(result any) (protocol.CallToolResult, *protocol.Error) {
	switch result.(type) {
	case protocol.CallToolResult, *protocol.CallToolResult:
		return protocol.CallToolResult{}, internalToolResultError()
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		return protocol.CallToolResult{}, internalToolResultError()
	}

	wrapped, err := protocol.NewCallToolResult(encoded)
	if err != nil {
		return protocol.CallToolResult{}, internalToolResultError()
	}

	return wrapped, nil
}

func internalToolResultError() *protocol.Error {
	return &protocol.Error{
		Code:    protocol.ErrInternalError,
		Message: "internal error",
	}
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

func validateCallParams(rawParams json.RawMessage) (callParams, *protocol.Error) {
	if !isJSONObject(rawParams) {
		return callParams{}, invalidParamsError()
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawParams, &fields); err != nil {
		return callParams{}, invalidParamsError()
	}

	name, ok := stringField(fields, "name")
	if !ok {
		return callParams{}, invalidParamsError()
	}

	arguments, hasArguments := fields["arguments"]
	if !hasArguments || isJSONNull(arguments) {
		arguments = json.RawMessage(`{}`)
	} else if !isJSONObject(arguments) {
		return callParams{}, invalidParamsError()
	}

	return callParams{
		Name:      name,
		Arguments: arguments,
	}, nil
}
