package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const jsonSchema202012Dialect = "https://json-schema.org/draft/2020-12/schema"

// TestToolSchemasConformToJSONSchema202012 is the permanent gate for every
// schema currently advertised by tools/list and by the static catalog. It
// validates the complete keyword vocabulary used by FlashGate's tool schemas;
// adding a new keyword fails closed until this validator is extended.
func TestToolSchemasConformToJSONSchema202012(t *testing.T) {
	for _, tool := range filesystemRuntimeTools(newFakeFileSystem()) {
		definition := tool.Definition()
		validateToolSchemaDocument(t, definition.Name+" inputSchema", normalizeSchema(t, definition.InputSchema))
		validateToolSchemaDocument(t, definition.Name+" outputSchema", normalizeSchema(t, definition.OutputSchema))
	}

	catalogPath := filepath.Join("..", "..", "..", "docs", "mcp-tool-catalog.json")
	raw, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Tools []struct {
			Name         string         `json:"name"`
			InputSchema  map[string]any `json:"inputSchema"`
			ResultSchema map[string]any `json:"resultSchema"`
		} `json:"tools"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&catalog); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	for _, tool := range catalog.Tools {
		validateToolSchemaDocument(t, "catalog "+tool.Name+" inputSchema", tool.InputSchema)
		validateToolSchemaDocument(t, "catalog "+tool.Name+" resultSchema", tool.ResultSchema)
	}
}

func TestToolSchemaValidatorFailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]any
	}{
		{"missing dialect", map[string]any{"type": "object"}},
		{"wrong dialect", map[string]any{"$schema": "http://json-schema.org/draft-07/schema#", "type": "object"}},
		{"unknown keyword", map[string]any{"$schema": jsonSchema202012Dialect, "type": "object", "nullable": true}},
		{"unknown type", map[string]any{"$schema": jsonSchema202012Dialect, "type": "int"}},
		{"required missing property", map[string]any{"$schema": jsonSchema202012Dialect, "type": "object", "properties": map[string]any{}, "required": []any{"missing"}}},
		{"duplicate required", map[string]any{"$schema": jsonSchema202012Dialect, "type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []any{"path", "path"}}},
		{"empty oneOf", map[string]any{"$schema": jsonSchema202012Dialect, "oneOf": []any{}}},
		{"invalid minimum", map[string]any{"$schema": jsonSchema202012Dialect, "type": "integer", "minimum": "zero"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateSchema202012(tc.schema, true, "$", map[string]struct{}{}); err == nil {
				t.Fatal("invalid schema was accepted")
			}
		})
	}
}

func filesystemRuntimeTools(filesystem *fakeFileSystem) []Tool {
	return []Tool{
		NewListDirectoryTool(filesystem), NewReadFileTool(filesystem, 1024), NewGetPathInfoTool(filesystem),
		NewWriteFileTool(filesystem), NewCreateDirectoryTool(filesystem), NewDeletePathTool(filesystem),
		NewCopyPathTool(filesystem), NewMovePathTool(filesystem),
	}
}

func validateToolSchemaDocument(t *testing.T, name string, schema map[string]any) {
	t.Helper()
	if err := validateSchema202012(schema, true, "$", map[string]struct{}{}); err != nil {
		t.Fatalf("%s is not valid JSON Schema 2020-12: %v", name, err)
	}
}

func validateSchema202012(schema map[string]any, root bool, path string, seenRequired map[string]struct{}) error {
	allowed := map[string]struct{}{
		"$schema": {}, "type": {}, "properties": {}, "required": {}, "additionalProperties": {},
		"items": {}, "oneOf": {}, "const": {}, "minLength": {}, "minimum": {}, "description": {},
	}
	for keyword := range schema {
		if _, ok := allowed[keyword]; !ok {
			return fmt.Errorf("%s: unsupported keyword %q", path, keyword)
		}
	}
	if root {
		if schema["$schema"] != jsonSchema202012Dialect {
			return fmt.Errorf("%s: $schema must be %q", path, jsonSchema202012Dialect)
		}
	} else if _, ok := schema["$schema"]; ok {
		return fmt.Errorf("%s: nested $schema is not part of the FlashGate dialect contract", path)
	}

	if rawType, ok := schema["type"]; ok {
		typeName, ok := rawType.(string)
		if !ok || !contains([]string{"array", "boolean", "integer", "object", "string"}, typeName) {
			return fmt.Errorf("%s.type: unsupported value %#v", path, rawType)
		}
	}
	if description, ok := schema["description"]; ok {
		if text, ok := description.(string); !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("%s.description: must be a non-empty string", path)
		}
	}
	for _, keyword := range []string{"minLength", "minimum"} {
		if value, ok := schema[keyword]; ok && !nonNegativeInteger(value) {
			return fmt.Errorf("%s.%s: must be a non-negative integer", path, keyword)
		}
	}
	if value, ok := schema["additionalProperties"]; ok {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s.additionalProperties: must be boolean", path)
		}
	}

	properties := map[string]any{}
	if rawProperties, ok := schema["properties"]; ok {
		var valid bool
		properties, valid = rawProperties.(map[string]any)
		if !valid {
			return fmt.Errorf("%s.properties: must be an object", path)
		}
		names := make([]string, 0, len(properties))
		for name := range properties {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			child, ok := properties[name].(map[string]any)
			if !ok {
				return fmt.Errorf("%s.properties.%s: must be a schema object", path, name)
			}
			if err := validateSchema202012(child, false, path+".properties."+name, map[string]struct{}{}); err != nil {
				return err
			}
		}
	}
	if rawRequired, ok := schema["required"]; ok {
		required, ok := rawRequired.([]any)
		if !ok {
			return fmt.Errorf("%s.required: must be an array", path)
		}
		for _, rawName := range required {
			name, ok := rawName.(string)
			if !ok || name == "" {
				return fmt.Errorf("%s.required: entries must be non-empty strings", path)
			}
			if _, duplicate := seenRequired[name]; duplicate {
				return fmt.Errorf("%s.required: duplicate %q", path, name)
			}
			seenRequired[name] = struct{}{}
			if _, exists := properties[name]; !exists {
				return fmt.Errorf("%s.required: property %q is not declared", path, name)
			}
		}
	}
	if rawItems, ok := schema["items"]; ok {
		items, ok := rawItems.(map[string]any)
		if !ok {
			return fmt.Errorf("%s.items: must be a schema object", path)
		}
		if err := validateSchema202012(items, false, path+".items", map[string]struct{}{}); err != nil {
			return err
		}
	}
	if rawVariants, ok := schema["oneOf"]; ok {
		variants, ok := rawVariants.([]any)
		if !ok || len(variants) == 0 {
			return fmt.Errorf("%s.oneOf: must be a non-empty array", path)
		}
		for index, rawVariant := range variants {
			variant, ok := rawVariant.(map[string]any)
			if !ok {
				return fmt.Errorf("%s.oneOf[%d]: must be a schema object", path, index)
			}
			if err := validateSchema202012(variant, false, fmt.Sprintf("%s.oneOf[%d]", path, index), map[string]struct{}{}); err != nil {
				return err
			}
		}
	}
	if expected, ok := schema["const"]; ok {
		if expected == nil || reflect.ValueOf(expected).Kind() == reflect.Invalid {
			return fmt.Errorf("%s.const: null is outside the current tool-schema contract", path)
		}
	}
	return nil
}

func nonNegativeInteger(value any) bool {
	switch number := value.(type) {
	case json.Number:
		integer, err := number.Int64()
		return err == nil && integer >= 0
	case int:
		return number >= 0
	case float64:
		return number >= 0 && number == float64(int64(number))
	default:
		return false
	}
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
