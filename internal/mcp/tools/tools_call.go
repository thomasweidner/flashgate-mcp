package tools

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
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

	execCtx := ctx.Context
	if execCtx == nil {
		if h.registry.IsCatalog() {
			return nil, invalidParamsError()
		}
		execCtx = context.Background()
	}

	ctx.Context = execCtx
	var entry registeredTool
	var binding execution.CatalogContext
	var ok bool
	if h.registry.IsCatalog() {
		view, scope, err := h.registry.activeView(ctx)
		if err != nil {
			return nil, invalidParamsError()
		}
		binding = scope
		entry, ok = view.entries[params.Name]
	} else {
		entry, ok = h.registry.lookup(params.Name)
	}
	if !ok {
		return nil, invalidParamsError()
	}
	result, err := entry.invoke(ctx, binding, params.Arguments)
	if err == nil {
		wrapped, wrapErr := entry.contract.wrap(execCtx, result)
		if wrapErr == nil {
			if observe, _ := execCtx.Value(resultObserverKey{}).(func(string)); observe != nil {
				observe(result.Variant())
			}
			return wrapped, nil
		}
		err = wrapErr
		if errors.Is(wrapErr, errToolResultLimit) {
			err = mapFilesystemError(fs.ErrLimitExceeded)
		}
	}
	var expected *toolExecutionError
	if errors.As(err, &expected) && expected != nil {
		wrapped, wrapErr := protocol.NewCallToolErrorResultContext(execCtx, string(expected.category), expected.message)
		if wrapErr == nil {
			return wrapped, nil
		}
	}
	return nil, internalToolResultError()
}

type resultObserverKey struct{}

// ObserveResults attaches an in-process observer for validated tool results.
func ObserveResults(ctx context.Context, observe func(string)) context.Context {
	return context.WithValue(ctx, resultObserverKey{}, observe)
}

func (tool registeredTool) invoke(ctx handlers.Context, binding execution.CatalogContext, arguments json.RawMessage) (ToolResult, error) {
	if tool.bound != nil {
		invocation, err := binding.BindInvocation(tool.route, tool.variants)
		if err != nil {
			return ToolResult{}, err
		}
		ctx.Context = invocation.Context(ctx.Context)
		return tool.bound(ctx, invocation, arguments)
	}
	return tool.execute(ctx, arguments)
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
