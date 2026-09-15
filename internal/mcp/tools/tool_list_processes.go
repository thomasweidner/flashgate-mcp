package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	processdomain "github.com/thomasweidner/flashgate-mcp/internal/process"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const (
	listProcessesToolName  = "list_processes"
	defaultProcessPageSize = 100
	maximumProcessPageSize = 200
)

// ListProcessesTool exposes bounded, PID-ordered process observation.
type ListProcessesTool struct {
	lister processdomain.Lister
}

// NewListProcessesTool creates a list_processes tool.
func NewListProcessesTool(lister processdomain.Lister) *ListProcessesTool {
	return &ListProcessesTool{lister: lister}
}

func (t *ListProcessesTool) Name() string  { return listProcessesToolName }
func (t *ListProcessesTool) Title() string { return "List Processes" }
func (t *ListProcessesTool) Description() string {
	return "Lists a bounded page of running processes in deterministic PID order."
}
func (t *ListProcessesTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pageSize": map[string]any{
				"type": "integer", "minimum": 1, "maximum": maximumProcessPageSize,
				"description": "Maximum entries to return. Defaults to 100.",
			},
			"cursor": map[string]any{
				"type": "string", "minLength": 1,
				"description": "Opaque continuation cursor returned by the previous page.",
			},
		},
		"additionalProperties": false,
	}
}
func (t *ListProcessesTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: toolOutputSchema(t.Name())}
}

func (t *ListProcessesTool) Execute(ctx context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments listProcessesArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}

	pageSize := defaultProcessPageSize
	if arguments.PageSize != nil {
		if *arguments.PageSize < 1 || *arguments.PageSize > maximumProcessPageSize {
			return nil, invalidParamsError()
		}
		pageSize = *arguments.PageSize
	}
	lastPID, err := decodeProcessCursor(arguments.Cursor)
	if err != nil {
		return nil, invalidParamsError()
	}

	entries, err := t.lister.List(ctx)
	if err != nil {
		return nil, &protocol.Error{Code: protocol.ErrInternalError, Message: "process observation failed"}
	}
	start := 0
	for start < len(entries) && entries[start].PID <= lastPID {
		start++
	}
	end := start + pageSize
	if end > len(entries) {
		end = len(entries)
	}
	page := append([]processdomain.Entry(nil), entries[start:end]...)
	result := listProcessesResult{Processes: page, More: end < len(entries)}
	if result.More {
		result.NextCursor = encodeProcessCursor(page[len(page)-1].PID)
	}
	return result, nil
}

type listProcessesArguments struct {
	PageSize *int    `json:"pageSize,omitempty"`
	Cursor   *string `json:"cursor,omitempty"`
}

type listProcessesResult struct {
	Processes  []processdomain.Entry `json:"processes"`
	More       bool                  `json:"more"`
	NextCursor string                `json:"nextCursor,omitempty"`
}

type processCursor struct {
	Version int    `json:"v"`
	LastPID uint32 `json:"lastPid"`
}

func encodeProcessCursor(lastPID uint32) string {
	payload, _ := json.Marshal(processCursor{Version: 1, LastPID: lastPID})
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeProcessCursor(value *string) (uint32, error) {
	if value == nil {
		return 0, nil
	}
	if strings.TrimSpace(*value) != *value || *value == "" || len(*value) > 128 {
		return 0, errors.New("invalid cursor")
	}
	payload, err := base64.RawURLEncoding.DecodeString(*value)
	if err != nil || base64.RawURLEncoding.EncodeToString(payload) != *value {
		return 0, errors.New("invalid cursor")
	}
	var cursor processCursor
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.Version != 1 || cursor.LastPID == 0 {
		return 0, fmt.Errorf("invalid cursor")
	}
	if decoder.Decode(&struct{}{}) == nil {
		return 0, errors.New("invalid cursor")
	}
	return cursor.LastPID, nil
}
