package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIRunsVersionedBaselineComparison(t *testing.T) {
	workflowPath := filepath.Join("..", "..", ".github", "workflows", "ci.yml")
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read CI workflow: %v", err)
	}

	workflow := string(data)
	for _, required := range []string{
		"benchmark-baseline-comparison:",
		"go test ./internal/benchmark -run '^TestCompleteVersionedPlatformBaselines$' -count=1",
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("CI workflow does not enforce benchmark baseline comparison: missing %q", required)
		}
	}
}
