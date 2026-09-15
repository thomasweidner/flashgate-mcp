package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/systeminfo"
)

const systemInfoToolName = "system_info"

// SystemInfoTool exposes a deliberately small set of non-identifying host facts.
type SystemInfoTool struct{ provider systeminfo.Provider }

// NewSystemInfoTool creates a system_info tool.
func NewSystemInfoTool(provider systeminfo.Provider) *SystemInfoTool {
	return &SystemInfoTool{provider: provider}
}

func (t *SystemInfoTool) Name() string  { return systemInfoToolName }
func (t *SystemInfoTool) Title() string { return "System Info" }
func (t *SystemInfoTool) Description() string {
	return "Returns the operating system, architecture, and OS version without machine identifiers."
}
func (t *SystemInfoTool) InputSchema() any {
	return map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}
}
func (t *SystemInfoTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: systemInfoOutputSchema()}
}
func (t *SystemInfoTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments struct{}
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}

	info, err := t.provider.Info()
	if err != nil || info.OS == "" || info.Architecture == "" || info.Version == "" {
		return nil, internalToolResultError()
	}
	return systemInfoResult{OS: info.OS, Architecture: info.Architecture, Version: info.Version}, nil
}

type systemInfoResult struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Version      string `json:"version"`
}
