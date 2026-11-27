//go:build windows || linux

package benchmark

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type auditScenario struct {
	name     string
	settings string
	required bool
}

const auditRequiredSettings = "failure_policy = 'audit_required'\nrequired_operations = ['filesystem.write']"

var auditScenarios = []auditScenario{
	{name: "disabled", settings: "enabled = false"},
	{name: "default", settings: "enabled = true\nfailure_policy = 'operation_first'"},
	{name: "single-record", settings: "max_batch_events = 1\nflush_interval = '10 ms'"},
	{name: "batch-32", settings: "max_batch_events = 32\nflush_interval = '25 ms'"},
	{name: "required", settings: auditRequiredSettings, required: true},
	{name: "required-5s", settings: auditRequiredSettings + "\nrequired_intent_timeout = '5 s'", required: true},
	{name: "pressure", settings: "max_queue_events = 4\nmax_queue_size = '16 KiB'\nmax_batch_events = 4\nmax_batch_size = '16 KiB'"},
	{name: "rotation", settings: "max_segment_size = '16 KiB'\nmax_store_size = '64 KiB'"},
}

// BenchmarkAuditSTDIO reuses the real subprocess, protocol, timing and
// resource collectors. Run it only in the documented measurement window.
func BenchmarkAuditSTDIO(b *testing.B) {
	binary := auditBinary(b, "FLASHGATE_AUDIT_BENCH_BINARY")
	reference := auditBinary(b, "FLASHGATE_AUDIT_REFERENCE_BINARY")
	for _, workers := range []int{1, 10} {
		b.Run(fmt.Sprintf("reference/processes-%d", workers), func(b *testing.B) {
			measureAudit(b, reference, auditScenario{name: "reference"}, workers)
		})
		for _, scenario := range auditScenarios {
			b.Run(fmt.Sprintf("%s/processes-%d", scenario.name, workers), func(b *testing.B) {
				measureAudit(b, binary, scenario, workers)
			})
		}
	}
}

func auditBinary(t testing.TB, key string) string {
	t.Helper()
	binary := os.Getenv(key)
	if binary == "" {
		t.Skipf("bind an absolute server binary with %s", key)
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("server binary path must be absolute")
	}
	return binary
}

func auditWorkloadFixture(t testing.TB, settings string) (config, data string) {
	t.Helper()
	dir := t.TempDir()
	config = filepath.Join(dir, "flashgate_config.toml")
	data = filepath.Join(dir, "data")
	denied := filepath.Join(dir, "denied")
	for _, path := range []string{data, denied} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(data, "read.txt"), []byte(strings.Repeat("data", 256)), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	deniedRoot, err := json.Marshal(denied)
	if err != nil {
		t.Fatal(err)
	}
	doc := fmt.Sprintf("[files]\ndefault_id = 'work'\nwork.path = %[1]s\nwork.access = ['files.read','files.write']\n"+
		"denied.path = %[2]s\ndenied.access = ['files.write']\n[profiles]\nactive = 'write'\n", root, deniedRoot)
	if settings != "" {
		doc += "[audit]\n" + settings + "\n"
	}
	if err := writeConfigFixture(config, []byte(doc)); err != nil {
		t.Fatal(err)
	}
	return config, data
}

func auditRequests(worker, repetitions int) []requestSpec {
	requests := []requestSpec{initializeSpec()}
	for n := range repetitions {
		requests = append(requests, toolCallSpec("read_file", map[string]any{"root_id": "work", "path": "read.txt"}))
		invalid := toolCallSpec("read_file", map[string]any{"root_id": "denied", "path": "read.txt"})
		invalid.expectToolError = true
		requests = append(requests, invalid, toolCallSpec("write_file", map[string]any{
			"root_id": "work", "path": fmt.Sprintf("write-%d-%d.txt", worker, n), "content": "bench", "overwrite": true,
		}))
	}
	return requests
}

// Exercise the workload configuration and native counters during functional
// validation without collecting or reporting timing values.
func TestAuditSTDIOFixture(t *testing.T) {
	scenarios := append([]auditScenario{{name: "reference"}}, auditScenarios...)
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			key := "FLASHGATE_AUDIT_FIXTURE_BINARY"
			if scenario.name == "reference" {
				key = "FLASHGATE_AUDIT_REFERENCE_BINARY"
			}
			binary := auditBinary(t, key)
			config, data := auditWorkloadFixture(t, scenario.settings)
			sample, err := executeObservedScenario(t.Context(), binary, data, config, auditRequests(0, 1), func(pid int) error {
				_, err := auditProcessIO(pid)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			if sample.counters.Calls != 3 || sample.counters.ReadBytes != 1024 {
				t.Fatalf("unexpected fixture counters: %+v", sample.counters)
			}
			if written, err := os.ReadFile(filepath.Join(data, "write-0-0.txt")); err != nil || string(written) != "bench" {
				t.Fatalf("fixture write could not be verified: %v", err)
			}
		})
	}
}

func TestAuditRequiredRefusalBinding(t *testing.T) {
	wireError := func(category, message string) []byte {
		t.Helper()
		result, err := protocol.NewCallToolErrorResult(category, message)
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	denial := wireError("access_denied", "filesystem error: access denied")
	unexpected := wireError("invalid_path", "filesystem error: invalid path")
	request := toolCallSpec("write_file", nil)
	if err := addResultCounters(request, denial, &Counters{}); err == nil {
		t.Fatal("ordinary write fixture accepted a denial")
	}
	request.allowAccessDenied = true
	if err := addResultCounters(request, denial, &Counters{}); err != nil {
		t.Fatal(err)
	}
	if err := addResultCounters(request, unexpected, &Counters{}); err == nil {
		t.Fatal("required write fixture accepted an unrelated failure")
	}
	read := toolCallSpec("read_file", nil)
	read.allowAccessDenied = true
	if err := addResultCounters(read, denial, &Counters{}); err == nil {
		t.Fatal("required write allowance leaked to a read fixture")
	}
}

func measureAudit(b *testing.B, binary string, scenario auditScenario, workers int) {
	config, data := auditWorkloadFixture(b, scenario.settings)
	var allowed, denied, writes, refusals, startup, shutdown, peak, cpu, readIO, writeIO []uint64
	var totalRequests int
	var elapsed time.Duration
	iteration := 0
	for b.Loop() {
		started := time.Now()
		samples := make([]processSample, workers)
		counters := make([][2]uint64, workers)
		failures := make([]error, workers)
		var group sync.WaitGroup
		for index := range workers {
			group.Go(func() {
				requests := auditRequests(iteration*workers+index, 8)
				if scenario.required {
					for i := 3; i < len(requests); i += 3 {
						requests[i].allowAccessDenied = true
					}
				}
				samples[index], failures[index] = executeObservedScenario(b.Context(), binary, data, config, requests, func(pid int) error {
					var err error
					counters[index], err = auditProcessIO(pid)
					return err
				})
			})
		}
		group.Wait()
		elapsed += time.Since(started)
		for i, sample := range samples {
			if failures[i] != nil {
				b.Fatal(failures[i])
			}
			startup = append(startup, sample.startupNS)
			shutdown = append(shutdown, sample.shutdownNS)
			readIO = append(readIO, counters[i][0])
			writeIO = append(writeIO, counters[i][1])
			totalRequests += int(sample.counters.Calls)
			if sample.resources.peakWorkingSetBytes != nil {
				peak = append(peak, *sample.resources.peakWorkingSetBytes)
			}
			if sample.resources.userCPUNS != nil && sample.resources.systemCPUNS != nil {
				cpu = append(cpu, *sample.resources.userCPUNS+*sample.resources.systemCPUNS)
			}
			for j, record := range sample.records[1:] {
				switch j % 3 {
				case 0:
					allowed = append(allowed, record.elapsedNS)
				case 1:
					denied = append(denied, record.elapsedNS)
				case 2:
					var result toolResultEnvelope
					if err := json.Unmarshal(record.result, &result); err != nil {
						b.Fatal(err)
					}
					leaf := fmt.Sprintf("write-%d-%d.txt", iteration*workers+i, j/3)
					path := filepath.Join(data, leaf)
					if result.accessDenied() {
						if _, err := os.Lstat(path); !os.IsNotExist(err) {
							b.Fatalf("refused write did not leave an absent target: %v", err)
						}
						refusals = append(refusals, record.elapsedNS)
					} else {
						writes = append(writes, record.elapsedNS)
					}
				}
			}
		}
		iteration++
	}
	for _, metric := range []struct {
		name   string
		values []uint64
	}{
		{"allowed_read_ns", allowed}, {"denied_read_ns", denied}, {"write_ns", writes}, {"refused_write_ns", refusals},
		{"startup_ns", startup}, {"shutdown_ns", shutdown},
		{"peak_rss_B", peak}, {"cpu_ns", cpu},
		{auditReadIOName, readIO}, {auditWriteIOName, writeIO},
	} {
		if data := summarize(metric.values); data.Samples != 0 {
			b.ReportMetric(float64(data.P50), metric.name+"_p50")
			b.ReportMetric(float64(data.P95), metric.name+"_p95")
		}
	}
	b.ReportMetric(float64(totalRequests)/elapsed.Seconds(), "requests/s")
	b.ReportMetric(float64(len(refusals)), "refused_writes")
	b.ReportMetric(float64(len(writes)), "completed_writes")
	var retained int64
	var retainedEvents uint64
	var retainedLosses [5]uint64
	store := filepath.Join(filepath.Dir(config), "audit")
	if err := filepath.WalkDir(store, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		retained += info.Size()
		if filepath.Ext(path) == ".jsonl" {
			events, losses, err := auditSegmentMetrics(path)
			if err != nil {
				return err
			}
			retainedEvents += events
			for cause, count := range losses {
				retainedLosses[cause] += count
			}
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		b.Fatal(err)
	}
	b.ReportMetric(float64(retained), "retained_B")
	b.ReportMetric(float64(retainedEvents), "retained_events")
	for cause, name := range [...]string{"pressure", "projection", "write_failure", "shutdown", "intent_timeout"} {
		b.ReportMetric(float64(retainedLosses[cause]), "retained_loss_"+name)
	}
}

func auditSegmentMetrics(path string) (events uint64, losses [5]uint64, err error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, losses, err
	}
	defer file.Close()
	lines := bufio.NewScanner(file)
	lines.Buffer(make([]byte, 1024), 128<<10)
	for lines.Scan() {
		var record struct {
			Phase  string      `json:"phase"`
			Losses [][5]uint64 `json:"losses"`
		}
		if err := json.Unmarshal(lines.Bytes(), &record); err != nil {
			return 0, losses, fmt.Errorf("decode retained audit record: %w", err)
		}
		if record.Phase == "segment" {
			continue
		}
		events++
		for _, priority := range record.Losses {
			for cause, count := range priority {
				losses[cause] += count
			}
		}
	}
	return events, losses, lines.Err()
}
