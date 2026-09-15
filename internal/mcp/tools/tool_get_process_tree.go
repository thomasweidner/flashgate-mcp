package tools

import (
	"context"
	"encoding/json"
	"sort"

	processdomain "github.com/thomasweidner/flashgate-mcp/internal/process"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const (
	getProcessTreeToolName  = "get_process_tree"
	defaultProcessTreeDepth = 3
	maximumProcessTreeDepth = 8
	maximumProcessTreeNodes = 200
)

// GetProcessTreeTool exposes a bounded descendant tree rooted at one PID.
// BL-118 owns runtime registration and authorization.
type GetProcessTreeTool struct{ observer processdomain.TreeObserver }

func NewGetProcessTreeTool(observer processdomain.TreeObserver) *GetProcessTreeTool {
	return &GetProcessTreeTool{observer: observer}
}

func (t *GetProcessTreeTool) Name() string  { return getProcessTreeToolName }
func (t *GetProcessTreeTool) Title() string { return "Get Process Tree" }
func (t *GetProcessTreeTool) Description() string {
	return "Returns a bounded descendant tree from a point-in-time process snapshot."
}
func (t *GetProcessTreeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pid": map[string]any{"type": "integer", "minimum": 1},
			"maxDepth": map[string]any{
				"type": "integer", "minimum": 0, "maximum": maximumProcessTreeDepth,
				"description": "Maximum descendant depth. Defaults to 3.",
			},
		},
		"required": []string{"pid"}, "additionalProperties": false,
	}
}
func (t *GetProcessTreeTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: toolOutputSchema(t.Name())}
}

func (t *GetProcessTreeTool) Execute(ctx context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments getProcessTreeArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil || arguments.PID == 0 {
		return nil, invalidParamsError()
	}
	maxDepth := defaultProcessTreeDepth
	if arguments.MaxDepth != nil {
		if *arguments.MaxDepth < 0 || *arguments.MaxDepth > maximumProcessTreeDepth {
			return nil, invalidParamsError()
		}
		maxDepth = *arguments.MaxDepth
	}
	snapshot, err := t.observer.Tree(ctx)
	if err != nil {
		return nil, &protocol.Error{Code: protocol.ErrInternalError, Message: "process observation failed"}
	}
	result, found := buildProcessTree(snapshot.Processes, arguments.PID, maxDepth)
	if !found {
		return nil, &protocol.Error{Code: protocol.ErrInternalError, Message: "process not found"}
	}
	result.Partial = result.Partial || snapshot.Partial
	return result, nil
}

func buildProcessTree(snapshot []processdomain.Details, rootPID uint32, maxDepth int) (getProcessTreeResult, bool) {
	byPID := make(map[uint32]processdomain.Details, len(snapshot))
	children := make(map[uint32][]uint32)
	for _, entry := range snapshot {
		if entry.PID == 0 || entry.Name == "" {
			continue
		}
		byPID[entry.PID] = entry
		children[entry.ParentPID] = append(children[entry.ParentPID], entry.PID)
	}
	if _, ok := byPID[rootPID]; !ok {
		return getProcessTreeResult{}, false
	}
	for parent := range children {
		sort.Slice(children[parent], func(i, j int) bool { return children[parent][i] < children[parent][j] })
	}
	type pendingNode struct {
		pid   uint32
		depth int
	}
	queue := []pendingNode{{pid: rootPID}}
	result := getProcessTreeResult{RootPID: rootPID, Processes: make([]processTreeNode, 0)}
	seen := make(map[uint32]struct{})
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, duplicate := seen[current.pid]; duplicate {
			continue
		}
		seen[current.pid] = struct{}{}
		entry, ok := byPID[current.pid]
		if !ok {
			result.Partial = true
			continue
		}
		if len(result.Processes) == maximumProcessTreeNodes {
			result.Truncated = true
			break
		}
		result.Processes = append(result.Processes, processTreeNode{PID: entry.PID, ParentPID: entry.ParentPID, Name: entry.Name, Depth: current.depth})
		if current.depth == maxDepth {
			if len(children[current.pid]) > 0 {
				result.Truncated = true
			}
			continue
		}
		for _, child := range children[current.pid] {
			queue = append(queue, pendingNode{pid: child, depth: current.depth + 1})
		}
	}
	return result, true
}

type getProcessTreeArguments struct {
	PID      uint32 `json:"pid"`
	MaxDepth *int   `json:"maxDepth,omitempty"`
}
type processTreeNode struct {
	PID       uint32 `json:"pid"`
	ParentPID uint32 `json:"parentPid"`
	Name      string `json:"name"`
	Depth     int    `json:"depth"`
}
type getProcessTreeResult struct {
	RootPID   uint32            `json:"rootPid"`
	Processes []processTreeNode `json:"processes"`
	Truncated bool              `json:"truncated"`
	Partial   bool              `json:"partial"`
}
