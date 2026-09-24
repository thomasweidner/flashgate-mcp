package benchmark

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Options configures a benchmark run against a previously built server binary.
type Options struct {
	BinaryPath       string
	Commit           string
	WorkingTreeDirty bool
	Repetitions      int
	BudgetPath       string
}

type processSample struct {
	startupNS   uint64
	durationNS  uint64
	counters    Counters
	records     []responseRecord
	resources   sampleResources
	exitStatus  int
	warnings    []string
	unsupported []string
}

type sampleResources struct {
	status              string
	idleWorkingSetBytes *uint64
	peakWorkingSetBytes *uint64
	userCPUNS           *uint64
	systemCPUNS         *uint64
}

// Run measures the real FlashGate server process and evaluates optional budgets.
func Run(ctx context.Context, options Options) (Result, error) {
	if options.Repetitions < 1 {
		return Result{}, fmt.Errorf("repetitions must be at least one")
	}
	if options.BinaryPath == "" {
		return Result{}, fmt.Errorf("binary path is required")
	}
	binaryPath, err := filepath.Abs(options.BinaryPath)
	if err != nil {
		return Result{}, fmt.Errorf("normalize benchmark binary path: %w", err)
	}
	binaryPath = filepath.Clean(binaryPath)
	if _, err := os.Stat(binaryPath); err != nil {
		return Result{}, fmt.Errorf("inspect benchmark binary: %w", err)
	}
	options.BinaryPath = binaryPath

	fixtures, err := createCorpus()
	if err != nil {
		return Result{}, err
	}
	defer func() {
		_ = fixtures.remove()
	}()

	result := Result{
		SchemaVersion:          ResultSchemaVersion,
		BenchmarkSuiteVersion:  BenchmarkSuiteVersion,
		WorkflowCatalogVersion: WorkflowCatalogVersion,
		CorpusVersion:          CorpusVersion,
		Project:                "flashgate-mcp",
		Commit:                 options.Commit,
		WorkingTreeDirty:       options.WorkingTreeDirty,
		GeneratedAtUTC:         time.Now().UTC(),
		GoVersion:              runtime.Version(),
		OS:                     runtime.GOOS,
		Architecture:           runtime.GOARCH,
		RuntimeMode:            "direct_stdio",
		Transport:              "stdio",
		ExecutionBackend:       "current_process",
		Profile:                "read_only",
		Parallelism:            1,
		Repetitions:            options.Repetitions,
		Warnings:               []string{},
		UnsupportedMetrics:     []string{},
		BudgetEvaluation: BudgetEvaluation{
			SchemaVersion: BudgetSchemaVersion,
			Messages:      []string{},
		},
	}

	allSamples := make([]processSample, 0, 3+options.Repetitions*11)
	first, err := executeScenario(ctx, options.BinaryPath, fixtures.root, true, []requestSpec{initializeSpec()})
	if err != nil {
		return Result{}, fmt.Errorf("first process start: %w", err)
	}
	allSamples = append(allSamples, first)
	result.StartMeasurements.FirstProcessStart = startMeasurement([]processSample{first})

	subsequent := make([]processSample, 0, options.Repetitions)
	for index := 0; index < options.Repetitions; index++ {
		sample, err := executeScenario(ctx, options.BinaryPath, fixtures.root, true, []requestSpec{initializeSpec()})
		if err != nil {
			return Result{}, fmt.Errorf("subsequent process start %d: %w", index+1, err)
		}
		subsequent = append(subsequent, sample)
		allSamples = append(allSamples, sample)
	}
	result.StartMeasurements.SubsequentProcessStart = startMeasurement(subsequent)

	for _, profile := range []struct {
		name     string
		readOnly bool
	}{
		{name: "read_only", readOnly: true},
		{name: "default", readOnly: false},
	} {
		sample, err := executeScenario(ctx, options.BinaryPath, fixtures.root, profile.readOnly, []requestSpec{initializeSpec(), toolsListSpec()})
		if err != nil {
			return Result{}, fmt.Errorf("tools/list profile %s: %w", profile.name, err)
		}
		measurement, err := toolsListMeasurement(profile.name, sample)
		if err != nil {
			return Result{}, err
		}
		result.ToolsList = append(result.ToolsList, measurement)
		allSamples = append(allSamples, sample)
	}

	for _, workflow := range referenceWorkflows() {
		samples := make([]processSample, 0, options.Repetitions)
		for index := 0; index < options.Repetitions; index++ {
			sample, err := executeScenario(ctx, options.BinaryPath, fixtures.root, true, workflow.requests)
			if err != nil {
				return Result{}, fmt.Errorf("workflow %s repetition %d: %w", workflow.name, index+1, err)
			}
			if err := validateWorkflowUsefulOutput(workflow, sample.counters); err != nil {
				return Result{}, fmt.Errorf("workflow %s repetition %d useful output: %w", workflow.name, index+1, err)
			}
			samples = append(samples, sample)
			allSamples = append(allSamples, sample)
		}
		result.Workflows = append(result.Workflows, workflowMeasurement(workflow.name, samples))
	}

	result.Resources = summarizeResources(allSamples)
	result.Warnings = collectWarnings(allSamples)
	result.UnsupportedMetrics = collectUnsupported(allSamples)

	if options.BudgetPath != "" {
		if err := applyBudgetEvaluation(options.BudgetPath, &result); err != nil {
			return Result{}, err
		}
	}

	return result, nil
}

func applyBudgetEvaluation(path string, result *Result) error {
	evaluation, err := EvaluateBudgets(path, *result)
	if err != nil {
		return err
	}
	result.BudgetEvaluation = evaluation
	return nil
}

func executeScenario(ctx context.Context, binaryPath string, root string, readOnly bool, requests []requestSpec) (sample processSample, returnErr error) {
	scenarioContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	command := exec.CommandContext(scenarioContext, binaryPath)
	command.Env = benchmarkEnvironment(root, readOnly)
	stdin, err := command.StdinPipe()
	if err != nil {
		return processSample{}, fmt.Errorf("open stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return processSample{}, fmt.Errorf("open stdout: %w", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr

	startedAt := time.Now()
	if err := command.Start(); err != nil {
		return processSample{}, fmt.Errorf("start server: %w", err)
	}
	waited := false
	stdinClosed := false
	defer func() {
		if !stdinClosed {
			_ = stdin.Close()
		}
		if !waited {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
		_ = stdout.Close()
	}()

	reader := bufio.NewReader(stdout)
	metrics, metricsErr := newProcessMetricReader(command.Process.Pid)
	sample = processSample{exitStatus: -1}
	if metricsErr != nil {
		sample.warnings = append(sample.warnings, "process metrics unavailable on "+runtime.GOOS)
		sample.unsupported = append(sample.unsupported, unsupportedSnapshot().unsupported...)
	} else {
		defer func() {
			if closeErr := metrics.Close(); closeErr != nil {
				if returnErr != nil {
					returnErr = fmt.Errorf("%w (additionally, process metrics close failed on %s: %v)", returnErr, runtime.GOOS, closeErr)
				} else {
					sample.warnings = append(sample.warnings, "process metrics close failed on "+runtime.GOOS)
				}
			}
		}()
	}

	var idleSnapshot processSnapshot
	for index, spec := range requests {
		request, err := marshalRequest(index+1, spec)
		if err != nil {
			return sample, err
		}
		if _, err := stdin.Write(request); err != nil {
			return sample, fmt.Errorf("write %s request: %w", spec.Method, err)
		}
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return sample, fmt.Errorf("read %s response: %w", spec.Method, err)
		}
		response, err := decodeResponse(line, index+1, spec.Method)
		if err != nil {
			return sample, err
		}
		if spec.Method == "initialize" {
			if err := validateInitializeResult(response.Result); err != nil {
				return sample, err
			}
		}
		record := responseRecord{
			method:        spec.Method,
			requestBytes:  uint64(len(request)),
			responseBytes: uint64(len(line)),
			resultBytes:   uint64(len(response.Result)),
			result:        append([]byte(nil), response.Result...),
		}
		sample.records = append(sample.records, record)
		sample.counters.RequestBytes += record.requestBytes
		sample.counters.ResponseBytes += record.responseBytes
		sample.counters.ResultBytes += record.resultBytes
		if err := addResultCounters(spec.Method, response.Result, &sample.counters); err != nil {
			return sample, err
		}

		if index == 0 {
			sample.startupNS = uint64(time.Since(startedAt).Nanoseconds())
		}
		sample.durationNS = uint64(time.Since(startedAt).Nanoseconds())

		if spec.Method == "initialize" {
			notificationBytes, err := writeNotification(stdin, initializedNotificationSpec())
			if err != nil {
				return sample, err
			}
			sample.counters.RequestBytes += notificationBytes
		}

		if index == 0 {
			if metrics != nil {
				idleSnapshot, err = metrics.Snapshot()
				if err != nil {
					sample.warnings = append(sample.warnings, "idle process metrics unavailable on "+runtime.GOOS)
					sample.unsupported = append(sample.unsupported, unsupportedSnapshot().unsupported...)
				} else {
					sample.unsupported = append(sample.unsupported, idleSnapshot.unsupported...)
				}
			}
		}
	}

	var finalSnapshot processSnapshot
	if metrics != nil {
		finalSnapshot, err = metrics.Snapshot()
		if err != nil {
			sample.warnings = append(sample.warnings, "final process metrics unavailable on "+runtime.GOOS)
			sample.unsupported = append(sample.unsupported, unsupportedSnapshot().unsupported...)
		} else {
			sample.unsupported = append(sample.unsupported, finalSnapshot.unsupported...)
		}
	}
	sample.resources = combineResourceSnapshots(idleSnapshot, finalSnapshot)
	if sample.resources.status == "" {
		sample.resources.status = "not_supported"
	}

	exitStatus, waitErr := controlledExit(command, stdin)
	waited = true
	stdinClosed = true
	sample.exitStatus = exitStatus
	sample.warnings = append(sample.warnings, sanitizeStderr(stderr.String(), root, binaryPath)...)
	if waitErr != nil {
		return sample, waitErr
	}
	return sample, nil
}

func controlledExit(command *exec.Cmd, stdin interface{ Close() error }) (int, error) {
	closeErr := stdin.Close()
	if closeErr != nil {
		_ = command.Process.Kill()
	}
	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case err := <-wait:
		exitStatus := command.ProcessState.ExitCode()
		if closeErr != nil {
			return exitStatus, fmt.Errorf("close server stdin: %w", closeErr)
		}
		if err != nil {
			return exitStatus, fmt.Errorf("server exit status %d: %w", exitStatus, err)
		}
		return exitStatus, nil
	case <-timer.C:
		_ = command.Process.Kill()
		waitErr := <-wait
		if waitErr != nil {
			return command.ProcessState.ExitCode(), fmt.Errorf("server controlled exit timed out: %w", waitErr)
		}
		return command.ProcessState.ExitCode(), fmt.Errorf("server controlled exit timed out")
	}
}

func benchmarkEnvironment(root string, readOnly bool) []string {
	environment := []string{
		"MCP_ROOT=" + root,
		"MCP_READ_ONLY=" + strconv.FormatBool(readOnly),
		"MCP_ALLOW_CWD_ROOT=false",
		"MCP_DEBUG=false",
	}
	for _, key := range []string{"SYSTEMROOT", "WINDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			environment = append(environment, key+"="+value)
		}
	}
	return environment
}

func combineResourceSnapshots(idle processSnapshot, final processSnapshot) sampleResources {
	status := final.status
	if status == "" {
		status = idle.status
	}
	return sampleResources{
		status:              status,
		idleWorkingSetBytes: idle.workingSetBytes,
		peakWorkingSetBytes: final.peakWorkingSetBytes,
		userCPUNS:           final.userCPUNS,
		systemCPUNS:         final.systemCPUNS,
	}
}

func startMeasurement(samples []processSample) StartMeasurement {
	measurement := StartMeasurement{
		Repetitions:    len(samples),
		DurationNS:     summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.startupNS })),
		RequestBytes:   summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.counters.RequestBytes })),
		ResponseBytes:  summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.counters.ResponseBytes })),
		ResultBytes:    summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.counters.ResultBytes })),
		Resources:      summarizeResources(samples),
		ExitStatuses:   exitStatuses(samples),
		StderrWarnings: collectWarnings(samples),
	}
	measurement.WarningCount = len(measurement.StderrWarnings)
	return measurement
}

func toolsListMeasurement(profile string, sample processSample) (ToolsListMeasurement, error) {
	for _, record := range sample.records {
		if record.method != "tools/list" {
			continue
		}
		toolCount, schemaCount, err := decodeToolsList(record.result)
		if err != nil {
			return ToolsListMeasurement{}, err
		}
		return ToolsListMeasurement{
			Profile:            profile,
			ToolCount:          toolCount,
			SchemaCount:        schemaCount,
			RequestBytes:       record.requestBytes,
			ResponseBytes:      record.responseBytes,
			ResultBytes:        record.resultBytes,
			ApproxTokensBytes4: approxTokensBytes4(record.responseBytes),
		}, nil
	}
	return ToolsListMeasurement{}, fmt.Errorf("tools/list response record missing")
}

func workflowMeasurement(name string, samples []processSample) WorkflowMeasurement {
	measurement := WorkflowMeasurement{
		Name:               name,
		Repetitions:        len(samples),
		RequestBytes:       summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.counters.RequestBytes })),
		ResponseBytes:      summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.counters.ResponseBytes })),
		ResultBytes:        summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.counters.ResultBytes })),
		DurationNS:         summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.durationNS })),
		ReadBytes:          summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.counters.ReadBytes })),
		WrittenBytes:       summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.counters.WrittenBytes })),
		ScannedBytes:       summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.counters.ScannedBytes })),
		Entries:            summarize(sampleValues(samples, func(sample processSample) uint64 { return sample.counters.Entries })),
		ApproxTokensBytes4: summarize(sampleValues(samples, func(sample processSample) uint64 { return approxTokensBytes4(sample.counters.ResponseBytes) })),
		Resources:          summarizeResources(samples),
		ExitStatuses:       exitStatuses(samples),
		StderrWarnings:     collectWarnings(samples),
	}
	if len(samples) > 0 {
		measurement.Calls = samples[0].counters.Calls
	}
	measurement.WarningCount = len(measurement.StderrWarnings)
	return measurement
}

func sampleValues(samples []processSample, selectValue func(processSample) uint64) []uint64 {
	values := make([]uint64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, selectValue(sample))
	}
	return values
}

func summarizeResources(samples []processSample) ResourceSummary {
	idle := optionalSampleValues(samples, func(sample processSample) *uint64 { return sample.resources.idleWorkingSetBytes })
	peak := optionalSampleValues(samples, func(sample processSample) *uint64 { return sample.resources.peakWorkingSetBytes })
	userCPU := optionalSampleValues(samples, func(sample processSample) *uint64 { return sample.resources.userCPUNS })
	systemCPU := optionalSampleValues(samples, func(sample processSample) *uint64 { return sample.resources.systemCPUNS })
	result := ResourceSummary{Status: "not_supported"}
	if len(idle)+len(peak)+len(userCPU)+len(systemCPU) > 0 {
		result.Status = "supported"
	}
	if len(idle) > 0 {
		value := summarize(idle)
		result.IdleWorkingSetBytes = &value
	}
	if len(peak) > 0 {
		value := summarize(peak)
		result.PeakWorkingSetBytes = &value
	}
	if len(userCPU) > 0 {
		value := summarize(userCPU)
		result.UserCPUNS = &value
	}
	if len(systemCPU) > 0 {
		value := summarize(systemCPU)
		result.SystemCPUNS = &value
	}
	return result
}

func optionalSampleValues(samples []processSample, selectValue func(processSample) *uint64) []uint64 {
	values := make([]uint64, 0, len(samples))
	for _, sample := range samples {
		if value := selectValue(sample); value != nil {
			values = append(values, *value)
		}
	}
	return values
}

func exitStatuses(samples []processSample) map[string]int {
	result := make(map[string]int)
	for _, sample := range samples {
		result[strconv.Itoa(sample.exitStatus)]++
	}
	return result
}

func collectWarnings(samples []processSample) []string {
	warnings := []string{}
	for _, sample := range samples {
		warnings = append(warnings, sample.warnings...)
	}
	return uniqueSorted(warnings)
}

func collectUnsupported(samples []processSample) []string {
	values := []string{}
	for _, sample := range samples {
		values = append(values, sample.unsupported...)
	}
	return uniqueSorted(values)
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func sanitizeStderr(stderr string, root string, binaryPath string) []string {
	if strings.TrimSpace(stderr) == "" {
		return nil
	}
	replacements := validRedactionPaths(root, hostPathDir(binaryPath), os.TempDir())
	result := []string{}
	for _, line := range strings.Split(stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, path := range replacements {
			line = path.pattern.ReplaceAllString(line, "[REDACTED_PATH]")
		}
		result = append(result, line)
	}
	return uniqueSorted(result)
}

type redactionPath struct {
	pattern *regexp.Regexp
}

func validRedactionPaths(paths ...string) []redactionPath {
	result := make([]redactionPath, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		cleaned := strings.TrimSpace(path)
		if !isNontrivialAbsolutePath(cleaned) {
			continue
		}
		cleaned = strings.TrimRight(cleaned, `/\`)
		key := strings.ToLower(strings.ReplaceAll(cleaned, `\`, "/"))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		var expression strings.Builder
		if isWindowsAbsolutePath(cleaned) {
			expression.WriteString("(?i)")
		}
		for _, character := range cleaned {
			if character == '/' || character == '\\' {
				expression.WriteString(`[/\\]`)
			} else {
				expression.WriteString(regexp.QuoteMeta(string(character)))
			}
		}
		result = append(result, redactionPath{pattern: regexp.MustCompile(expression.String())})
	}
	return result
}

func isNontrivialAbsolutePath(path string) bool {
	if len(path) < 4 || path == "." {
		return false
	}
	if isWindowsAbsolutePath(path) {
		trimmed := strings.TrimRight(path, `/\`)
		return len(trimmed) > 2
	}
	if strings.HasPrefix(path, "/") {
		return strings.Trim(path, "/") != ""
	}
	return false
}

func isWindowsAbsolutePath(path string) bool {
	if len(path) >= 3 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}
	return strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`)
}

func hostPathDir(path string) string {
	if isWindowsAbsolutePath(path) {
		normalized := strings.ReplaceAll(path, `\`, "/")
		index := strings.LastIndexByte(normalized, '/')
		if index <= 2 {
			return normalized[:index+1]
		}
		return normalized[:index]
	}
	return filepath.Dir(path)
}
