package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const toolSchemaSnapshotVersion = "flashgate-mcp-tool-schemas/v1"

type toolSchemaSnapshot struct {
	SchemaVersion   string               `json:"schemaVersion"`
	ProtocolVersion string               `json:"protocolVersion"`
	Tools           []toolSchemaContract `json:"tools"`
}

type toolSchemaContract struct {
	Name         string         `json:"name"`
	InputSchema  any            `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema"`
}

func TestRuntimeToolSchemasMatchSnapshot(t *testing.T) {
	actual, err := json.MarshalIndent(currentToolSchemaSnapshot(), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	actual = append(actual, '\n')

	path := filepath.Join("testdata", "tool-schemas.json")
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("runtime tool schemas differ from %s; review the public contract and update the snapshot deliberately\nactual:\n%s", path, actual)
	}
}

func currentToolSchemaSnapshot() toolSchemaSnapshot {
	registry := createToolRegistry(noopFileSystem{}, 1024, toolCapabilities{filesystemWrite: true})
	runtimeTools := registry.List()

	snapshot := toolSchemaSnapshot{
		SchemaVersion:   toolSchemaSnapshotVersion,
		ProtocolVersion: protocol.ProtocolVersion,
		Tools:           make([]toolSchemaContract, 0, len(runtimeTools)),
	}
	for _, runtimeTool := range runtimeTools {
		definition := runtimeTool.Definition()
		snapshot.Tools = append(snapshot.Tools, toolSchemaContract{
			Name:         definition.Name,
			InputSchema:  definition.InputSchema,
			OutputSchema: definition.OutputSchema,
		})
	}
	return snapshot
}
