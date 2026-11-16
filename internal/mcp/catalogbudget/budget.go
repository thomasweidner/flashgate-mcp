// Package catalogbudget checks whole-catalog field limits for an MCP result.
package catalogbudget

import (
	"encoding/json"
	"fmt"
)

type Limits struct {
	MaxTools             uint64 `json:"max_tools"`
	MaxNameBytes         uint64 `json:"max_name_bytes"`
	MaxTitleBytes        uint64 `json:"max_title_bytes"`
	MaxDescriptionBytes  uint64 `json:"max_description_bytes"`
	MaxInputSchemaBytes  uint64 `json:"max_input_schema_bytes"`
	MaxOutputSchemaBytes uint64 `json:"max_output_schema_bytes"`
}

// Check validates the actual serialized catalog using the same limits at
// startup and in executable conformance tests.
func Check(raw json.RawMessage, limit Limits) (Limits, error) {
	var result struct {
		Tools []struct {
			Name         string          `json:"name"`
			Title        string          `json:"title"`
			Description  string          `json:"description"`
			InputSchema  json.RawMessage `json:"inputSchema"`
			OutputSchema json.RawMessage `json:"outputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return Limits{}, fmt.Errorf("decode catalog fields: %w", err)
	}
	if result.Tools == nil {
		return Limits{}, fmt.Errorf("missing catalog tool array")
	}
	seen := make(map[string]bool, len(result.Tools))
	usage := Limits{MaxTools: uint64(len(result.Tools))}
	for _, tool := range result.Tools {
		if tool.Name == "" || seen[tool.Name] {
			return Limits{}, fmt.Errorf("empty or duplicate catalog tool name %q", tool.Name)
		}
		seen[tool.Name] = true
		usage.MaxNameBytes += uint64(len(tool.Name))
		usage.MaxTitleBytes += uint64(len(tool.Title))
		usage.MaxDescriptionBytes += uint64(len(tool.Description))
		usage.MaxInputSchemaBytes += uint64(len(tool.InputSchema))
		usage.MaxOutputSchemaBytes += uint64(len(tool.OutputSchema))
	}
	for _, field := range []struct {
		name        string
		actual, max uint64
	}{
		{"tool count", usage.MaxTools, limit.MaxTools},
		{"name bytes", usage.MaxNameBytes, limit.MaxNameBytes},
		{"title bytes", usage.MaxTitleBytes, limit.MaxTitleBytes},
		{"description bytes", usage.MaxDescriptionBytes, limit.MaxDescriptionBytes},
		{"input schema bytes", usage.MaxInputSchemaBytes, limit.MaxInputSchemaBytes},
		{"output schema bytes", usage.MaxOutputSchemaBytes, limit.MaxOutputSchemaBytes},
	} {
		if field.max == 0 {
			return Limits{}, fmt.Errorf("invalid zero catalog limit for %s", field.name)
		}
		if field.actual > field.max {
			return Limits{}, fmt.Errorf("catalog %s %d exceeds %d", field.name, field.actual, field.max)
		}
	}
	return usage, nil
}
