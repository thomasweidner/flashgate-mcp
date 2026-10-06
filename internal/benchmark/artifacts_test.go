package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestVersionedBenchmarkArtifacts(t *testing.T) {
	benchmarkDirectory := filepath.Join("..", "..", "benchmarks")

	schemaData, err := os.ReadFile(filepath.Join(benchmarkDirectory, "baseline.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		t.Fatalf("invalid baseline schema JSON: %v", err)
	}
	if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("unexpected JSON Schema dialect: %v", schema["$schema"])
	}
	requiredFields := make(map[string]struct{})
	for _, field := range schema["required"].([]any) {
		requiredFields[field.(string)] = struct{}{}
	}
	for _, field := range []string{"benchmark_suite_version", "workflow_catalog_version", "corpus_version", "runtime_mode", "transport", "execution_backend", "profile", "parallelism"} {
		if _, ok := requiredFields[field]; !ok {
			t.Fatalf("baseline schema does not require provenance field %q", field)
		}
	}

	budgetPath := filepath.Join(benchmarkDirectory, "budgets.json")
	budgets, err := loadBudgetFile(budgetPath)
	if err != nil {
		t.Fatal(err)
	}
	if budgets.SchemaVersion != BudgetSchemaVersion {
		t.Fatalf("budget schema=%q, want %q", budgets.SchemaVersion, BudgetSchemaVersion)
	}
	if len(budgets.Hard.Serialization) != 6 {
		t.Fatalf("serialization budget count=%d, want 6", len(budgets.Hard.Serialization))
	}

	artifactPaths, err := filepath.Glob(filepath.Join(benchmarkDirectory, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, artifactPath := range artifactPaths {
		artifactData, err := os.ReadFile(artifactPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, pattern := range forbiddenBenchmarkArtifactPatterns() {
			if pattern.Match(artifactData) {
				t.Fatalf("versioned benchmark artifact %s contains forbidden host-specific or secret pattern %q", artifactPath, pattern.String())
			}
		}
	}

	baselinePaths, err := filepath.Glob(filepath.Join(benchmarkDirectory, "baseline.*-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, baselinePath := range baselinePaths {
		baseline, baselineData, err := loadValidatedBaselineArtifact(baselinePath, budgetPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateVersionedBaseline(baseline); err != nil {
			t.Fatalf("invalid versioned baseline %s: %v", baselinePath, err)
		}
		for _, pattern := range forbiddenBenchmarkArtifactPatterns() {
			if pattern.Match(baselineData) {
				t.Fatalf("versioned baseline contains forbidden host-specific or secret pattern %q", pattern.String())
			}
		}
	}

	dirty := Result{WorkingTreeDirty: true}
	if err := validateVersionedBaseline(dirty); err == nil || !bytes.Contains([]byte(err.Error()), []byte("working_tree_dirty")) {
		t.Fatalf("dirty versioned baseline was not rejected: %v", err)
	}
}

func forbiddenBenchmarkArtifactPatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		regexp.MustCompile(`(^|["[:space:]])[A-Za-z]:[\\/]`),
		regexp.MustCompile(`(?i)\\Users\\|/home/|/Users/|OneDrive|ThomasW`),
		regexp.MustCompile(`flashgate-benchmark-[0-9]`),
		regexp.MustCompile(`(?i)(auth\.json|api[_-]?key|password|secret|token\s*=|bearer[ :]|private[_-]?key|connection[_-]?string)`),
	}
}

func validateVersionedBaseline(baseline Result) error {
	if baseline.WorkingTreeDirty {
		return fmt.Errorf("working_tree_dirty must be false")
	}
	if baseline.SchemaVersion != ResultSchemaVersion || baseline.Project != "flashgate-mcp" {
		return fmt.Errorf("unexpected identity %q %q", baseline.SchemaVersion, baseline.Project)
	}
	if baseline.BenchmarkSuiteVersion != BenchmarkSuiteVersion || baseline.WorkflowCatalogVersion != WorkflowCatalogVersion || baseline.CorpusVersion != CorpusVersion {
		return fmt.Errorf("incomplete benchmark version provenance")
	}
	if baseline.RuntimeMode != "direct_stdio" || baseline.Transport != "stdio" || baseline.ExecutionBackend != "current_process" || baseline.Profile != "read_only" || baseline.Parallelism != 1 {
		return fmt.Errorf("unexpected execution provenance")
	}
	if baseline.Commit == "" || baseline.GoVersion == "" || baseline.OS == "" || baseline.Architecture == "" || baseline.Repetitions < 1 {
		return fmt.Errorf("incomplete baseline provenance")
	}
	if len(baseline.Workflows) != len(referenceWorkflows()) || len(baseline.ToolsList) != len(expectedToolsListProfiles) {
		return fmt.Errorf("incomplete baseline measurement sets")
	}
	if baseline.BudgetEvaluation.SchemaVersion != BudgetSchemaVersion || baseline.BudgetEvaluation.HardFailures != 0 {
		return fmt.Errorf("invalid baseline budget evaluation")
	}
	return nil
}

func TestWorkflowCatalogMatchesRunner(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "workflows.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		SchemaVersion string `json:"schema_version"`
		Workflows     []struct {
			Name              string  `json:"name"`
			ToolsCallCount    int     `json:"tools_call_count"`
			ExpectedReadBytes *uint64 `json:"expected_read_bytes,omitempty"`
			ExpectedEntries   *uint64 `json:"expected_entries,omitempty"`
		} `json:"workflows"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.SchemaVersion != WorkflowCatalogVersion {
		t.Fatalf("workflow schema=%q", catalog.SchemaVersion)
	}
	definitions := referenceWorkflows()
	if len(catalog.Workflows) != len(definitions) {
		t.Fatalf("catalog workflows=%d, runner workflows=%d", len(catalog.Workflows), len(definitions))
	}
	for index, workflow := range definitions {
		if catalog.Workflows[index].Name != workflow.name {
			t.Fatalf("workflow %d name=%q, want %q", index, catalog.Workflows[index].Name, workflow.name)
		}
		calls := 0
		for _, request := range workflow.requests {
			if request.Method == "tools/call" {
				calls++
			}
		}
		if catalog.Workflows[index].ToolsCallCount != calls {
			t.Fatalf("workflow %s calls=%d, want %d", workflow.name, catalog.Workflows[index].ToolsCallCount, calls)
		}
		if !equalOptionalUint64(catalog.Workflows[index].ExpectedReadBytes, workflow.expectedReadBytes) {
			t.Fatalf("workflow %s expected_read_bytes=%v, want %v", workflow.name, catalog.Workflows[index].ExpectedReadBytes, workflow.expectedReadBytes)
		}
		if !equalOptionalUint64(catalog.Workflows[index].ExpectedEntries, workflow.expectedEntries) {
			t.Fatalf("workflow %s expected_entries=%v, want %v", workflow.name, catalog.Workflows[index].ExpectedEntries, workflow.expectedEntries)
		}
	}
}

func equalOptionalUint64(left, right *uint64) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}
