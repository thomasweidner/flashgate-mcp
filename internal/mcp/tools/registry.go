package tools

import (
	"context"
	"encoding/json"
	"slices"
	"sync"

	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
)

type registeredTool struct {
	tool     Tool
	contract ResultContract
	execute  func(handlers.Context, json.RawMessage) (ToolResult, error)
}

// Registry stores MCP tools.
type Registry struct {
	tools map[string]registeredTool
	order []string
	mu    sync.RWMutex
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]registeredTool),
		order: make([]string, 0),
	}
}

// Register registers or replaces a tool.
func (r *Registry) Register(tool Tool) error {
	return r.register(tool, func(ctx handlers.Context, arguments json.RawMessage) (ToolResult, error) {
		return tool.Execute(ctx.Context, arguments)
	})
}

// RegisterBound separates catalog metadata from invocation requiring a
// validated protocol and a request-bound application service.
func (r *Registry) RegisterBound(metadata Tool, execute func(handlers.Context, json.RawMessage) (ToolResult, error)) error {
	if execute == nil {
		return execution.ErrMismatch
	}
	return r.register(boundMetadata{metadata}, execute)
}

type boundMetadata struct{ Tool }

func (boundMetadata) Execute(context.Context, json.RawMessage) (ToolResult, error) {
	return ToolResult{}, execution.ErrMismatch
}

func (r *Registry) register(tool Tool, execute func(handlers.Context, json.RawMessage) (ToolResult, error)) error {
	contract, err := tool.ResultContract().validatedCopy()
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()

	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}

	r.tools[name] = registeredTool{tool: tool, contract: contract, execute: execute}
	return nil
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	entry, ok := r.lookup(name)
	return entry.tool, ok
}

// ResultContract returns an independent copy of the validated contract
// captured at registration and used by the call adapter.
func (r *Registry) ResultContract(name string) ResultContract {
	entry, _ := r.lookup(name)
	contract := slices.Clone(entry.contract)
	for index := range contract {
		contract[index].Parts = slices.Clone(contract[index].Parts)
	}
	return contract
}

func (r *Registry) lookup(name string) (registeredTool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools in registration order.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Tool, 0, len(r.order))

	for _, name := range r.order {
		tool, ok := r.tools[name]
		if ok {
			result = append(result, tool.tool)
		}
	}

	return result
}
