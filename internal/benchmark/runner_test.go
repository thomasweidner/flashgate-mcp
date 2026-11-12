package benchmark

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestBenchmarkConfigFixtures(t *testing.T) {
	t.Parallel()
	dir, dataRoot := t.TempDir(), t.TempDir()
	encodedRoot, err := json.Marshal(dataRoot)
	if err != nil {
		t.Fatal("encoding fixture root failed")
	}
	guard, err := security.NewPathGuard(dataRoot)
	if err != nil {
		t.Fatal("opening fixture data root failed")
	}
	for _, tc := range []struct {
		name     string
		readOnly bool
	}{
		{"read-only.toml", true},
		{"write.toml", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name)
			if err := writeBenchmarkConfig(path, encodedRoot, tc.readOnly); err != nil {
				t.Fatalf("creating benchmark configuration failed: %v", err)
			}
			cfg, err := config.Load(config.Source{Role: config.Direct, Path: path})
			if err != nil {
				t.Fatalf("loading trusted benchmark configuration failed: %v", err)
			}
			if cfg.Filesystem().RootPath() != dataRoot || cfg.Filesystem().ReadOnly() != tc.readOnly {
				t.Fatal("benchmark configuration profile changed")
			}
			if err := cfg.ValidateDataRoot(guard); err != nil {
				t.Fatal("benchmark configuration and data root were not separate")
			}
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				t.Fatal("benchmark configuration is not a regular file")
			}
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
				t.Fatal("benchmark configuration is not owner-only")
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal("reading benchmark fixture failed")
			}
			if err := writeBenchmarkConfig(path, encodedRoot, !tc.readOnly); err == nil {
				t.Fatal("existing benchmark fixture was accepted for replacement")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("existing benchmark fixture changed")
			}
			if err := os.Remove(path); err != nil {
				t.Fatal("benchmark fixture handle was not released")
			}
		})
	}
}

func TestBenchmarkConfigRejectsAliasedParent(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if runtime.GOOS == "windows" {
		if err := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", alias, target).Run(); err != nil {
			t.Fatal("creating native junction fixture failed")
		}
	} else if err := os.Symlink(target, alias); err != nil {
		t.Fatal("creating native symlink fixture failed")
	}
	if err := writeConfigFixture(filepath.Join(alias, "config.toml"), []byte("data")); err == nil {
		t.Fatal("aliased configuration parent was accepted")
	}
	if _, err := os.Lstat(filepath.Join(target, "config.toml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("configuration write reached the alias target")
	}
}

func TestBenchmarkEnvironmentIsMinimalAndDeterministic(t *testing.T) {
	t.Setenv("SYSTEMROOT", "test-system-root")
	t.Setenv("WINDIR", "test-windows-directory")
	t.Setenv("MCP_ROOT", "legacy-root")
	t.Setenv("MCP_MAX_LIST_ENTRIES", "1")
	t.Setenv("FLASHGATE_PRIVATE_TEST_VALUE", "must-not-be-copied")

	environment := benchmarkEnvironment()
	joined := strings.Join(environment, "\n")
	for _, required := range []string{
		"SYSTEMROOT=test-system-root",
		"WINDIR=test-windows-directory",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("benchmark environment omitted %q: %q", required, environment)
		}
	}
	for _, forbidden := range []string{"MCP_ROOT=", "MCP_MAX_LIST_ENTRIES=", "FLASHGATE_PRIVATE_TEST_VALUE="} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("benchmark environment copied %q", forbidden)
		}
	}
}

func TestApplyBudgetEvaluationKeepsWarningChannelsSeparate(t *testing.T) {
	result := completeBudgetTestResult()
	result.Warnings = collectWarnings([]processSample{{warnings: []string{"runtime warning"}}})
	result.Workflows[0].DurationNS = MetricSummary{
		Samples: 1,
		Min:     100_000_001,
		P50:     100_000_001,
		P95:     100_000_001,
		Max:     100_000_001,
	}

	if err := applyBudgetEvaluation(filepath.Join("..", "..", "benchmarks", "budgets.json"), &result); err != nil {
		t.Fatal(err)
	}
	if result.BudgetEvaluation.HardFailures != 0 || result.BudgetEvaluation.SoftWarnings != 1 {
		t.Fatalf("budget evaluation=%+v, want zero hard and one soft warning", result.BudgetEvaluation)
	}
	if len(result.BudgetEvaluation.Messages) != 1 || !strings.HasPrefix(result.BudgetEvaluation.Messages[0], "soft: ") {
		t.Fatalf("budget messages=%q, want one soft message", result.BudgetEvaluation.Messages)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "runtime warning" {
		t.Fatalf("general warnings=%q, want only the runtime warning", result.Warnings)
	}
}

func TestInitializedNotificationAndInitializeValidation(t *testing.T) {
	var output bytes.Buffer
	written, err := writeNotification(&output, initializedNotificationSpec())
	if err != nil {
		t.Fatal(err)
	}
	notification := output.Bytes()
	want := "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n"
	if string(notification) != want {
		t.Fatalf("initialized notification=%q, want %q", notification, want)
	}
	if written != uint64(len(notification)) || written != 55 {
		t.Fatalf("initialized notification bytes=%d, want 55", written)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(notification, &envelope); err != nil {
		t.Fatal(err)
	}
	if _, ok := envelope["id"]; ok {
		t.Fatal("initialized notification must not contain id")
	}

	valid := json.RawMessage(`{"protocolVersion":"2025-11-25","capabilities":{"tools":{}},"serverInfo":{"name":"flashgate-mcp","version":"dev"}}`)
	if err := validateInitializeResult(valid); err != nil {
		t.Fatalf("valid initialize result rejected: %v", err)
	}
	invalid := map[string]json.RawMessage{
		"wrong protocol":   json.RawMessage(`{"protocolVersion":"2024-01-01","capabilities":{},"serverInfo":{"name":"server","version":"v1"}}`),
		"missing protocol": json.RawMessage(`{"capabilities":{},"serverInfo":{"name":"server","version":"v1"}}`),
		"missing info":     json.RawMessage(`{"protocolVersion":"2025-11-25","capabilities":{}}`),
		"missing caps":     json.RawMessage(`{"protocolVersion":"2025-11-25","serverInfo":{"name":"server","version":"v1"}}`),
		"null caps":        json.RawMessage(`{"protocolVersion":"2025-11-25","capabilities":null,"serverInfo":{"name":"server","version":"v1"}}`),
	}
	for name, result := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := validateInitializeResult(result); err == nil {
				t.Fatal("invalid initialize result accepted")
			}
		})
	}
}

func TestToolResultCounters(t *testing.T) {
	for _, tc := range []struct {
		name, tool, result string
		readBytes, entries uint64
		wantError          bool
	}{
		{name: "split read", tool: "read_file", result: `{"content":[{"type":"text","text":"é\n"}],"structuredContent":{"size":3}}`, readBytes: 3},
		{name: "legacy read", tool: "read_file", result: `{"structuredContent":{"content":"FlashGate benchmark text.\n","size":26}}`, readBytes: 26},
		{name: "empty read", tool: "read_file", result: `{"content":[{"type":"text","text":""}],"structuredContent":{"size":0}}`},
		{name: "path size", tool: "get_path_info", result: `{"structuredContent":{"exists":true,"size":26}}`},
		{name: "write size", tool: "write_file", result: `{"structuredContent":{"path":"out.txt","size":26}}`},
		{name: "directory entries", tool: "list_directory", result: `{"structuredContent":{"entries":[{"name":"a.txt"},{"name":"b.txt"}]}}`, entries: 2},
		{name: "read missing size", tool: "read_file", result: `{"structuredContent":{}}`, wantError: true},
		{name: "read tool error", tool: "read_file", result: `{"isError":true,"structuredContent":{"size":26}}`, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var counters Counters
			err := addResultCounters(toolCallSpec(tc.tool, nil), json.RawMessage(tc.result), &counters)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v, wantError %v", err, tc.wantError)
			}
			if counters.Calls != 1 || counters.ReadBytes != tc.readBytes || counters.ScannedBytes != 0 || counters.Entries != tc.entries {
				t.Fatalf("counters = %+v, want one call, %d read bytes, zero scanned bytes and %d entries", counters, tc.readBytes, tc.entries)
			}
		})
	}
}

func TestSanitizeStderrRedactsOnlyNontrivialAbsolutePaths(t *testing.T) {
	tests := []struct {
		name       string
		stderr     string
		root       string
		binaryPath string
		want       string
	}{
		{"relative binary", "file.txt. remains intact.", ".", "flashgate-mcp.exe", "file.txt. remains intact."},
		{"dot binary", "extension .txt and sentence.", ".", ".", "extension .txt and sentence."},
		{"windows case", `failed at c:\users\alice\corpus\file.txt`, `C:\Users\Alice\Corpus`, `C:\Build\flashgate.exe`, `failed at [REDACTED_PATH]\file.txt`},
		{"windows separators", `failed at c:/users/alice/corpus/file.txt`, `C:\Users\Alice\Corpus`, `C:\Build\flashgate.exe`, `failed at [REDACTED_PATH]/file.txt`},
		{"linux", "failed at /home/alice/corpus/file.txt", "/home/alice/corpus", "/opt/flashgate/server", "failed at [REDACTED_PATH]/file.txt"},
		{"macOS", "failed at /Users/alice/corpus/file.txt", "/Users/alice/corpus", "/Applications/FlashGate/server", "failed at [REDACTED_PATH]/file.txt"},
		{"normal filename", "cannot open file.txt.", ".", "server", "cannot open file.txt."},
		{"multiple paths", `C:\Corpus\a.txt failed via C:/Build/server.exe`, `C:\Corpus`, `C:\Build\server.exe`, `[REDACTED_PATH]\a.txt failed via [REDACTED_PATH]/server.exe`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeStderr(tc.stderr, tc.root, tc.binaryPath)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("sanitize=%q, want %q", got, tc.want)
			}
		})
	}
	if paths := validRedactionPaths("", ".", "/", `C:\`); len(paths) != 0 {
		t.Fatalf("trivial redaction paths accepted: %d", len(paths))
	}
}

func TestUnsupportedSnapshotNamesEveryOmittedMetric(t *testing.T) {
	snapshot := unsupportedSnapshot()
	if snapshot.status != "not_supported" {
		t.Fatalf("status=%q, want not_supported", snapshot.status)
	}
	if snapshot.workingSetBytes != nil || snapshot.peakWorkingSetBytes != nil || snapshot.userCPUNS != nil || snapshot.systemCPUNS != nil {
		t.Fatal("unsupported snapshot must omit numeric metrics")
	}
	want := []string{
		"idle_working_set_bytes",
		"peak_working_set_bytes",
		"user_cpu_ns",
		"system_cpu_ns",
	}
	if strings.Join(snapshot.unsupported, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unsupported=%v, want %v", snapshot.unsupported, want)
	}
}
