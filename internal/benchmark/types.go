package benchmark

import "time"

const (
	// ResultSchemaVersion identifies the machine-readable benchmark result contract.
	ResultSchemaVersion = "flashgate-benchmark/v1"
	// BudgetSchemaVersion identifies the machine-readable regression budget contract.
	BudgetSchemaVersion = "flashgate-benchmark-budgets/v1"
	// BenchmarkSuiteVersion identifies the benchmark implementation and measurement semantics.
	BenchmarkSuiteVersion = "flashgate-benchmark-suite/v1"
	// WorkflowCatalogVersion identifies the reference workflow catalog.
	WorkflowCatalogVersion = "flashgate-benchmark-workflows/v1"
	// CorpusVersion identifies the deterministic benchmark fixture corpus.
	CorpusVersion = "flashgate-benchmark-corpus/v1"
)

// Result is the versioned, host-path-free benchmark result.
type Result struct {
	SchemaVersion           string                   `json:"schema_version"`
	BenchmarkSuiteVersion   string                   `json:"benchmark_suite_version"`
	WorkflowCatalogVersion  string                   `json:"workflow_catalog_version"`
	CorpusVersion           string                   `json:"corpus_version"`
	Project                 string                   `json:"project"`
	Commit                  string                   `json:"commit"`
	WorkingTreeDirty        bool                     `json:"working_tree_dirty"`
	GeneratedAtUTC          time.Time                `json:"generated_at_utc"`
	GoVersion               string                   `json:"go_version"`
	OS                      string                   `json:"os"`
	Architecture            string                   `json:"architecture"`
	RuntimeMode             string                   `json:"runtime_mode"`
	Transport               string                   `json:"transport"`
	ExecutionBackend        string                   `json:"execution_backend"`
	Profile                 string                   `json:"profile"`
	Parallelism             int                      `json:"parallelism"`
	Repetitions             int                      `json:"repetitions"`
	StartMeasurements       ProcessStartMeasurements `json:"start_measurements"`
	Resources               ResourceSummary          `json:"resource_measurements"`
	ToolsList               []ToolsListMeasurement   `json:"tools_list_measurements"`
	Workflows               []WorkflowMeasurement    `json:"workflow_measurements"`
	BudgetEvaluation        BudgetEvaluation         `json:"budget_evaluation"`
	Warnings                []string                 `json:"warnings"`
	UnsupportedMetrics      []string                 `json:"unsupported_metrics"`
	AuthoritativeProvenance *AuthoritativeProvenance `json:"authoritative_provenance,omitempty"`
}

// AuthoritativeProvenance binds a native baseline measurement to its prepared
// binary, source/build inputs, controller, workspace and host-gate evidence.
// Paths and host identifiers are deliberately not retained.
type AuthoritativeProvenance struct {
	BinarySHA256              string    `json:"binary_sha256"`
	SourceTreeSHA256          string    `json:"source_tree_sha256"`
	BuildInputsSHA256         string    `json:"build_inputs_sha256"`
	ControllerSHA256          string    `json:"controller_sha256"`
	WorkspaceIdentitySHA256   string    `json:"workspace_identity_sha256"`
	PreflightEvidenceSHA256   string    `json:"preflight_evidence_sha256"`
	FinalHostGateSHA256       string    `json:"final_host_gate_sha256"`
	PreparationStartedAtUTC   time.Time `json:"preparation_started_at_utc"`
	PreparationCompletedAtUTC time.Time `json:"preparation_completed_at_utc"`
	MeasurementStartedAtUTC   time.Time `json:"measurement_started_at_utc"`
	MeasurementCompletedAtUTC time.Time `json:"measurement_completed_at_utc"`
}

// ProcessStartMeasurements separates the first start after build from later new processes.
type ProcessStartMeasurements struct {
	FirstProcessStart      StartMeasurement `json:"first_process_start"`
	SubsequentProcessStart StartMeasurement `json:"subsequent_process_start"`
}

// StartMeasurement aggregates real process startup samples through initialize.
type StartMeasurement struct {
	Repetitions    int             `json:"repetitions"`
	DurationNS     MetricSummary   `json:"duration_ns"`
	RequestBytes   MetricSummary   `json:"request_bytes"`
	ResponseBytes  MetricSummary   `json:"response_bytes"`
	ResultBytes    MetricSummary   `json:"result_bytes"`
	Resources      ResourceSummary `json:"resources"`
	ExitStatuses   map[string]int  `json:"exit_statuses"`
	StderrWarnings []string        `json:"stderr_warnings"`
	WarningCount   int             `json:"stderr_warning_count"`
}

// ResourceSummary aggregates supported process resource measurements.
type ResourceSummary struct {
	Status              string         `json:"status"`
	IdleWorkingSetBytes *MetricSummary `json:"idle_working_set_bytes,omitempty"`
	PeakWorkingSetBytes *MetricSummary `json:"peak_working_set_bytes,omitempty"`
	UserCPUNS           *MetricSummary `json:"user_cpu_ns,omitempty"`
	SystemCPUNS         *MetricSummary `json:"system_cpu_ns,omitempty"`
}

// ToolsListMeasurement records deterministic tools/list wire sizes for one profile.
type ToolsListMeasurement struct {
	Profile            string `json:"profile"`
	ToolCount          int    `json:"tool_count"`
	SchemaCount        int    `json:"schema_count"`
	RequestBytes       uint64 `json:"request_bytes"`
	ResponseBytes      uint64 `json:"response_bytes"`
	ResultBytes        uint64 `json:"result_bytes"`
	ApproxTokensBytes4 uint64 `json:"approx_tokens_bytes4"`
}

// WorkflowMeasurement aggregates one reference workflow over new server processes.
type WorkflowMeasurement struct {
	Name               string          `json:"name"`
	Repetitions        int             `json:"repetitions"`
	Calls              int             `json:"calls"`
	RequestBytes       MetricSummary   `json:"request_bytes"`
	ResponseBytes      MetricSummary   `json:"response_bytes"`
	ResultBytes        MetricSummary   `json:"result_bytes"`
	DurationNS         MetricSummary   `json:"duration_ns"`
	ReadBytes          MetricSummary   `json:"read_bytes"`
	WrittenBytes       MetricSummary   `json:"written_bytes"`
	ScannedBytes       MetricSummary   `json:"scanned_bytes"`
	Entries            MetricSummary   `json:"entries"`
	ApproxTokensBytes4 MetricSummary   `json:"approx_tokens_bytes4"`
	Resources          ResourceSummary `json:"resources"`
	ExitStatuses       map[string]int  `json:"exit_statuses"`
	StderrWarnings     []string        `json:"stderr_warnings"`
	WarningCount       int             `json:"stderr_warning_count"`
}

// BudgetEvaluation reports hard failures separately from soft review warnings.
type BudgetEvaluation struct {
	SchemaVersion string   `json:"schema_version"`
	HardFailures  int      `json:"hard_failures"`
	SoftWarnings  int      `json:"soft_warnings"`
	Messages      []string `json:"messages"`
}
