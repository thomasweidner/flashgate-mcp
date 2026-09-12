package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStrictArtifactValidatorRejectsSecurityRelevantMutations(t *testing.T) {
	benchmarkDirectory := filepath.Join("..", "..", "benchmarks")
	budgetPath := filepath.Join(benchmarkDirectory, "budgets.json")
	windowsSource := readTestArtifact(t, filepath.Join(benchmarkDirectory, "baseline.windows-amd64.json"))
	linuxSource := readTestArtifact(t, filepath.Join(benchmarkDirectory, "baseline.linux-amd64.json"))

	tests := []struct {
		name         string
		wantArtifact string
		wantCause    string
		mutate       func(*[]byte, *[]byte)
	}{
		{
			name: "hard budget exceeded with embedded zero evaluation", wantArtifact: "baseline.windows-amd64.json", wantCause: "budget failure",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateResultArtifact(t, *windows, func(result *Result) {
					result.ToolsList[0].RequestBytes = 60
				})
			},
		},
		{
			name: "relevant value lowered with stale embedded evaluation", wantArtifact: "baseline.windows-amd64.json", wantCause: "budget_evaluation mismatch",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateResultArtifact(t, *windows, func(result *Result) {
					original := result.StartMeasurements.SubsequentProcessStart.DurationNS
					result.StartMeasurements.SubsequentProcessStart.DurationNS.P95 = 100_000_001
					if result.StartMeasurements.SubsequentProcessStart.DurationNS.Max < 100_000_001 {
						result.StartMeasurements.SubsequentProcessStart.DurationNS.Max = 100_000_001
					}
					if err := applyBudgetEvaluation(budgetPath, result); err != nil {
						t.Fatal(err)
					}
					result.StartMeasurements.SubsequentProcessStart.DurationNS = original
				})
			},
		},
		{
			name: "embedded evaluation differs from recomputation", wantArtifact: "baseline.windows-amd64.json", wantCause: "budget_evaluation mismatch",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateResultArtifact(t, *windows, func(result *Result) {
					result.BudgetEvaluation.HardFailures = 1
					result.BudgetEvaluation.Messages = []string{"hard: fabricated"}
				})
			},
		},
		{
			name: "both platform artifacts contain the same fabricated soft evaluation", wantArtifact: "baseline.windows-amd64.json", wantCause: "budget_evaluation mismatch",
			mutate: func(windows, linux *[]byte) {
				mutation := func(result *Result) {
					result.BudgetEvaluation.SoftWarnings = 1
					result.BudgetEvaluation.Messages = []string{"soft: fabricated"}
				}
				*windows = mutateResultArtifact(t, *windows, mutation)
				*linux = mutateResultArtifact(t, *linux, mutation)
			},
		},
		{
			name: "only Windows artifact changed", wantArtifact: "baseline.windows-amd64.json", wantCause: "cross-platform deterministic projection",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateResultArtifact(t, *windows, func(result *Result) {
					incrementConstantMetric(&result.Workflows[0].ApproxTokensBytes4)
				})
			},
		},
		{
			name: "only Linux artifact changed", wantArtifact: "baseline.linux-amd64.json", wantCause: "cross-platform deterministic projection",
			mutate: func(_, linux *[]byte) {
				*linux = mutateResultArtifact(t, *linux, func(result *Result) {
					incrementConstantMetric(&result.Workflows[0].ApproxTokensBytes4)
				})
			},
		},
		{
			name: "both platform artifacts identically exceed a hard budget", wantArtifact: "baseline.windows-amd64.json", wantCause: "budget failure",
			mutate: func(windows, linux *[]byte) {
				mutation := func(result *Result) { result.ToolsList[0].RequestBytes = 60 }
				*windows = mutateResultArtifact(t, *windows, mutation)
				*linux = mutateResultArtifact(t, *linux, mutation)
			},
		},
		{
			name: "both platform artifacts contain the same matching hard failure", wantArtifact: "baseline.windows-amd64.json", wantCause: "budget failure",
			mutate: func(windows, linux *[]byte) {
				mutation := func(result *Result) {
					result.ToolsList[0].RequestBytes = 60
					if err := applyBudgetEvaluation(budgetPath, result); err != nil {
						t.Fatal(err)
					}
				}
				*windows = mutateResultArtifact(t, *windows, mutation)
				*linux = mutateResultArtifact(t, *linux, mutation)
			},
		},
		{
			name: "unknown top-level field", wantArtifact: "baseline.windows-amd64.json", wantCause: "$.unexpected_top_level: unknown field",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateJSONObject(t, *windows, func(object map[string]any) {
					object["unexpected_top_level"] = true
				})
			},
		},
		{
			name: "unknown nested measurement field", wantArtifact: "baseline.windows-amd64.json", wantCause: "unknown_nested: unknown field",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateJSONObject(t, *windows, func(object map[string]any) {
					workflows := object["workflow_measurements"].([]any)
					workflow := workflows[0].(map[string]any)
					requestBytes := workflow["request_bytes"].(map[string]any)
					requestBytes["unknown_nested"] = 1
				})
			},
		},
		{
			name: "missing required field", wantArtifact: "baseline.windows-amd64.json", wantCause: "$.working_tree_dirty: missing required field",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateJSONObject(t, *windows, func(object map[string]any) {
					delete(object, "working_tree_dirty")
				})
			},
		},
		{
			name: "wrong JSON type", wantArtifact: "baseline.windows-amd64.json", wantCause: "$.repetitions: expected number",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateJSONObject(t, *windows, func(object map[string]any) {
					object["repetitions"] = "30"
				})
			},
		},
		{
			name: "null required value", wantArtifact: "baseline.windows-amd64.json", wantCause: "$.warnings: null is not allowed",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateJSONObject(t, *windows, func(object map[string]any) {
					object["warnings"] = nil
				})
			},
		},
		{
			name: "noncanonical workflow exit status", wantArtifact: "baseline.windows-amd64.json", wantCause: "canonical signed decimal exit status",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateResultArtifact(t, *windows, func(result *Result) {
					result.Workflows[0].ExitStatuses = map[string]int{"+0": result.Workflows[0].Repetitions}
				})
			},
		},
		{
			name: "workflow exit status count exceeds portable range", wantArtifact: "baseline.windows-amd64.json", wantCause: "outside the schema range",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateResultArtifact(t, *windows, func(result *Result) {
					result.Workflows[0].ExitStatuses = map[string]int{"0": int(benchmarkJSONMaxInt) + 1}
				})
			},
		},
		{
			name: "trailing JSON object", wantArtifact: "baseline.windows-amd64.json", wantCause: "trailing JSON data",
			mutate: func(windows, _ *[]byte) {
				*windows = append(append([]byte{}, *windows...), []byte("\n{}")...)
			},
		},
		{
			name: "duplicate security-relevant property", wantArtifact: "baseline.windows-amd64.json", wantCause: "$.project: duplicate JSON field",
			mutate: func(windows, _ *[]byte) {
				*windows = append([]byte(`{"project":"flashgate-mcp",`), (*windows)[1:]...)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			windows := append([]byte{}, windowsSource...)
			linux := append([]byte{}, linuxSource...)
			tc.mutate(&windows, &linux)
			directory := t.TempDir()
			writeTestArtifact(t, filepath.Join(directory, "baseline.windows-amd64.json"), windows)
			writeTestArtifact(t, filepath.Join(directory, "baseline.linux-amd64.json"), linux)

			err := validatePlatformArtifactSet(directory, budgetPath)
			if err == nil || !strings.Contains(err.Error(), tc.wantArtifact) || !strings.Contains(err.Error(), tc.wantCause) {
				t.Fatalf("validator error=%v, want artifact %q and cause %q", err, tc.wantArtifact, tc.wantCause)
			}
		})
	}
}

func TestPlatformArtifactSetAcceptsRunnerProducedSoftWarnings(t *testing.T) {
	benchmarkDirectory := filepath.Join("..", "..", "benchmarks")
	budgetPath := filepath.Join(benchmarkDirectory, "budgets.json")
	tests := []struct {
		name          string
		softPlatforms map[string]bool
	}{
		{name: "both platforms", softPlatforms: map[string]bool{"windows": true, "linux": true}},
		{name: "Windows only", softPlatforms: map[string]bool{"windows": true}},
		{name: "Linux only", softPlatforms: map[string]bool{"linux": true}},
		{name: "no budget warning", softPlatforms: map[string]bool{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			for _, platform := range []string{"windows", "linux"} {
				artifactPath := filepath.Join(benchmarkDirectory, "baseline."+platform+"-amd64.json")
				result, _, err := loadValidatedBaselineArtifact(artifactPath, budgetPath)
				if err != nil {
					t.Fatal(err)
				}
				result.Warnings = collectWarnings(nil)
				if tc.softPlatforms[platform] {
					result.StartMeasurements.SubsequentProcessStart.DurationNS.P95 = 100_000_001
					if result.StartMeasurements.SubsequentProcessStart.DurationNS.Max < 100_000_001 {
						result.StartMeasurements.SubsequentProcessStart.DurationNS.Max = 100_000_001
					}
				}
				if err := applyBudgetEvaluation(budgetPath, &result); err != nil {
					t.Fatal(err)
				}

				wantSoft := 0
				if tc.softPlatforms[platform] {
					wantSoft = 1
				}
				if result.BudgetEvaluation.HardFailures != 0 || result.BudgetEvaluation.SoftWarnings != wantSoft {
					t.Fatalf("%s budget evaluation=%+v, want zero hard and %d soft", platform, result.BudgetEvaluation, wantSoft)
				}
				if wantSoft == 1 && (len(result.BudgetEvaluation.Messages) != 1 || !strings.HasPrefix(result.BudgetEvaluation.Messages[0], "soft: ")) {
					t.Fatalf("%s budget messages=%q, want one soft message", platform, result.BudgetEvaluation.Messages)
				}
				for _, warning := range result.Warnings {
					if strings.HasPrefix(warning, "soft: ") {
						t.Fatalf("%s general warnings contain budget message %q", platform, warning)
					}
				}
				writeTestBaseline(t, filepath.Join(directory, "baseline."+platform+"-amd64.json"), result)
			}

			if err := validatePlatformArtifactSet(directory, budgetPath); err != nil {
				t.Fatalf("runner-produced matching soft warnings must pass the complete platform gate: %v", err)
			}
		})
	}
}

func TestPlatformArtifactSetRejectsGeneralRunnerWarning(t *testing.T) {
	benchmarkDirectory := filepath.Join("..", "..", "benchmarks")
	budgetPath := filepath.Join(benchmarkDirectory, "budgets.json")
	directory := t.TempDir()

	for _, platform := range []string{"windows", "linux"} {
		artifactPath := filepath.Join(benchmarkDirectory, "baseline."+platform+"-amd64.json")
		result, _, err := loadValidatedBaselineArtifact(artifactPath, budgetPath)
		if err != nil {
			t.Fatal(err)
		}
		var samples []processSample
		if platform == "windows" {
			samples = []processSample{{warnings: []string{"runtime warning"}}}
		}
		result.Warnings = collectWarnings(samples)
		if err := applyBudgetEvaluation(budgetPath, &result); err != nil {
			t.Fatal(err)
		}
		writeTestBaseline(t, filepath.Join(directory, "baseline."+platform+"-amd64.json"), result)
	}

	err := validatePlatformArtifactSet(directory, budgetPath)
	if err == nil || !strings.Contains(err.Error(), "warnings must be empty") {
		t.Fatalf("general runner warning error=%v, want fatal completeness rejection", err)
	}
}

func TestPlatformArtifactSetRejectsInvalidSoftBudgetDefinitions(t *testing.T) {
	benchmarkDirectory := filepath.Join("..", "..", "benchmarks")
	budgetSource := readTestArtifact(t, filepath.Join(benchmarkDirectory, "budgets.json"))
	workflowSource := readTestArtifact(t, filepath.Join(benchmarkDirectory, "workflows.json"))
	windowsSource := readTestArtifact(t, filepath.Join(benchmarkDirectory, "baseline.windows-amd64.json"))
	linuxSource := readTestArtifact(t, filepath.Join(benchmarkDirectory, "baseline.linux-amd64.json"))

	tests := []struct {
		name   string
		want   string
		mutate func(map[string]any)
	}{
		{
			name: "missing soft workflow key", want: "soft workflow p95 budgets missing key(s): initialize",
			mutate: func(object map[string]any) {
				_, workflows := softBudgetObjects(t, object)
				delete(workflows, "initialize")
			},
		},
		{
			name: "unknown soft workflow key", want: "soft workflow p95 budgets unknown key(s): unexpected_workflow",
			mutate: func(object map[string]any) {
				_, workflows := softBudgetObjects(t, object)
				workflows["unexpected_workflow"] = 1
			},
		},
		{
			name: "zero soft workflow limit", want: "soft workflow p95 budget initialize must be greater than zero",
			mutate: func(object map[string]any) {
				_, workflows := softBudgetObjects(t, object)
				workflows["initialize"] = 0
			},
		},
		{
			name: "multiple missing soft workflow keys are sorted", want: "soft workflow p95 budgets missing key(s): initialize, read_file_small",
			mutate: func(object map[string]any) {
				_, workflows := softBudgetObjects(t, object)
				delete(workflows, "read_file_small")
				delete(workflows, "initialize")
			},
		},
		{
			name: "multiple unknown soft workflow keys are sorted", want: "soft workflow p95 budgets unknown key(s): alpha_unknown, zeta_unknown",
			mutate: func(object map[string]any) {
				_, workflows := softBudgetObjects(t, object)
				workflows["zeta_unknown"] = 1
				workflows["alpha_unknown"] = 1
			},
		},
		{
			name: "missing scalar soft limit", want: "max_system_cpu_ns: missing required field",
			mutate: func(object map[string]any) {
				soft, _ := softBudgetObjects(t, object)
				delete(soft, "max_system_cpu_ns")
			},
		},
	}
	for _, field := range []string{
		"subsequent_process_start_p95_ns",
		"max_idle_working_set_bytes",
		"max_peak_working_set_bytes",
		"max_user_cpu_ns",
		"max_system_cpu_ns",
	} {
		field := field
		tests = append(tests, struct {
			name   string
			want   string
			mutate func(map[string]any)
		}{
			name: "zero scalar soft limit " + field,
			want: "soft budget " + field + " must be greater than zero",
			mutate: func(object map[string]any) {
				soft, _ := softBudgetObjects(t, object)
				soft[field] = 0
			},
		})
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			budget := mutateJSONObject(t, budgetSource, tc.mutate)
			writeTestArtifact(t, filepath.Join(directory, "budgets.json"), budget)
			writeTestArtifact(t, filepath.Join(directory, "workflows.json"), workflowSource)
			writeTestArtifact(t, filepath.Join(directory, "baseline.windows-amd64.json"), windowsSource)
			writeTestArtifact(t, filepath.Join(directory, "baseline.linux-amd64.json"), linuxSource)

			err := validatePlatformArtifactSet(directory, filepath.Join(directory, "budgets.json"))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid soft budget definition error=%v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestPlatformArtifactSetRejectsManipulatedSoftBudgetWithDualPlatformRegression(t *testing.T) {
	benchmarkDirectory := filepath.Join("..", "..", "benchmarks")
	directory := t.TempDir()
	budget := mutateJSONObject(t, readTestArtifact(t, filepath.Join(benchmarkDirectory, "budgets.json")), func(object map[string]any) {
		_, workflows := softBudgetObjects(t, object)
		delete(workflows, "initialize")
	})
	writeTestArtifact(t, filepath.Join(directory, "budgets.json"), budget)
	writeTestArtifact(t, filepath.Join(directory, "workflows.json"), readTestArtifact(t, filepath.Join(benchmarkDirectory, "workflows.json")))

	for _, platform := range []string{"windows", "linux"} {
		artifact := mutateJSONObject(t, readTestArtifact(t, filepath.Join(benchmarkDirectory, "baseline."+platform+"-amd64.json")), func(object map[string]any) {
			workflows := object["workflow_measurements"].([]any)
			measurement := workflows[0].(map[string]any)
			duration := measurement["duration_ns"].(map[string]any)
			duration["p95"] = 100_000_001
			duration["max"] = 100_000_001
		})
		writeTestArtifact(t, filepath.Join(directory, "baseline."+platform+"-amd64.json"), artifact)
	}

	err := validatePlatformArtifactSet(directory, filepath.Join(directory, "budgets.json"))
	if err == nil || !strings.Contains(err.Error(), "soft workflow p95 budgets missing key(s): initialize") {
		t.Fatalf("manipulated budget with dual-platform regression error=%v", err)
	}
}

func TestArtifactBudgetValidationPreservesSoftAndHardSemantics(t *testing.T) {
	benchmarkDirectory := filepath.Join("..", "..", "benchmarks")
	budgetPath := filepath.Join(benchmarkDirectory, "budgets.json")
	result, _, err := loadValidatedBaselineArtifact(filepath.Join(benchmarkDirectory, "baseline.windows-amd64.json"), budgetPath)
	if err != nil {
		t.Fatal(err)
	}

	soft := cloneBenchmarkResult(t, result)
	soft.StartMeasurements.SubsequentProcessStart.DurationNS.P95 = 100_000_001
	if soft.StartMeasurements.SubsequentProcessStart.DurationNS.Max < 100_000_001 {
		soft.StartMeasurements.SubsequentProcessStart.DurationNS.Max = 100_000_001
	}
	if err := applyBudgetEvaluation(budgetPath, &soft); err != nil {
		t.Fatal(err)
	}
	if soft.BudgetEvaluation.HardFailures != 0 || soft.BudgetEvaluation.SoftWarnings != 1 {
		t.Fatalf("soft fixture evaluation=%+v, want zero hard and one soft", soft.BudgetEvaluation)
	}
	if err := validateArtifactBudgets("soft-warning.json", budgetPath, soft); err != nil {
		t.Fatalf("matching soft warning must remain non-fatal: %v", err)
	}

	hard := cloneBenchmarkResult(t, result)
	hard.ToolsList[0].RequestBytes = 60
	if err := applyBudgetEvaluation(budgetPath, &hard); err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactBudgets("hard-failure.json", budgetPath, hard); err == nil || !strings.Contains(err.Error(), "hard-failure.json budget failure") {
		t.Fatalf("matching embedded hard failure was not rejected: %v", err)
	}
}

func validatePlatformArtifactSet(directory string, budgetPath string) error {
	baselines, err := loadRequiredPlatformBaselines(directory, budgetPath)
	if err != nil {
		return err
	}
	for _, platform := range []string{"windows", "linux"} {
		if err := validateCompletePlatformBaseline(baselines[platform].result, platform); err != nil {
			return fmt.Errorf("artifact baseline.%s-amd64.json completeness: %w", platform, err)
		}
	}
	return validateDeterministicPlatformMatch(baselines["windows"].result, baselines["linux"].result)
}

func incrementConstantMetric(summary *MetricSummary) {
	summary.Min++
	summary.P50++
	summary.P95++
	summary.Max++
}

func mutateResultArtifact(t *testing.T, raw []byte, mutate func(*Result)) []byte {
	t.Helper()
	var result Result
	if err := decodeStrictJSON(raw, &result); err != nil {
		t.Fatal(err)
	}
	mutate(&result)
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func mutateJSONObject(t *testing.T, raw []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		t.Fatal(err)
	}
	mutate(object)
	encoded, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func softBudgetObjects(t *testing.T, object map[string]any) (map[string]any, map[string]any) {
	t.Helper()
	soft, ok := object["soft"].(map[string]any)
	if !ok {
		t.Fatal("soft budget object is missing")
	}
	workflows, ok := soft["workflow_p95_ns"].(map[string]any)
	if !ok {
		t.Fatal("soft workflow budget object is missing")
	}
	return soft, workflows
}

func readTestArtifact(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeTestArtifact(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
