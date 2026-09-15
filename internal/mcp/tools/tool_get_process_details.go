package tools

import (
	"context"
	"encoding/json"
	"errors"

	processdomain "github.com/thomasweidner/flashgate-mcp/internal/process"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const getProcessDetailsToolName = "get_process_details"

var processDetailFields = []string{"name", "parentPid", "threadCount"}

// GetProcessDetailsTool exposes explicitly selected, portable metadata for one
// PID. BL-118 owns runtime registration and authorization.
type GetProcessDetailsTool struct{ detailer processdomain.Detailer }

func NewGetProcessDetailsTool(detailer processdomain.Detailer) *GetProcessDetailsTool {
	return &GetProcessDetailsTool{detailer: detailer}
}
func (t *GetProcessDetailsTool) Name() string  { return getProcessDetailsToolName }
func (t *GetProcessDetailsTool) Title() string { return "Get Process Details" }
func (t *GetProcessDetailsTool) Description() string {
	return "Returns explicitly selected portable details for one running process."
}
func (t *GetProcessDetailsTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pid": map[string]any{"type": "integer", "minimum": 1},
			"fields": map[string]any{
				"type": "array", "minItems": 1, "maxItems": len(processDetailFields), "uniqueItems": true,
				"items": map[string]any{"type": "string", "enum": processDetailFields},
			},
		},
		"required": []string{"pid", "fields"}, "additionalProperties": false,
	}
}
func (t *GetProcessDetailsTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: toolOutputSchema(t.Name())}
}
func (t *GetProcessDetailsTool) Execute(ctx context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments getProcessDetailsArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil || arguments.PID == 0 || !validProcessDetailFields(arguments.Fields) {
		return nil, invalidParamsError()
	}
	details, err := t.detailer.Details(ctx, arguments.PID)
	if err != nil {
		switch {
		case errors.Is(err, processdomain.ErrNotFound):
			return nil, &protocol.Error{Code: protocol.ErrInternalError, Message: "process not found"}
		case errors.Is(err, processdomain.ErrAccessDenied):
			return nil, &protocol.Error{Code: protocol.ErrInternalError, Message: "process access denied"}
		default:
			return nil, &protocol.Error{Code: protocol.ErrInternalError, Message: "process observation failed"}
		}
	}
	result := getProcessDetailsResult{PID: details.PID}
	for _, field := range arguments.Fields {
		switch field {
		case "name":
			result.Name = &details.Name
		case "parentPid":
			result.ParentPID = &details.ParentPID
		case "threadCount":
			result.ThreadCount = &details.ThreadCount
		}
	}
	return result, nil
}

func validProcessDetailFields(fields []string) bool {
	if len(fields) == 0 || len(fields) > len(processDetailFields) {
		return false
	}
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if field != "name" && field != "parentPid" && field != "threadCount" {
			return false
		}
		if _, duplicate := seen[field]; duplicate {
			return false
		}
		seen[field] = struct{}{}
	}
	return true
}

type getProcessDetailsArguments struct {
	PID    uint32   `json:"pid"`
	Fields []string `json:"fields"`
}
type getProcessDetailsResult struct {
	PID         uint32  `json:"pid"`
	Name        *string `json:"name,omitempty"`
	ParentPID   *uint32 `json:"parentPid,omitempty"`
	ThreadCount *uint32 `json:"threadCount,omitempty"`
}
