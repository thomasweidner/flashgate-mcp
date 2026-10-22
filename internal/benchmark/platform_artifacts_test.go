package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

const (
	expectedBaselineCommit     = "cfd211fa81cc48ee1dc463966718442f2ab5223c"
	expectedBaselineGoVersion  = "go1.26.4"
	expectedBaselineArch       = "amd64"
	expectedBaselineRepetition = 30
)

type loadedPlatformBaseline struct {
	result Result
	raw    []byte
}

func TestCompleteVersionedPlatformBaselines(t *testing.T) {
	benchmarkDirectory := filepath.Join("..", "..", "benchmarks")
	baselines, err := loadRequiredPlatformBaselines(benchmarkDirectory, filepath.Join(benchmarkDirectory, "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"windows", "linux"} {
		baseline := baselines[platform]
		if err := validateCompletePlatformBaseline(baseline.result, platform); err != nil {
			t.Fatalf("%s baseline: %v", platform, err)
		}
		for _, pattern := range forbiddenBenchmarkArtifactPatterns() {
			if pattern.Match(baseline.raw) {
				t.Fatalf("%s baseline contains forbidden host-specific or secret pattern %q", platform, pattern.String())
			}
		}
	}
	if err := validateDeterministicPlatformMatch(baselines["windows"].result, baselines["linux"].result); err != nil {
		t.Fatal(err)
	}
}

func TestCompletePlatformBaselineValidatorRejectsMutations(t *testing.T) {
	benchmarkDirectory := filepath.Join("..", "..", "benchmarks")
	baselines, err := loadRequiredPlatformBaselines(benchmarkDirectory, filepath.Join(benchmarkDirectory, "budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		want   string
		mutate func(*Result, *Result)
	}{
		{"warning entry", "warnings must be empty", func(windows, _ *Result) { windows.Warnings = []string{"warning"} }},
		{"unsupported metric", "unsupported_metrics must be empty", func(windows, _ *Result) { windows.UnsupportedMetrics = []string{"user_cpu_ns"} }},
		{"unsupported resources", "resource status", func(windows, _ *Result) { windows.Resources.Status = "unsupported" }},
		{"missing resource series", "idle_working_set_bytes is missing", func(windows, _ *Result) { windows.Resources.IdleWorkingSetBytes = nil }},
		{"wrong sample count", "request_bytes samples", func(windows, _ *Result) { windows.Workflows[0].RequestBytes.Samples-- }},
		{"additional exit code", "exit status keys", func(windows, _ *Result) { windows.Workflows[0].ExitStatuses["1"] = 1 }},
		{"stderr warning count", "stderr_warning_count", func(windows, _ *Result) { windows.Workflows[0].WarningCount = 1 }},
		{"workflow order", "workflow order", func(windows, _ *Result) {
			windows.Workflows[0], windows.Workflows[1] = windows.Workflows[1], windows.Workflows[0]
		}},
		{"deterministic byte mismatch", "cross-platform deterministic projection", func(_, linux *Result) {
			linux.ToolsList[0].ResponseBytes++
		}},
		{"missing workflow", "workflow count", func(windows, _ *Result) { windows.Workflows = windows.Workflows[:len(windows.Workflows)-1] }},
		{"additional workflow", "workflow count", func(windows, _ *Result) { windows.Workflows = append(windows.Workflows, windows.Workflows[0]) }},
		{"wrong commit", "commit", func(windows, _ *Result) { windows.Commit = "0000000000000000000000000000000000000000" }},
		{"dirty tree", "working_tree_dirty", func(windows, _ *Result) { windows.WorkingTreeDirty = true }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			windows := cloneBenchmarkResult(t, baselines["windows"].result)
			linux := cloneBenchmarkResult(t, baselines["linux"].result)
			tc.mutate(&windows, &linux)
			err := validateCompletePlatformBaseline(windows, "windows")
			if err == nil {
				err = validateCompletePlatformBaseline(linux, "linux")
			}
			if err == nil {
				err = validateDeterministicPlatformMatch(windows, linux)
			}
			if err == nil || !containsError(err, tc.want) {
				t.Fatalf("validator error=%v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestLoadRequiredPlatformBaselinesRejectsInvalidSets(t *testing.T) {
	budgetPath := filepath.Join("..", "..", "benchmarks", "budgets.json")
	validWindows := Result{OS: "windows", Architecture: expectedBaselineArch}
	validLinux := Result{OS: "linux", Architecture: expectedBaselineArch}
	tests := []struct {
		name      string
		windows   Result
		linux     Result
		omitLinux bool
	}{
		{"missing platform", validWindows, validLinux, true},
		{"unknown platform", validWindows, Result{OS: "darwin", Architecture: expectedBaselineArch}, false},
		{"duplicate platform", validWindows, Result{OS: "windows", Architecture: expectedBaselineArch}, false},
		{"wrong architecture", validWindows, Result{OS: "linux", Architecture: "arm64"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			writeTestBaseline(t, filepath.Join(directory, "baseline.windows-amd64.json"), tc.windows)
			if !tc.omitLinux {
				writeTestBaseline(t, filepath.Join(directory, "baseline.linux-amd64.json"), tc.linux)
			}
			if _, err := loadRequiredPlatformBaselines(directory, budgetPath); err == nil {
				t.Fatal("invalid platform baseline set was accepted")
			}
		})
	}
}

func loadRequiredPlatformBaselines(directory string, budgetPath string) (map[string]loadedPlatformBaseline, error) {
	required := []struct {
		filename     string
		expectedOS   string
		expectedArch string
	}{
		{filename: "baseline.windows-amd64.json", expectedOS: "windows", expectedArch: "amd64"},
		{filename: "baseline.linux-amd64.json", expectedOS: "linux", expectedArch: "amd64"},
	}
	baselines := make(map[string]loadedPlatformBaseline, len(required))
	for _, expected := range required {
		path := filepath.Join(directory, expected.filename)
		result, raw, err := loadValidatedBaselineArtifact(path, budgetPath)
		if err != nil {
			return nil, err
		}
		if result.OS != expected.expectedOS || result.Architecture != expected.expectedArch {
			return nil, fmt.Errorf(
				"artifact %s embedded platform identity=%s/%s, want %s/%s from filename",
				expected.filename, result.OS, result.Architecture, expected.expectedOS, expected.expectedArch,
			)
		}
		if _, exists := baselines[result.OS]; exists {
			return nil, fmt.Errorf("duplicate baseline platform %q", result.OS)
		}
		baselines[result.OS] = loadedPlatformBaseline{result: result, raw: raw}
	}
	if len(baselines) != 2 || baselines["windows"].raw == nil || baselines["linux"].raw == nil {
		return nil, fmt.Errorf("required Windows and Linux baselines are incomplete")
	}
	return baselines, nil
}

func validateCompletePlatformBaseline(result Result, expectedOS string) error {
	if result.SchemaVersion != ResultSchemaVersion || result.BenchmarkSuiteVersion != BenchmarkSuiteVersion || result.WorkflowCatalogVersion != WorkflowCatalogVersion || result.CorpusVersion != CorpusVersion {
		return fmt.Errorf("version provenance is incomplete")
	}
	if result.Project != "flashgate-mcp" || result.Commit != expectedBaselineCommit {
		return fmt.Errorf("project or commit provenance is invalid")
	}
	if result.WorkingTreeDirty {
		return fmt.Errorf("working_tree_dirty must be false")
	}
	if result.GoVersion != expectedBaselineGoVersion || result.OS != expectedOS || result.Architecture != expectedBaselineArch {
		return fmt.Errorf("platform provenance is invalid")
	}
	if result.RuntimeMode != "direct_stdio" || result.Transport != "stdio" || result.ExecutionBackend != "current_process" || result.Profile != "read_only" || result.Parallelism != 1 || result.Repetitions != expectedBaselineRepetition {
		return fmt.Errorf("execution provenance is invalid")
	}
	if result.BudgetEvaluation.SchemaVersion != BudgetSchemaVersion || result.BudgetEvaluation.HardFailures != 0 {
		return fmt.Errorf("budget evaluation must contain zero hard failures")
	}
	if len(result.Warnings) != 0 {
		return fmt.Errorf("warnings must be empty")
	}
	if len(result.UnsupportedMetrics) != 0 {
		return fmt.Errorf("unsupported_metrics must be empty")
	}

	if len(result.ToolsList) != len(expectedToolsListProfiles) {
		return fmt.Errorf("tools/list profile count=%d", len(result.ToolsList))
	}
	for index, profile := range expectedToolsListProfiles {
		if result.ToolsList[index].Profile != profile {
			return fmt.Errorf("tools/list profile order at %d=%q", index, result.ToolsList[index].Profile)
		}
	}

	definitions := referenceWorkflows()
	if len(result.Workflows) != len(definitions) {
		return fmt.Errorf("workflow count=%d, want %d", len(result.Workflows), len(definitions))
	}
	for index, definition := range definitions {
		if result.Workflows[index].Name != definition.name {
			return fmt.Errorf("workflow order at %d=%q, want %q", index, result.Workflows[index].Name, definition.name)
		}
	}

	if err := validateStartMeasurement("first_process_start", result.StartMeasurements.FirstProcessStart, 1); err != nil {
		return err
	}
	if err := validateStartMeasurement("subsequent_process_start", result.StartMeasurements.SubsequentProcessStart, result.Repetitions); err != nil {
		return err
	}
	for index := range result.Workflows {
		if err := validateWorkflowMeasurement(result.Workflows[index], result.Repetitions); err != nil {
			return err
		}
	}

	globalSamples := result.StartMeasurements.FirstProcessStart.Repetitions + result.StartMeasurements.SubsequentProcessStart.Repetitions + len(result.ToolsList)
	for _, workflow := range result.Workflows {
		globalSamples += workflow.Repetitions
	}
	if err := validateResourceSummary("resource_measurements", result.Resources, globalSamples); err != nil {
		return err
	}
	return nil
}

func validateStartMeasurement(name string, measurement StartMeasurement, expectedRepetitions int) error {
	if measurement.Repetitions != expectedRepetitions {
		return fmt.Errorf("%s repetitions=%d, want %d", name, measurement.Repetitions, expectedRepetitions)
	}
	for _, metric := range []struct {
		name     string
		summary  MetricSummary
		constant bool
	}{
		{"duration_ns", measurement.DurationNS, false},
		{"request_bytes", measurement.RequestBytes, true},
		{"response_bytes", measurement.ResponseBytes, true},
		{"result_bytes", measurement.ResultBytes, true},
	} {
		if err := validateMetricSummary(name+"."+metric.name, metric.summary, expectedRepetitions, metric.constant); err != nil {
			return err
		}
	}
	if err := validateResourceSummary(name+".resources", measurement.Resources, expectedRepetitions); err != nil {
		return err
	}
	return validateExitAndStderr(name, measurement.ExitStatuses, measurement.StderrWarnings, measurement.WarningCount, expectedRepetitions)
}

func validateWorkflowMeasurement(measurement WorkflowMeasurement, expectedRepetitions int) error {
	if measurement.Repetitions != expectedRepetitions {
		return fmt.Errorf("workflow %s repetitions=%d, want %d", measurement.Name, measurement.Repetitions, expectedRepetitions)
	}
	for _, metric := range []struct {
		name     string
		summary  MetricSummary
		constant bool
	}{
		{"request_bytes", measurement.RequestBytes, true},
		{"response_bytes", measurement.ResponseBytes, true},
		{"result_bytes", measurement.ResultBytes, true},
		{"duration_ns", measurement.DurationNS, false},
		{"read_bytes", measurement.ReadBytes, true},
		{"written_bytes", measurement.WrittenBytes, true},
		{"scanned_bytes", measurement.ScannedBytes, true},
		{"entries", measurement.Entries, true},
		{"approx_tokens_bytes4", measurement.ApproxTokensBytes4, true},
	} {
		if err := validateMetricSummary("workflow "+measurement.Name+"."+metric.name, metric.summary, expectedRepetitions, metric.constant); err != nil {
			return err
		}
	}
	if err := validateResourceSummary("workflow "+measurement.Name+".resources", measurement.Resources, expectedRepetitions); err != nil {
		return err
	}
	return validateExitAndStderr("workflow "+measurement.Name, measurement.ExitStatuses, measurement.StderrWarnings, measurement.WarningCount, expectedRepetitions)
}

func validateResourceSummary(name string, resources ResourceSummary, expectedSamples int) error {
	if resources.Status != "supported" {
		return fmt.Errorf("%s resource status=%q, want supported", name, resources.Status)
	}
	metrics := []struct {
		name    string
		summary *MetricSummary
	}{
		{"idle_working_set_bytes", resources.IdleWorkingSetBytes},
		{"peak_working_set_bytes", resources.PeakWorkingSetBytes},
		{"user_cpu_ns", resources.UserCPUNS},
		{"system_cpu_ns", resources.SystemCPUNS},
	}
	for _, metric := range metrics {
		if metric.summary == nil {
			return fmt.Errorf("%s.%s is missing", name, metric.name)
		}
		if err := validateMetricSummary(name+"."+metric.name, *metric.summary, expectedSamples, false); err != nil {
			return err
		}
	}
	return nil
}

func validateMetricSummary(name string, summary MetricSummary, expectedSamples int, requireConstant bool) error {
	if summary.Samples != expectedSamples || summary.Samples <= 0 {
		return fmt.Errorf("%s samples=%d, want %d", name, summary.Samples, expectedSamples)
	}
	if summary.Min > summary.P50 || summary.P50 > summary.P95 || summary.P95 > summary.Max {
		return fmt.Errorf("%s quantiles are not ordered", name)
	}
	if requireConstant && (summary.Min != summary.P50 || summary.P50 != summary.P95 || summary.P95 != summary.Max) {
		return fmt.Errorf("%s deterministic values are not constant", name)
	}
	return nil
}

func validateExitAndStderr(name string, exitStatuses map[string]int, warnings []string, warningCount int, repetitions int) error {
	if len(exitStatuses) != 1 || exitStatuses["0"] != repetitions {
		return fmt.Errorf("%s exit status keys/counts are invalid", name)
	}
	if len(warnings) != 0 || warningCount != 0 {
		return fmt.Errorf("%s stderr_warning_count=%d warnings=%d", name, warningCount, len(warnings))
	}
	return nil
}

type exitStatusProjection struct {
	Code  string
	Count int
}

type startProjection struct {
	Name               string
	Repetitions        int
	RequestBytes       MetricSummary
	ResponseBytes      MetricSummary
	ResultBytes        MetricSummary
	ExitStatuses       []exitStatusProjection
	StderrWarnings     []string
	StderrWarningCount int
}

type workflowProjection struct {
	Name               string
	Repetitions        int
	Calls              int
	RequestBytes       MetricSummary
	ResponseBytes      MetricSummary
	ResultBytes        MetricSummary
	ReadBytes          MetricSummary
	WrittenBytes       MetricSummary
	ScannedBytes       MetricSummary
	Entries            MetricSummary
	ApproxTokensBytes4 MetricSummary
	ExitStatuses       []exitStatusProjection
	StderrWarnings     []string
	StderrWarningCount int
}

type deterministicBaselineProjection struct {
	SchemaVersion          string
	BenchmarkSuiteVersion  string
	WorkflowCatalogVersion string
	CorpusVersion          string
	Project                string
	Commit                 string
	WorkingTreeDirty       bool
	GoVersion              string
	RuntimeMode            string
	Transport              string
	ExecutionBackend       string
	Profile                string
	Parallelism            int
	Repetitions            int
	Starts                 []startProjection
	ToolsList              []ToolsListMeasurement
	Workflows              []workflowProjection
	Warnings               []string
	UnsupportedMetrics     []string
}

func validateDeterministicPlatformMatch(windows Result, linux Result) error {
	windowsProjection := projectDeterministicBaseline(windows)
	linuxProjection := projectDeterministicBaseline(linux)
	if !reflect.DeepEqual(windowsProjection, linuxProjection) {
		return fmt.Errorf("artifacts baseline.windows-amd64.json and baseline.linux-amd64.json cross-platform deterministic projection differs")
	}
	return nil
}

func projectDeterministicBaseline(result Result) deterministicBaselineProjection {
	starts := []startProjection{
		projectStart("first_process_start", result.StartMeasurements.FirstProcessStart),
		projectStart("subsequent_process_start", result.StartMeasurements.SubsequentProcessStart),
	}
	workflows := make([]workflowProjection, 0, len(result.Workflows))
	for _, measurement := range result.Workflows {
		workflows = append(workflows, workflowProjection{
			Name: measurement.Name, Repetitions: measurement.Repetitions, Calls: measurement.Calls,
			RequestBytes: measurement.RequestBytes, ResponseBytes: measurement.ResponseBytes, ResultBytes: measurement.ResultBytes,
			ReadBytes: measurement.ReadBytes, WrittenBytes: measurement.WrittenBytes, ScannedBytes: measurement.ScannedBytes,
			Entries: measurement.Entries, ApproxTokensBytes4: measurement.ApproxTokensBytes4,
			ExitStatuses: projectExitStatuses(measurement.ExitStatuses), StderrWarnings: append([]string{}, measurement.StderrWarnings...),
			StderrWarningCount: measurement.WarningCount,
		})
	}
	return deterministicBaselineProjection{
		SchemaVersion: result.SchemaVersion, BenchmarkSuiteVersion: result.BenchmarkSuiteVersion,
		WorkflowCatalogVersion: result.WorkflowCatalogVersion, CorpusVersion: result.CorpusVersion,
		Project: result.Project, Commit: result.Commit, WorkingTreeDirty: result.WorkingTreeDirty, GoVersion: result.GoVersion,
		RuntimeMode: result.RuntimeMode, Transport: result.Transport, ExecutionBackend: result.ExecutionBackend,
		Profile: result.Profile, Parallelism: result.Parallelism, Repetitions: result.Repetitions,
		Starts: starts, ToolsList: append([]ToolsListMeasurement{}, result.ToolsList...), Workflows: workflows,
		Warnings: append([]string{}, result.Warnings...), UnsupportedMetrics: append([]string{}, result.UnsupportedMetrics...),
	}
}

func projectStart(name string, measurement StartMeasurement) startProjection {
	return startProjection{
		Name: name, Repetitions: measurement.Repetitions,
		RequestBytes: measurement.RequestBytes, ResponseBytes: measurement.ResponseBytes, ResultBytes: measurement.ResultBytes,
		ExitStatuses: projectExitStatuses(measurement.ExitStatuses), StderrWarnings: append([]string{}, measurement.StderrWarnings...),
		StderrWarningCount: measurement.WarningCount,
	}
}

func projectExitStatuses(statuses map[string]int) []exitStatusProjection {
	projection := make([]exitStatusProjection, 0, len(statuses))
	for code, count := range statuses {
		projection = append(projection, exitStatusProjection{Code: code, Count: count})
	}
	sort.Slice(projection, func(i, j int) bool { return projection[i].Code < projection[j].Code })
	return projection
}

func cloneBenchmarkResult(t *testing.T, source Result) Result {
	t.Helper()
	data, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var clone Result
	if err := json.Unmarshal(data, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func containsError(err error, substring string) bool {
	return err != nil && len(substring) > 0 && stringContains(err.Error(), substring)
}

func stringContains(value string, substring string) bool {
	for index := 0; index+len(substring) <= len(value); index++ {
		if value[index:index+len(substring)] == substring {
			return true
		}
	}
	return false
}

func writeTestBaseline(t *testing.T, path string, result Result) {
	t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
