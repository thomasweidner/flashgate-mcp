package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/systeminfo"
)

const systemInfoToolName = "system_info"

var systemInfoFields = []string{"os", "architecture", "version"}

// SystemInfoTool exposes a deliberately small set of non-identifying host facts.
type SystemInfoTool struct{ provider systeminfo.Provider }

// NewSystemInfoTool creates a system_info tool.
func NewSystemInfoTool(provider systeminfo.Provider) *SystemInfoTool {
	return &SystemInfoTool{provider: provider}
}

func (t *SystemInfoTool) Name() string  { return systemInfoToolName }
func (t *SystemInfoTool) Title() string { return "System Info" }
func (t *SystemInfoTool) Description() string {
	return "Returns selected operating system facts without machine identifiers."
}
func (t *SystemInfoTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"fields": map[string]any{
				"type":        "array",
				"description": "Fields to return. Defaults to all allowed fields.",
				"items":       map[string]any{"type": "string", "enum": systemInfoFields},
				"minItems":    1,
				"uniqueItems": true,
			},
		},
		"additionalProperties": false,
	}
}
func (t *SystemInfoTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: systemInfoOutputSchema()}
}
func (t *SystemInfoTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments struct {
		Fields *[]string `json:"fields,omitempty"`
	}
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}
	fields := systemInfoFields
	if arguments.Fields != nil {
		if !validSystemInfoFields(*arguments.Fields) {
			return nil, invalidParamsError()
		}
		fields = *arguments.Fields
	}

	info, err := t.provider.Info()
	if err != nil {
		return nil, internalToolResultError()
	}
	result := make(map[string]string, len(fields))
	for _, field := range fields {
		var value string
		switch field {
		case "os":
			value = info.OS
		case "architecture":
			value = info.Architecture
		case "version":
			value = info.Version
		}
		if value == "" {
			return nil, internalToolResultError()
		}
		result[field] = value
	}
	return result, nil
}

func validSystemInfoFields(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if _, duplicate := seen[field]; duplicate {
			return false
		}
		seen[field] = struct{}{}
		switch field {
		case "os", "architecture", "version":
		default:
			return false
		}
	}
	return true
}
