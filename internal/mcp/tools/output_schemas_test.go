package tools

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
)

func TestFilesystemStructuredResultsMatchOutputSchemas(t *testing.T) {
	tests := []struct {
		name   string
		result any
	}{
		{listDirectoryToolName, listDirectoryResult{Entries: []fs.Entry{{Name: "file.txt", Size: 4}}}},
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
			schema, err := mcptest.CompileToolSchema(filesystemOutputSchema(toolName), false)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(tc.result)
			if err != nil {
				t.Fatal(err)
			}
			value, err := mcptest.SchemaInstance(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(value); err != nil {
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

func normalizeSchema(t *testing.T, schema any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
