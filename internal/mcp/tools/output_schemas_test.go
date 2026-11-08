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
		{readFileToolName, payloadMetadata{Size: 4}},
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

func TestReadFileOutputSchemaDescribesOnlyMetadata(t *testing.T) {
	tool := NewReadFileTool(newFakeFileSystem(), 1024)
	result, err := tool.ResultContract().wrap(t.Context(), NewPayloadResult(defaultVariant, requireTextPart(t, "file text")))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	want := []any{map[string]any{"type": "text", "text": "file text"}}
	if !reflect.DeepEqual(decoded["content"], want) {
		t.Fatalf("unexpected text payload: %#v", decoded["content"])
	}
	structured := decoded["structuredContent"].(map[string]any)
	if !reflect.DeepEqual(structured, map[string]any{"size": float64(9)}) {
		t.Fatalf("unexpected structured metadata: %#v", structured)
	}
	schema := normalizeSchema(t, filesystemOutputSchema(readFileToolName))
	properties := schema["properties"].(map[string]any)
	if len(properties) != 1 || properties["size"] == nil {
		t.Fatalf("read_file outputSchema must describe size only: %#v", schema)
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
