package tools

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRuntimeDefinitionsMatchStaticCatalog(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "mcp-tool-catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Tools []struct {
			Name         string         `json:"name"`
			Title        string         `json:"title"`
			Description  string         `json:"description"`
			InputSchema  map[string]any `json:"inputSchema"`
			ResultSchema map[string]any `json:"resultSchema"`
		} `json:"tools"`
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatalf("invalid tool catalog: %v", err)
	}

	fake := newFakeFileSystem()
	runtimeTools := []Tool{
		NewListDirectoryTool(fake), NewReadFileTool(fake, 1024), NewGetPathInfoTool(fake),
		NewHashFilesTool(fake, 1024),
		NewWriteFileTool(fake), NewCreateDirectoryTool(fake), NewDeletePathTool(fake),
		NewCopyPathTool(fake), NewMovePathTool(fake),
	}
	if len(catalog.Tools) != len(runtimeTools) {
		t.Fatalf("catalog has %d tools, runtime has %d", len(catalog.Tools), len(runtimeTools))
	}

	for index, runtimeTool := range runtimeTools {
		definition := runtimeTool.Definition()
		entry := catalog.Tools[index]
		if definition.Name != entry.Name || definition.Title != entry.Title || definition.Description != entry.Description {
			t.Fatalf("definition mismatch at index %d: runtime=%#v catalog=%#v", index, definition, entry)
		}

		runtimeSchema := normalizeSchema(t, definition.InputSchema)
		if !reflect.DeepEqual(runtimeSchema, entry.InputSchema) {
			t.Fatalf("%s input schema mismatch: runtime=%#v catalog=%#v", definition.Name, runtimeSchema, entry.InputSchema)
		}

		runtimeOutputSchema := normalizeSchema(t, definition.OutputSchema)
		if !reflect.DeepEqual(runtimeOutputSchema, entry.ResultSchema) {
			t.Fatalf("%s output schema mismatch: runtime=%#v catalog=%#v", definition.Name, runtimeOutputSchema, entry.ResultSchema)
		}
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
