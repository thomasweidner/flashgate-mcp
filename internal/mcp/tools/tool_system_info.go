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
	return "Returns operating-system facts and allowlisted locale/terminal environment values without machine identifiers or secrets."
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
	environment, ok := releasedEnvironment(info.Environment)
	if !ok {
		return nil, internalToolResultError()
	}
	return systemInfoResult{OS: info.OS, Architecture: info.Architecture, Version: info.Version, Environment: environment}, nil
}

func releasedEnvironment(values map[string]string) (map[string]string, bool) {
	allowed := make(map[string]struct{}, len(systeminfo.ReleasedEnvironmentVariables()))
	for _, name := range systeminfo.ReleasedEnvironmentVariables() {
		allowed[name] = struct{}{}
	}
	result := make(map[string]string, len(values))
	for name, value := range values {
		if _, ok := allowed[name]; !ok || value == "" {
			return nil, false
		}
		result[name] = value
	}
	return result, true
}

type systemInfoResult struct {
	OS           string            `json:"os"`
	Architecture string            `json:"architecture"`
	Version      string            `json:"version"`
	Environment  map[string]string `json:"environment"`
}
