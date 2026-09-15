package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIRunsSchemaSnapshotGate(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}

	workflow := string(raw)
	required := []string{
		"schema-snapshot:",
		"name: MCP schema snapshot",
		"go test ./internal/mcp/tools -run '^TestRuntimeSchemasMatchStaticCatalog$' -count=1",
	}
	for _, fragment := range required {
		if !strings.Contains(workflow, fragment) {
			t.Errorf("CI workflow is missing schema-snapshot contract fragment %q", fragment)
		}
	}
}
