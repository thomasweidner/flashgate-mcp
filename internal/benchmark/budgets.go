package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var expectedToolsListProfiles = []string{"read_only", "default"}

type budgetFile struct {
	SchemaVersion string      `json:"schema_version"`
	Hard          hardBudgets `json:"hard"`
	Soft          softBudgets `json:"soft"`
}

type hardBudgets struct {
	ToolsList     map[string]toolsListBudget `json:"tools_list"`
	Workflows     map[string]workflowBudget  `json:"workflows"`
	Serialization serializationBudgetSet     `json:"serialization"`
}

type toolsListBudget struct {
	ToolCount        int    `json:"tool_count"`
	SchemaCount      int    `json:"schema_count"`
	MaxRequestBytes  uint64 `json:"max_request_bytes"`
	MaxResponseBytes uint64 `json:"max_response_bytes"`
	MaxResultBytes   uint64 `json:"max_result_bytes"`
}

type workflowBudget struct {
	Calls            int    `json:"calls"`
	MaxRequestBytes  uint64 `json:"max_request_bytes"`
	MaxResponseBytes uint64 `json:"max_response_bytes"`
	MaxResultBytes   uint64 `json:"max_result_bytes"`
	MaxReadBytes     uint64 `json:"max_read_bytes"`
	MaxWrittenBytes  uint64 `json:"max_written_bytes"`
	MaxScannedBytes  uint64 `json:"max_scanned_bytes"`
	MaxEntries       uint64 `json:"max_entries"`
}

// SerializationBudget is the deterministic payload and allocation gate for one fixture.
type SerializationBudget struct {
	MaxPayloadBytes                      uint64 `json:"max_payload_bytes"`
	MaxAllocsPerOp                       uint64 `json:"max_allocs_per_op"`
	UsefulPayloadBytes                   uint64 `json:"useful_payload_bytes"`
	MaxWireAmplificationMilli            uint64 `json:"max_wire_amplification_milli"`
	MaxApproxTokenCostMilliPerUsefulByte uint64 `json:"max_approx_token_cost_milli_per_useful_byte"`
	MaxSerializationCopies               uint64 `json:"max_serialization_copies"`
}

type serializationBudgetSet map[string]SerializationBudget

func (budgets *serializationBudgetSet) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := opening.(json.Delim); !ok || delimiter != '{' {
		return fmt.Errorf("serialization budgets must be an object")
	}
	decoded := make(serializationBudgetSet)
	for decoder.More() {
		nameToken, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := nameToken.(string)
		if !ok || name == "" {
			return fmt.Errorf("serialization budget fixture name must not be empty")
		}
		if _, exists := decoded[name]; exists {
			return fmt.Errorf("duplicate serialization budget fixture %q", name)
		}
		var budget SerializationBudget
		if err := decoder.Decode(&budget); err != nil {
			return err
		}
		decoded[name] = budget
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	*budgets = decoded
	return nil
}

type softBudgets struct {
	SubsequentProcessStartP95NS uint64            `json:"subsequent_process_start_p95_ns"`
	MaxIdleWorkingSetBytes      uint64            `json:"max_idle_working_set_bytes"`
	MaxPeakWorkingSetBytes      uint64            `json:"max_peak_working_set_bytes"`
	MaxUserCPUNS                uint64            `json:"max_user_cpu_ns"`
	MaxSystemCPUNS              uint64            `json:"max_system_cpu_ns"`
	WorkflowP95NS               map[string]uint64 `json:"workflow_p95_ns"`
}

// EvaluateBudgets compares deterministic hard gates and noise-sensitive soft review budgets.
func EvaluateBudgets(path string, result Result) (BudgetEvaluation, error) {
	budgets, err := loadBudgetFile(path)
	if err != nil {
		return BudgetEvaluation{}, err
	}
	expectedWorkflows, err := loadExpectedWorkflowNames(filepath.Join(filepath.Dir(path), "workflows.json"))
	if err != nil {
		return BudgetEvaluation{}, err
	}
	if err := validateBudgetDefinitions(budgets, expectedWorkflows); err != nil {
		return BudgetEvaluation{}, err
	}

	evaluation := BudgetEvaluation{SchemaVersion: BudgetSchemaVersion, Messages: []string{}}
	hardFailure := func(format string, args ...any) {
		evaluation.HardFailures++
		evaluation.Messages = append(evaluation.Messages, "hard: "+fmt.Sprintf(format, args...))
	}
	softWarning := func(format string, args ...any) {
		evaluation.SoftWarnings++
		evaluation.Messages = append(evaluation.Messages, "soft: "+fmt.Sprintf(format, args...))
	}

	validateToolsListMeasurements(result.ToolsList, budgets.Hard.ToolsList, hardFailure)
	validateWorkflowMeasurements(result.Workflows, expectedWorkflows, budgets.Hard.Workflows, hardFailure)

	for _, measurement := range result.ToolsList {
		budget, ok := budgets.Hard.ToolsList[measurement.Profile]
		if !ok {
			hardFailure("tools/list profile %s has no budget", measurement.Profile)
			continue
		}
		if measurement.ToolCount != budget.ToolCount {
			hardFailure("tools/list %s tool_count=%d exceeds contract value %d", measurement.Profile, measurement.ToolCount, budget.ToolCount)
		}
		if measurement.SchemaCount != budget.SchemaCount {
			hardFailure("tools/list %s schema_count=%d exceeds contract value %d", measurement.Profile, measurement.SchemaCount, budget.SchemaCount)
		}
		checkHardMaximum(&evaluation, "tools/list "+measurement.Profile+" request_bytes", measurement.RequestBytes, budget.MaxRequestBytes)
		checkHardMaximum(&evaluation, "tools/list "+measurement.Profile+" response_bytes", measurement.ResponseBytes, budget.MaxResponseBytes)
		checkHardMaximum(&evaluation, "tools/list "+measurement.Profile+" result_bytes", measurement.ResultBytes, budget.MaxResultBytes)
	}

	for _, measurement := range result.Workflows {
		budget, ok := budgets.Hard.Workflows[measurement.Name]
		if !ok {
			hardFailure("workflow %s has no budget", measurement.Name)
			continue
		}
		if measurement.Calls != budget.Calls {
			hardFailure("workflow %s calls=%d, want %d", measurement.Name, measurement.Calls, budget.Calls)
		}
		checkHardMaximum(&evaluation, "workflow "+measurement.Name+" request_bytes", measurement.RequestBytes.Max, budget.MaxRequestBytes)
		checkHardMaximum(&evaluation, "workflow "+measurement.Name+" response_bytes", measurement.ResponseBytes.Max, budget.MaxResponseBytes)
		checkHardMaximum(&evaluation, "workflow "+measurement.Name+" result_bytes", measurement.ResultBytes.Max, budget.MaxResultBytes)
		checkHardMaximum(&evaluation, "workflow "+measurement.Name+" read_bytes", measurement.ReadBytes.Max, budget.MaxReadBytes)
		checkHardMaximum(&evaluation, "workflow "+measurement.Name+" written_bytes", measurement.WrittenBytes.Max, budget.MaxWrittenBytes)
		checkHardMaximum(&evaluation, "workflow "+measurement.Name+" scanned_bytes", measurement.ScannedBytes.Max, budget.MaxScannedBytes)
		checkHardMaximum(&evaluation, "workflow "+measurement.Name+" entries", measurement.Entries.Max, budget.MaxEntries)
		if limit := budgets.Soft.WorkflowP95NS[measurement.Name]; measurement.DurationNS.P95 > limit {
			softWarning("workflow %s duration p95=%d exceeds %d", measurement.Name, measurement.DurationNS.P95, limit)
		}
	}

	if limit := budgets.Soft.SubsequentProcessStartP95NS; result.StartMeasurements.SubsequentProcessStart.DurationNS.P95 > limit {
		softWarning("subsequent_process_start p95=%d exceeds %d", result.StartMeasurements.SubsequentProcessStart.DurationNS.P95, limit)
	}
	if metric := result.Resources.IdleWorkingSetBytes; metric != nil && metric.Max > budgets.Soft.MaxIdleWorkingSetBytes {
		softWarning("idle working set max=%d exceeds %d", metric.Max, budgets.Soft.MaxIdleWorkingSetBytes)
	}
	if metric := result.Resources.PeakWorkingSetBytes; metric != nil && metric.Max > budgets.Soft.MaxPeakWorkingSetBytes {
		softWarning("peak working set max=%d exceeds %d", metric.Max, budgets.Soft.MaxPeakWorkingSetBytes)
	}
	if metric := result.Resources.UserCPUNS; metric != nil && metric.Max > budgets.Soft.MaxUserCPUNS {
		softWarning("user CPU max=%d exceeds %d", metric.Max, budgets.Soft.MaxUserCPUNS)
	}
	if metric := result.Resources.SystemCPUNS; metric != nil && metric.Max > budgets.Soft.MaxSystemCPUNS {
		softWarning("system CPU max=%d exceeds %d", metric.Max, budgets.Soft.MaxSystemCPUNS)
	}

	return evaluation, nil
}

// LoadSerializationBudgets loads the allocation and payload gates used by the serialization tests.
func LoadSerializationBudgets(path string) (map[string]SerializationBudget, error) {
	budgets, err := loadBudgetFile(path)
	if err != nil {
		return nil, err
	}
	if len(budgets.Hard.Serialization) == 0 {
		return nil, fmt.Errorf("benchmark serialization budgets are empty")
	}
	return map[string]SerializationBudget(budgets.Hard.Serialization), nil
}

func loadBudgetFile(path string) (budgetFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return budgetFile{}, benchmarkReadError("benchmark budgets", path, err)
	}
	var budgets budgetFile
	if err := decodeStrictJSON(data, &budgets); err != nil {
		return budgetFile{}, fmt.Errorf("decode benchmark budgets: %w", err)
	}
	if budgets.SchemaVersion != BudgetSchemaVersion {
		return budgetFile{}, fmt.Errorf("unsupported budget schema version %q", budgets.SchemaVersion)
	}
	return budgets, nil
}

func loadExpectedWorkflowNames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, benchmarkReadError("benchmark workflow catalog", path, err)
	}
	var catalog struct {
		SchemaVersion        string `json:"schema_version"`
		Profile              string `json:"profile"`
		ProcessPerRepetition bool   `json:"process_per_repetition"`
		Workflows            []struct {
			Name              string   `json:"name"`
			Steps             []string `json:"steps"`
			ToolsCallCount    int      `json:"tools_call_count"`
			ExpectedReadBytes *uint64  `json:"expected_read_bytes,omitempty"`
			ExpectedEntries   *uint64  `json:"expected_entries,omitempty"`
		} `json:"workflows"`
	}
	if err := decodeStrictJSON(data, &catalog); err != nil {
		return nil, fmt.Errorf("decode benchmark workflow catalog: %w", err)
	}
	if catalog.SchemaVersion != WorkflowCatalogVersion {
		return nil, fmt.Errorf("unsupported workflow catalog schema version %q", catalog.SchemaVersion)
	}
	seen := make(map[string]struct{}, len(catalog.Workflows))
	names := make([]string, 0, len(catalog.Workflows))
	for _, workflow := range catalog.Workflows {
		if workflow.Name == "" {
			return nil, fmt.Errorf("benchmark workflow catalog contains an empty name")
		}
		if _, ok := seen[workflow.Name]; ok {
			return nil, fmt.Errorf("benchmark workflow catalog contains duplicate %q", workflow.Name)
		}
		seen[workflow.Name] = struct{}{}
		names = append(names, workflow.Name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("benchmark workflow catalog is empty")
	}
	return names, nil
}

func validateBudgetDefinitions(budgets budgetFile, expectedWorkflows []string) error {
	if err := validateExactBudgetKeys("hard tools/list budgets", expectedToolsListProfiles, sortedBudgetKeys(budgets.Hard.ToolsList)); err != nil {
		return err
	}
	if err := validateExactBudgetKeys("hard workflow budgets", expectedWorkflows, sortedBudgetKeys(budgets.Hard.Workflows)); err != nil {
		return err
	}
	if err := validateExactBudgetKeys("soft workflow p95 budgets", expectedWorkflows, sortedBudgetKeys(budgets.Soft.WorkflowP95NS)); err != nil {
		return err
	}

	scalarLimits := []struct {
		name  string
		value uint64
	}{
		{"subsequent_process_start_p95_ns", budgets.Soft.SubsequentProcessStartP95NS},
		{"max_idle_working_set_bytes", budgets.Soft.MaxIdleWorkingSetBytes},
		{"max_peak_working_set_bytes", budgets.Soft.MaxPeakWorkingSetBytes},
		{"max_user_cpu_ns", budgets.Soft.MaxUserCPUNS},
		{"max_system_cpu_ns", budgets.Soft.MaxSystemCPUNS},
	}
	for _, limit := range scalarLimits {
		if limit.value == 0 {
			return fmt.Errorf("soft budget %s must be greater than zero", limit.name)
		}
	}
	for _, workflow := range sortedStrings(expectedWorkflows) {
		if budgets.Soft.WorkflowP95NS[workflow] == 0 {
			return fmt.Errorf("soft workflow p95 budget %s must be greater than zero", workflow)
		}
	}
	return nil
}

func validateExactBudgetKeys(area string, expectedNames []string, actualNames []string) error {
	expected := make(map[string]struct{}, len(expectedNames))
	for _, name := range expectedNames {
		expected[name] = struct{}{}
	}
	actual := make(map[string]struct{}, len(actualNames))
	for _, name := range actualNames {
		actual[name] = struct{}{}
	}

	missing := make([]string, 0)
	for name := range expected {
		if _, ok := actual[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		return fmt.Errorf("%s missing key(s): %s", area, strings.Join(missing, ", "))
	}

	unknown := make([]string, 0)
	for name := range actual {
		if _, ok := expected[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		return fmt.Errorf("%s unknown key(s): %s", area, strings.Join(unknown, ", "))
	}
	return nil
}

func sortedBudgetKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func validateToolsListMeasurements(measurements []ToolsListMeasurement, budgets map[string]toolsListBudget, hardFailure func(string, ...any)) {
	expected := make(map[string]struct{}, len(expectedToolsListProfiles))
	for _, profile := range expectedToolsListProfiles {
		expected[profile] = struct{}{}
	}
	if len(measurements) != len(expected) {
		hardFailure("tools/list measurement count=%d, want %d", len(measurements), len(expected))
	}
	seen := make(map[string]struct{}, len(measurements))
	for _, measurement := range measurements {
		if measurement.Profile == "" {
			hardFailure("tools/list measurement has empty profile")
			continue
		}
		if _, ok := expected[measurement.Profile]; !ok {
			hardFailure("tools/list profile %s is unknown", measurement.Profile)
			continue
		}
		if _, ok := seen[measurement.Profile]; ok {
			hardFailure("tools/list profile %s is duplicated", measurement.Profile)
			continue
		}
		seen[measurement.Profile] = struct{}{}
		if measurement.ToolCount <= 0 || measurement.SchemaCount <= 0 || measurement.RequestBytes == 0 || measurement.ResponseBytes == 0 || measurement.ResultBytes == 0 || measurement.ApproxTokensBytes4 == 0 {
			hardFailure("tools/list profile %s measurement is structurally incomplete", measurement.Profile)
		}
	}
	for profile := range expected {
		if _, ok := seen[profile]; !ok {
			hardFailure("tools/list profile %s measurement is missing", profile)
		}
		if _, ok := budgets[profile]; !ok {
			hardFailure("tools/list profile %s budget is missing", profile)
		}
	}
	for profile := range budgets {
		if _, ok := expected[profile]; !ok {
			hardFailure("tools/list budget profile %s is unknown", profile)
		}
	}
}

func validateWorkflowMeasurements(measurements []WorkflowMeasurement, expectedNames []string, budgets map[string]workflowBudget, hardFailure func(string, ...any)) {
	expected := make(map[string]struct{}, len(expectedNames))
	for _, name := range expectedNames {
		expected[name] = struct{}{}
	}
	if len(measurements) != len(expected) {
		hardFailure("workflow measurement count=%d, want %d", len(measurements), len(expected))
	}
	seen := make(map[string]struct{}, len(measurements))
	for _, measurement := range measurements {
		if measurement.Name == "" {
			hardFailure("workflow measurement has empty name")
			continue
		}
		if _, ok := expected[measurement.Name]; !ok {
			hardFailure("workflow %s is unknown", measurement.Name)
			continue
		}
		if _, ok := seen[measurement.Name]; ok {
			hardFailure("workflow %s is duplicated", measurement.Name)
			continue
		}
		seen[measurement.Name] = struct{}{}
		if !completeWorkflowMeasurement(measurement) {
			hardFailure("workflow %s measurement is structurally incomplete", measurement.Name)
		}
	}
	for name := range expected {
		if _, ok := seen[name]; !ok {
			hardFailure("workflow %s measurement is missing", name)
		}
		if _, ok := budgets[name]; !ok {
			hardFailure("workflow %s budget is missing", name)
		}
	}
	for name := range budgets {
		if _, ok := expected[name]; !ok {
			hardFailure("workflow budget %s is unknown", name)
		}
	}
}

func completeWorkflowMeasurement(measurement WorkflowMeasurement) bool {
	if measurement.Repetitions <= 0 || measurement.Resources.Status == "" || measurement.WarningCount != len(measurement.StderrWarnings) {
		return false
	}
	summaries := []MetricSummary{
		measurement.RequestBytes, measurement.ResponseBytes, measurement.ResultBytes, measurement.DurationNS,
		measurement.ReadBytes, measurement.WrittenBytes, measurement.ScannedBytes, measurement.Entries,
		measurement.ApproxTokensBytes4,
	}
	for _, summary := range summaries {
		if summary.Samples != measurement.Repetitions {
			return false
		}
	}
	if measurement.RequestBytes.Max == 0 || measurement.ResponseBytes.Max == 0 || measurement.ResultBytes.Max == 0 || measurement.DurationNS.Max == 0 {
		return false
	}
	exitSamples := 0
	for _, count := range measurement.ExitStatuses {
		exitSamples += count
	}
	return exitSamples == measurement.Repetitions
}

func checkHardMaximum(evaluation *BudgetEvaluation, name string, actual uint64, maximum uint64) {
	if actual <= maximum {
		return
	}
	evaluation.HardFailures++
	evaluation.Messages = append(evaluation.Messages, fmt.Sprintf("hard: %s=%d exceeds %d", name, actual, maximum))
}
