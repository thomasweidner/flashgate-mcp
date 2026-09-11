package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

func TestFilesystemRuntimeOutputSchemas(t *testing.T) {
	fake := newFakeFileSystem()
	runtimeTools := []Tool{
		NewListDirectoryTool(fake), NewReadFileTool(fake, 1024), NewGetPathInfoTool(fake),
		NewWriteFileTool(fake), NewCreateDirectoryTool(fake), NewDeletePathTool(fake),
		NewCopyPathTool(fake), NewMovePathTool(fake),
	}

	if len(runtimeTools) != 8 {
		t.Fatalf("expected exactly 8 runtime tools, got %d", len(runtimeTools))
	}
	for _, runtimeTool := range runtimeTools {
		schema := normalizeSchema(t, runtimeTool.Definition().OutputSchema)
		if schema == nil {
			t.Fatalf("%s has no outputSchema", runtimeTool.Name())
		}
		if schema["type"] != "object" {
			t.Fatalf("%s root outputSchema must have type object, got %#v", runtimeTool.Name(), schema["type"])
		}
		assertRequiredPropertiesExist(t, runtimeTool.Name(), schema)
	}
}

func TestFilesystemStructuredResultsMatchOutputSchemas(t *testing.T) {
	tests := []struct {
		name   string
		result any
	}{
		{listDirectoryToolName, listDirectoryResult{Entries: projectListDirectoryEntries([]fs.Entry{{Name: "file.txt", Size: 4}}, listDirectoryFields{name: true, isDir: true, size: true})}},
		{readFileToolName, readFileResult{Content: "text", Size: 4}},
		{getPathInfoToolName + " existing", getPathInfoExistingResult{Path: "file.txt", Exists: true, Name: "file.txt", Size: 4}},
		{getPathInfoToolName + " missing", getPathInfoMissingResult{Path: "missing.txt", Exists: false}},
		{writeFileToolName, writeFileResult{Path: "file.txt", Size: 4, Written: true}},
		{createDirectoryToolName, createDirectoryResult{Path: "dir", Created: true}},
		{deletePathToolName, deletePathResult{Path: "file.txt", Deleted: true}},
		{copyPathToolName, copyPathResult{Source: "source.txt", Target: "target.txt", Copied: true}},
		{movePathToolName, movePathResult{Source: "source.txt", Target: "target.txt", Moved: true}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			toolName := tc.name
			if toolName == getPathInfoToolName+" existing" || toolName == getPathInfoToolName+" missing" {
				toolName = getPathInfoToolName
			}
			value := normalizeResultValue(t, tc.result)
			if err := validateProjectSchema(normalizeSchema(t, filesystemOutputSchema(toolName)), value); err != nil {
				t.Fatalf("structuredContent does not match outputSchema: %v", err)
			}
		})
	}
}

func TestKnownOutputSchemaPropertyTypes(t *testing.T) {
	tests := []struct {
		tool     string
		property string
		expected string
	}{
		{listDirectoryToolName, "entries", "array"},
		{readFileToolName, "content", "string"},
		{readFileToolName, "size", "integer"},
		{writeFileToolName, "written", "boolean"},
		{createDirectoryToolName, "created", "boolean"},
		{deletePathToolName, "deleted", "boolean"},
		{copyPathToolName, "copied", "boolean"},
		{movePathToolName, "moved", "boolean"},
	}
	for _, tc := range tests {
		schema := normalizeSchema(t, filesystemOutputSchema(tc.tool))
		properties := schema["properties"].(map[string]any)
		property := properties[tc.property].(map[string]any)
		if property["type"] != tc.expected {
			t.Fatalf("%s.%s type=%#v, want %s", tc.tool, tc.property, property["type"], tc.expected)
		}
	}

	infoSchema := normalizeSchema(t, filesystemOutputSchema(getPathInfoToolName))
	variants := infoSchema["oneOf"].([]any)
	for index, rawVariant := range variants {
		variant := rawVariant.(map[string]any)
		properties := variant["properties"].(map[string]any)
		if index == 0 && !reflect.DeepEqual(properties["exists"], map[string]any{"const": false}) {
			t.Fatalf("missing variant must require exists:false: %#v", properties["exists"])
		}
		if index == 1 && !reflect.DeepEqual(properties["exists"], map[string]any{"const": true}) {
			t.Fatalf("existing variant must require exists:true: %#v", properties["exists"])
		}
	}
}

func TestReadFileOutputSchemaSeparatesMCPContentFromDomainContent(t *testing.T) {
	result, rpcErr := wrapSuccessfulToolResult(readFileResult{Content: "file text", Size: 9})
	if rpcErr != nil {
		t.Fatalf("unexpected wrapper error: %#v", rpcErr)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["content"].([]any); !ok {
		t.Fatalf("outer CallToolResult.content must be an array, got %#v", decoded["content"])
	}
	structured := decoded["structuredContent"].(map[string]any)
	if _, ok := structured["content"].(string); !ok {
		t.Fatalf("inner read_file content must be a string, got %#v", structured["content"])
	}
	schema := normalizeSchema(t, filesystemOutputSchema(readFileToolName))
	properties := schema["properties"].(map[string]any)
	if properties["content"].(map[string]any)["type"] != "string" {
		t.Fatalf("read_file outputSchema must describe domain content as string: %#v", schema)
	}
}

func assertRequiredPropertiesExist(t *testing.T, path string, schema map[string]any) {
	t.Helper()
	if required, ok := schema["required"].([]any); ok {
		properties, ok := schema["properties"].(map[string]any)
		if !ok {
			t.Fatalf("%s has required fields without properties", path)
		}
		for _, rawName := range required {
			name, ok := rawName.(string)
			if !ok {
				t.Fatalf("%s has non-string required field %#v", path, rawName)
			}
			if _, ok := properties[name]; !ok {
				t.Fatalf("%s requires missing property %q", path, name)
			}
		}
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for name, rawProperty := range properties {
			property, ok := rawProperty.(map[string]any)
			if !ok {
				continue
			}
			assertRequiredPropertiesExist(t, path+"."+name, property)
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		assertRequiredPropertiesExist(t, path+"[]", items)
	}
	if variants, ok := schema["oneOf"].([]any); ok {
		for index, rawVariant := range variants {
			assertRequiredPropertiesExist(t, fmt.Sprintf("%s.oneOf[%d]", path, index), rawVariant.(map[string]any))
		}
	}
}

func normalizeResultValue(t *testing.T, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var result any
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

// validateProjectSchema implements only the schema subset emitted by this package's tests.
func validateProjectSchema(schema map[string]any, value any) error {
	if expected, ok := schema["const"]; ok && !reflect.DeepEqual(value, expected) {
		return fmt.Errorf("value %#v does not equal const %#v", value, expected)
	}
	if variants, ok := schema["oneOf"].([]any); ok {
		matches := 0
		for _, rawVariant := range variants {
			if validateProjectSchema(rawVariant.(map[string]any), value) == nil {
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("value matched %d oneOf variants", matches)
		}
	}

	switch schema["type"] {
	case nil:
		return nil
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
		properties, _ := schema["properties"].(map[string]any)
		if required, ok := schema["required"].([]any); ok {
			for _, rawName := range required {
				name := rawName.(string)
				if _, ok := object[name]; !ok {
					return fmt.Errorf("missing required property %q", name)
				}
			}
		}
		for name, child := range object {
			rawProperty, known := properties[name]
			if !known {
				if schema["additionalProperties"] == false {
					return fmt.Errorf("unexpected property %q", name)
				}
				continue
			}
			if err := validateProjectSchema(rawProperty.(map[string]any), child); err != nil {
				return fmt.Errorf("property %s: %w", name, err)
			}
		}
	case "array":
		array, ok := value.([]any)
		if !ok {
			return fmt.Errorf("expected array, got %T", value)
		}
		items := schema["items"].(map[string]any)
		for index, item := range array {
			if err := validateProjectSchema(items, item); err != nil {
				return fmt.Errorf("item %d: %w", index, err)
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "integer":
		number, ok := value.(json.Number)
		if !ok {
			return fmt.Errorf("expected integer, got %T", value)
		}
		if _, err := number.Int64(); err != nil {
			return fmt.Errorf("expected integer: %w", err)
		}
	default:
		return fmt.Errorf("unsupported test schema type %#v", schema["type"])
	}
	return nil
}
