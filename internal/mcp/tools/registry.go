package tools

import "sync"

type registeredTool struct {
	tool     Tool
	contract ResultContract
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

	r.tools[name] = registeredTool{tool: tool, contract: contract}
	return nil
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	entry, ok := r.lookup(name)
	return entry.tool, ok
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
