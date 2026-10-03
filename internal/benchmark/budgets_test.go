package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSerializationBudgetsRejectsDuplicateFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), "budgets.json")
	data := []byte(`{"schema_version":"flashgate-benchmark-budgets/v1","hard":{"serialization":{"duplicate":{"max_payload_bytes":1,"max_allocs_per_op":1},"duplicate":{"max_payload_bytes":1,"max_allocs_per_op":1}}}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSerializationBudgets(path); err == nil || !strings.Contains(err.Error(), "duplicate JSON field") {
		t.Fatalf("duplicate serialization fixture was not rejected: %v", err)
	}
}

func TestLoadBudgetFileRejectsNonCanonicalJSON(t *testing.T) {
	canonical, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		want   string
		mutate func([]byte) []byte
	}{
		{
			name: "unknown field", want: "$.unexpected: unknown field",
			mutate: func(data []byte) []byte {
				return mutateJSONObject(t, data, func(object map[string]any) { object["unexpected"] = true })
			},
		},
		{
			name: "missing required field", want: "$.soft: missing required field",
			mutate: func(data []byte) []byte {
				return mutateJSONObject(t, data, func(object map[string]any) { delete(object, "soft") })
			},
		},
		{
			name: "trailing content", want: "trailing JSON data",
			mutate: func(data []byte) []byte { return append(append([]byte{}, data...), []byte("\n{}")...) },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "budgets.json")
			if err := os.WriteFile(path, tc.mutate(canonical), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadBudgetFile(path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("strict budget decoder error=%v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestEvaluateBudgetsRejectsIncompleteAndUnknownMeasurementSets(t *testing.T) {
	budgetPath := filepath.Join("..", "..", "benchmarks", "budgets.json")
	tests := []struct {
		name   string
		mutate func(*Result)
		want   string
	}{
		{"empty tool profiles", func(result *Result) { result.ToolsList = nil }, "tools/list measurement count"},
		{"one tool profile", func(result *Result) { result.ToolsList = result.ToolsList[:1] }, "tools/list measurement count"},
		{"duplicate tool profile", func(result *Result) { result.ToolsList[1] = result.ToolsList[0] }, "duplicated"},
		{"unknown tool profile", func(result *Result) { result.ToolsList[1].Profile = "unknown" }, "unknown"},
		{"empty tool profile name", func(result *Result) { result.ToolsList[0].Profile = "" }, "empty profile"},
		{"incomplete tool measurement", func(result *Result) { result.ToolsList[0].ResponseBytes = 0 }, "structurally incomplete"},
		{"empty workflows", func(result *Result) { result.Workflows = nil }, "workflow measurement count"},
		{"missing workflow", func(result *Result) { result.Workflows = result.Workflows[:len(result.Workflows)-1] }, "workflow measurement count"},
		{"duplicate workflow", func(result *Result) { result.Workflows[1] = result.Workflows[0] }, "duplicated"},
		{"unknown workflow", func(result *Result) { result.Workflows[0].Name = "unknown" }, "unknown"},
		{"empty workflow name", func(result *Result) { result.Workflows[0].Name = "" }, "empty name"},
		{"incomplete workflow", func(result *Result) { result.Workflows[0].RequestBytes.Samples = 0 }, "structurally incomplete"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := completeBudgetTestResult()
			tc.mutate(&result)
			evaluation, err := EvaluateBudgets(budgetPath, result)
			if err != nil {
				t.Fatal(err)
			}
			if evaluation.HardFailures == 0 {
				t.Fatal("invalid measurement set produced no hard failure")
			}
			if !strings.Contains(strings.Join(evaluation.Messages, "\n"), tc.want) {
				t.Fatalf("messages=%v, want substring %q", evaluation.Messages, tc.want)
			}
		})
	}
}

func TestEvaluateBudgetsSeparatesHardAndSoft(t *testing.T) {
	result := completeBudgetTestResult()
	result.ToolsList[0].RequestBytes = 60
	result.Workflows[0].DurationNS = MetricSummary{Samples: 1, Min: 100_000_001, P50: 100_000_001, P95: 100_000_001, Max: 100_000_001}
	evaluation, err := EvaluateBudgets(filepath.Join("..", "..", "benchmarks", "budgets.json"), result)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.HardFailures != 1 || evaluation.SoftWarnings != 1 {
		t.Fatalf("evaluation hard=%d soft=%d, want 1/1: %v", evaluation.HardFailures, evaluation.SoftWarnings, evaluation.Messages)
	}
}

func TestEvaluateBudgetsRejectsMissingUsefulOutput(t *testing.T) {
	budgetPath := filepath.Join("..", "..", "benchmarks", "budgets.json")
	tests := []struct {
		name     string
		workflow string
		mutate   func(*WorkflowMeasurement)
		want     string
	}{
		{"reduced read bytes", "read_file_small", func(measurement *WorkflowMeasurement) {
			measurement.ReadBytes = MetricSummary{Samples: 1, Min: 25, P50: 25, P95: 25, Max: 25}
		}, "read_bytes useful output range=25..25, want exact 26"},
		{"absent entries", "list_directory_small", func(measurement *WorkflowMeasurement) { measurement.Entries = MetricSummary{Samples: 1} }, "entries useful output range=0..0, want exact 3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := completeBudgetTestResult()
			for index := range result.Workflows {
				if result.Workflows[index].Name == tc.workflow {
					tc.mutate(&result.Workflows[index])
				}
			}
			evaluation, err := EvaluateBudgets(budgetPath, result)
			if err != nil {
				t.Fatal(err)
			}
			if evaluation.HardFailures != 1 || !strings.Contains(strings.Join(evaluation.Messages, "\n"), tc.want) {
				t.Fatalf("evaluation=%+v, want one hard failure containing %q", evaluation, tc.want)
			}
		})
	}
}

func TestValidateWorkflowUsefulOutput(t *testing.T) {
	workflow := workflowDefinition{name: "read", expectedReadBytes: exactCounter(26), expectedEntries: exactCounter(3)}
	if err := validateWorkflowUsefulOutput(workflow, Counters{ReadBytes: 26, Entries: 3}); err != nil {
		t.Fatalf("exact useful output rejected: %v", err)
	}
	if err := validateWorkflowUsefulOutput(workflow, Counters{ReadBytes: 25, Entries: 3}); err == nil || !strings.Contains(err.Error(), "read_bytes=25") {
		t.Fatalf("reduced useful output error=%v", err)
	}
}

func completeBudgetTestResult() Result {
	toolsList := []ToolsListMeasurement{
		{Profile: "read_only", ToolCount: 3, SchemaCount: 3, RequestBytes: 59, ResponseBytes: 1, ResultBytes: 1, ApproxTokensBytes4: 1},
		{Profile: "default", ToolCount: 8, SchemaCount: 8, RequestBytes: 59, ResponseBytes: 1, ResultBytes: 1, ApproxTokensBytes4: 1},
	}
	workflows := make([]WorkflowMeasurement, 0, len(referenceWorkflows()))
	for _, definition := range referenceWorkflows() {
		calls := 0
		for _, request := range definition.requests {
			if request.Method == "tools/call" {
				calls++
			}
		}
		one := MetricSummary{Samples: 1, Min: 1, P50: 1, P95: 1, Max: 1}
		zero := MetricSummary{Samples: 1}
		workflows = append(workflows, WorkflowMeasurement{
			Name:               definition.name,
			Repetitions:        1,
			Calls:              calls,
			RequestBytes:       one,
			ResponseBytes:      one,
			ResultBytes:        one,
			DurationNS:         one,
			ReadBytes:          zero,
			WrittenBytes:       zero,
			ScannedBytes:       zero,
			Entries:            zero,
			ApproxTokensBytes4: one,
			Resources:          ResourceSummary{Status: "not_supported"},
			ExitStatuses:       map[string]int{"0": 1},
			StderrWarnings:     []string{},
		})
		measurement := &workflows[len(workflows)-1]
		if definition.expectedReadBytes != nil {
			measurement.ReadBytes = MetricSummary{Samples: 1, Min: *definition.expectedReadBytes, P50: *definition.expectedReadBytes, P95: *definition.expectedReadBytes, Max: *definition.expectedReadBytes}
		}
		if definition.expectedEntries != nil {
			measurement.Entries = MetricSummary{Samples: 1, Min: *definition.expectedEntries, P50: *definition.expectedEntries, P95: *definition.expectedEntries, Max: *definition.expectedEntries}
		}
	}
	return Result{
		ToolsList: toolsList,
		Workflows: workflows,
		StartMeasurements: ProcessStartMeasurements{
			SubsequentProcessStart: StartMeasurement{DurationNS: MetricSummary{P95: 1}},
		},
	}
}
