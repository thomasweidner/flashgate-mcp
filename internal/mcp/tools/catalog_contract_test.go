package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type catalogDocument struct {
	Tools []catalogTool `json:"tools"`
}

type catalogTool struct {
	Name         string                     `json:"name"`
	Title        string                     `json:"title"`
	Description  string                     `json:"description"`
	InputSchema  map[string]any             `json:"inputSchema"`
	ResultSchema map[string]any             `json:"resultSchema"`
	Annotations  map[string]json.RawMessage `json:"annotations"`
}

func TestRuntimeDefinitionsMatchStaticCatalog(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "mcp-tool-catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := decodeToolCatalog(raw)
	if err != nil {
		t.Fatalf("invalid tool catalog: %v", err)
	}

	fake := newFakeFileSystem()
	runtimeTools := []Tool{
		NewListDirectoryTool(fake), NewReadFileTool(fake, 1024), NewGetPathInfoTool(fake),
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

		catalogAnnotations, err := decodeCatalogAnnotations(entry.Name, entry.Annotations)
		if err != nil {
			t.Fatal(err)
		}
		if definition.Annotations != catalogAnnotations {
			t.Fatalf("%s annotation mismatch: runtime=%#v catalog=%#v", definition.Name, definition.Annotations, catalogAnnotations)
		}
	}
}

func TestCatalogAnnotationsRequireExplicitFalseMembers(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "mcp-tool-catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog map[string]any
	if err := json.Unmarshal(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}), &catalog); err != nil {
		t.Fatal(err)
	}
	tools := catalog["tools"].([]any)
	annotations := tools[0].(map[string]any)["annotations"].(map[string]any)
	delete(annotations, "openWorldHint")
	mutated, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeToolCatalog(mutated); err == nil {
		t.Fatal("expected missing explicit false annotation member to fail")
	}
}

func decodeToolCatalog(raw []byte) (catalogDocument, error) {
	var catalog catalogDocument
	if err := json.Unmarshal(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}), &catalog); err != nil {
		return catalogDocument{}, err
	}
	for _, entry := range catalog.Tools {
		if _, err := decodeCatalogAnnotations(entry.Name, entry.Annotations); err != nil {
			return catalogDocument{}, err
		}
	}
	return catalog, nil
}

func decodeCatalogAnnotations(toolName string, raw map[string]json.RawMessage) (protocol.ToolAnnotations, error) {
	expectedMembers := []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"}
	if len(raw) != len(expectedMembers) {
		return protocol.ToolAnnotations{}, fmt.Errorf("%s annotations have %d members, want %d", toolName, len(raw), len(expectedMembers))
	}
	values := make(map[string]bool, len(expectedMembers))
	for _, member := range expectedMembers {
		rawValue, ok := raw[member]
		if !ok {
			return protocol.ToolAnnotations{}, fmt.Errorf("%s annotations omit %s", toolName, member)
		}
		var value bool
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return protocol.ToolAnnotations{}, fmt.Errorf("%s annotations.%s: %w", toolName, member, err)
		}
		values[member] = value
	}
	return protocol.ToolAnnotations{
		ReadOnlyHint:    values["readOnlyHint"],
		DestructiveHint: values["destructiveHint"],
		IdempotentHint:  values["idempotentHint"],
		OpenWorldHint:   values["openWorldHint"],
	}, nil
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
