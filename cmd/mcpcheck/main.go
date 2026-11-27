// Command mcpcheck runs the native MCP contracts and writes a focused report.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type requirementMap struct {
	UpstreamCommit string `json:"upstreamCommit"`
	Scenarios      []struct {
		Revision       string `json:"revision"`
		Scenario       string `json:"scenario"`
		Scoring        string `json:"scoring"`
		UpstreamReason string `json:"upstreamReason"`
		OfficialStatus string `json:"officialStatus"`
		Applicability  string `json:"applicability"`
		NativeCoverage string `json:"nativeCoverage"`
		NativeReason   string `json:"nativeReason"`
	} `json:"scenarios"`
}

func (mapping requirementMap) verifyNative(results map[string]string) error {
	for _, row := range mapping.Scenarios {
		if row.NativeCoverage != "" && results[row.NativeCoverage] != "PASS" {
			return fmt.Errorf("official mapping names unexecuted native test: %s", row.NativeCoverage)
		}
	}
	return nil
}

var requiredNativeFamilies = []string{
	"TestMCPConformanceSnapshots", "TestMCPConformanceSchema202012",
	"TestMCPConformanceCompletenessMutations", "TestMCPConformanceWireNegatives",
	"TestMCPConformanceLegacyNegotiation", "TestMCPConformanceCrossRevisionStateless",
	"TestMCPConformanceSTDIOProcess", "TestMCPConformanceInventory",
	"TestMCPConformancePingSTDIO", "TestMCPConformancePingRevisionIsolation",
	"TestMCPConformancePingRegistryIndependence",
	"TestMCPConformanceWireEfficiency", "TestMCPConformanceWireEfficiencyGateMutations",
	"TestStatelessCandidateWire", "TestStatelessMetadataAndProductionGate",
}

const nativeTestPattern = "Test(MCPConformance|Stateless|ProtocolRevision|Filesystem.*Wire|ToolsListWire)"
const serverPackage = "github.com/thomasweidner/flashgate-mcp/cmd/server"

var nativeSelection = regexp.MustCompile(nativeTestPattern)
var coverageRun = regexp.MustCompile("^validation-[0-9a-f]{12}$")

type coverageInput struct {
	path, digest, origin string
	size                 int64
}

func main() {
	report := flag.String("report", "", "new output Markdown path outside the source tree (required)")
	var coverage coverageInput
	flag.StringVar(&coverage.path, "coverage-evidence", "", "admitted CANDIDATE coverage test log")
	flag.StringVar(&coverage.digest, "coverage-sha256", "", "SHA-256 from the admitted producer gate")
	flag.StringVar(&coverage.origin, "coverage-origin", "", "original validation run of the producer gate")
	flag.Int64Var(&coverage.size, "coverage-bytes", 0, "length from the admitted producer gate")
	flag.Parse()
	var input *coverageInput
	flag.Visit(func(option *flag.Flag) {
		if strings.HasPrefix(option.Name, "coverage-") {
			input = &coverage
		}
	})
	if err := run(*report, input); err != nil {
		fmt.Fprintln(os.Stderr, "mcpcheck:", err)
		os.Exit(1)
	}
}

func sourceIdentity() (string, error) {
	command := exec.Command("git", "ls-files", "-z", "-t", "--cached", "--others", "--deleted", "--exclude-standard")
	raw, err := command.Output()
	if err != nil {
		return "", errors.New("Git source inventory unavailable")
	}
	paths, err := sourcePaths(raw)
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return "", fmt.Errorf("nonregular or unavailable source: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		digest := sha256.Sum256(data)
		fmt.Fprintf(hash, "%s\x00%x\n", path, digest)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sourcePaths(raw []byte) ([]string, error) {
	present, deleted := make(map[string]bool), make(map[string]bool)
	for record := range strings.SplitSeq(strings.TrimSuffix(string(raw), "\x00"), "\x00") {
		tag, path, ok := strings.Cut(record, " ")
		if !ok || path == "" {
			return nil, errors.New("invalid Git source inventory")
		}
		switch tag {
		case "H", "S", "?":
			present[path] = true
		case "R":
			deleted[path] = true
		default:
			return nil, fmt.Errorf("unexpected Git source inventory tag %q", tag)
		}
	}
	// Git lists an unstaged deletion both as cached and as removed. Hash the
	// candidate's actual source files, excluding those absent tracked paths.
	var paths []string
	for path := range present {
		if !deleted[path] {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// This is the reviewed subtest inventory for the active server test sources.
// Keep the names used by Go's JSON events, including normalized whitespace.
var nativeSubtests = map[string]string{
	"TestFilesystemCallToolWireErrorsRemainSafe": "gated_write_tool legacy_tool unknown_tool",
	"TestFilesystemCallToolWireSuccesses": `
		get_path_info_existing get_path_info_missing list_directory
		move_path_default_profile read_file_single_transmission`,
	"TestMCPConformanceCompletenessMutations": `
		activated-or-announced-revision duplicated-payload incomplete-matrix
		missing-applicable-error-fixture missing-contract missing-error-applicability
		missing-explicit-false missing-tool missing-variant-fixture
		new-tool-without-contract profile-drift snapshot-drift-annotations
		snapshot-drift-description snapshot-drift-inputSchema snapshot-drift-name
		snapshot-drift-outputSchema snapshot-drift-title unknown-capability`,
	"TestMCPConformancePingSTDIO": "read write",
	"TestMCPConformanceSchema202012": `
		bad-input-metaschema bad-output-metaschema catalog-drift
		draft-keywords-and-format-annotation file-reference
		instance-and-strict-decoder-negatives legacy-dialect missing-input
		missing-output nested-dialect nonobject-output open-input remote-reference unknown-dialect`,
	"TestMCPConformanceSnapshots": "2025-11-25/read 2025-11-25/write 2026-07-28/read 2026-07-28/write",
	"TestMCPConformanceWireEfficiencyGateMutations": `
		CRLF candidate_metadata_on_zero-revision_budget catalog_budgets
		catalog_budgets/duplicate_budget_key catalog_budgets/initialize catalog_budgets/tools/list
		changed_protocol_error_representation changed_tool_error_representation changed_variant_contract
		duplicated_payload extra_LF extra_heavy_copy extra_response_frame extra_result_metadata
		forged_same-length_content forged_size internal_whitespace invalid_budget leading_whitespace
		malformed_encoding missing_LF missing_budget missing_candidate_profile missing_error_wire_fixture
		missing_overwrite_fixture missing_semantic_fixture new_tool new_variant physical_newline_in_frame
		private_error_payload relocated_payload size_increase_with_identical_semantics stale_budget
		stale_error_budget trailing_whitespace unapproved_optional_composition`,
	"TestMCPConformanceWireNegatives":               "read-file-encoding",
	"TestProtocolRevisionWire":                      "server_discover_as_first_request supported_initialize unsupported_proposal_through_initialize",
	"TestToolsListWireOutputSchemasAndPayloadSizes": "read write",
}

const nativeSubtestSourceSHA256 = "95128bf616e1afa2e193071215cf6b277ba086ca59a1524916a4d229085bab1d"

// Go selects the active test files. Bind the reviewed subtest names to the
// files declaring those tests and the helper/revision inputs that name rows.
func nativeTestNames(ctx context.Context) (map[string]bool, error) {
	raw, err := exec.CommandContext(ctx, "go", "list", "-mod=vendor", "-json", "./cmd/server").Output()
	if err != nil {
		return nil, fmt.Errorf("native test inventory unavailable: %w", err)
	}
	var pkg struct {
		ImportPath, Dir           string
		TestGoFiles, XTestGoFiles []string
	}
	if err := json.Unmarshal(raw, &pkg); err != nil || pkg.ImportPath != serverPackage {
		return nil, errors.New("invalid native test package inventory")
	}
	names := make(map[string]bool)
	bound := make(map[string][sha256.Size]byte)
	record := func(path string, data []byte) {
		bound[filepath.ToSlash(path)] = sha256.Sum256(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")))
	}
	for _, name := range append(pkg.TestGoFiles, pkg.XTestGoFiles...) {
		if filepath.Base(name) != name {
			return nil, errors.New("invalid native test source name")
		}
		path := filepath.Join("cmd", "server", name)
		data, err := os.ReadFile(filepath.Join(pkg.Dir, name))
		if err != nil {
			return nil, err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, data, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			name := fn.Name.Name
			r, _ := utf8.DecodeRuneInString(strings.TrimPrefix(name, "Test"))
			if !unicode.IsLower(r) && nativeSelection.MatchString(name) {
				names[name] = true
				record(path, data)
			}
		}
	}
	if len(names) == 0 {
		return nil, errors.New("no selected native tests in active source")
	}
	// Catalog budget child names come from testCatalogBudgetMutations and
	// serverWireCases. The snapshot names also depend on the revision constants.
	for _, path := range []string{"cmd/server/catalog_budget_test.go", "internal/protocol/mcp.go", "internal/mcp/revision/revision.go"} {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		record(path, data)
	}
	if err := verifyNativeInventorySource(bound, nativeSubtestSourceSHA256); err != nil {
		return nil, err
	}
	for parent, children := range nativeSubtests {
		if !names[parent] {
			return nil, fmt.Errorf("subtest inventory parent is outside the active selection: %s", parent)
		}
		for _, child := range strings.Fields(children) {
			name := parent + "/" + child
			if names[name] {
				return nil, fmt.Errorf("duplicate subtest inventory identity: %s", name)
			}
			names[name] = true
		}
	}
	return names, nil
}

func verifyNativeInventorySource(files map[string][sha256.Size]byte, expected string) error {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		fmt.Fprintf(hash, "%s\x00%x\n", path, files[path])
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); actual != expected {
		return fmt.Errorf("reviewed native subtest inventory does not match active test sources (SHA-256 %s)", actual)
	}
	return nil
}

type testEvent struct {
	Action, Test, Package, Time, Output, ImportPath, FailedBuild string
}

func decodeTestEvent(line []byte) (testEvent, error) {
	var event testEvent
	decoder := json.NewDecoder(bytes.NewReader(line))
	seen := make(map[string]bool)
	open, err := decoder.Token()
	if err != nil || open != json.Delim('{') {
		return event, errors.New("invalid go test JSON evidence")
	}
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok || seen[strings.ToLower(name)] {
			return event, errors.New("invalid or duplicate go test event field")
		}
		seen[strings.ToLower(name)] = true
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return event, errors.New("invalid go test event value")
		}
	}
	if err := json.Unmarshal(line, &event); err != nil {
		return event, errors.New("invalid go test JSON evidence")
	}
	return event, nil
}

// Both inputs share the native contract evaluator. A broad producer adds an
// independently discovered selection and keeps only the original MCP events.
func testResults(raw []byte, expected map[string]bool) (map[string]string, []byte, error) {
	if !bytes.HasSuffix(raw, []byte("\n")) || !utf8.Valid(raw) {
		return nil, nil, errors.New("incomplete or invalid UTF-8 go test JSON evidence")
	}
	results := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	started, packagePass := false, false
	states := make(map[string]string)
	parents := make(map[string]string)
	activeChildren := make(map[string]int)
	var projection bytes.Buffer
	for scanner.Scan() {
		line := scanner.Bytes()
		event, err := decodeTestEvent(line)
		if err != nil {
			return nil, nil, err
		}
		switch event.Action {
		case "fail", "build-fail":
			return nil, nil, errors.New("native evidence contains a failure")
		case "build-output":
			if event.ImportPath == "" {
				return nil, nil, errors.New("build output has no import path")
			}
			continue
		case "start", "run", "pause", "cont", "pass", "output", "skip", "bench":
		default:
			return nil, nil, errors.New("unknown go test event action")
		}
		if event.Package == "" || event.FailedBuild != "" {
			return nil, nil, errors.New("incomplete go test package event")
		}
		if event.Package != serverPackage {
			if expected == nil {
				return nil, nil, errors.New("unexpected package in focused native evidence")
			}
			continue
		}
		top, _, _ := strings.Cut(event.Test, "/")
		if event.Test != "" && !nativeSelection.MatchString(top) {
			if expected == nil {
				return nil, nil, errors.New("unexpected test in focused native evidence")
			}
			continue
		}
		if event.Time == "" || event.Action == "skip" || packagePass ||
			(event.Action == "output" && strings.HasSuffix(strings.TrimSpace(event.Output), "(cached)") && event.Test == "") {
			return nil, nil, errors.New("cached, skipped or late native evidence")
		}
		if event.Test == "" && event.Action == "start" && !started {
			started = true
		} else if !started {
			return nil, nil, errors.New("missing native package start")
		} else if event.Test == "" {
			switch event.Action {
			case "output":
			case "pass":
				if len(states) != len(results) {
					return nil, nil, errors.New("native package completed with unfinished tests")
				}
				packagePass = true
			default:
				return nil, nil, errors.New("invalid native package event sequence")
			}
		} else {
			parent := parents[event.Test]
			subtest := top != event.Test
			current := states[event.Test]
			switch {
			case event.Action == "output":
			case event.Action == "run" && current == "":
				if subtest {
					// One t.Run name can itself contain slashes.
					parent = event.Test
					for {
						separator := strings.LastIndexByte(parent, '/')
						if separator < 0 {
							return nil, nil, fmt.Errorf("missing active native test parent for %s", event.Test)
						}
						parent = parent[:separator]
						if state := states[parent]; state == "run" || state == "pause" {
							break
						}
					}
					parents[event.Test] = parent
					activeChildren[parent]++
				}
				states[event.Test] = "run"
			case event.Action == "pause" && current == "run":
				states[event.Test] = "pause"
			case event.Action == "cont" && current == "pause":
				states[event.Test] = "run"
			case event.Action == "pass" && current == "run" && activeChildren[event.Test] == 0:
				states[event.Test] = "pass"
				results[event.Test] = "PASS"
				if subtest {
					activeChildren[parent]--
				}
			default:
				return nil, nil, fmt.Errorf("invalid native event sequence for %s", event.Test)
			}
		}
		if expected != nil {
			projection.Write(line)
			projection.WriteByte('\n')
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	if !packagePass {
		return nil, nil, errors.New("missing package completion evidence")
	}
	for _, name := range requiredNativeFamilies {
		if results[name] != "PASS" {
			return nil, nil, fmt.Errorf("missing native contract evidence: %s", name)
		}
	}
	for name := range expected {
		if results[name] != "PASS" {
			return nil, nil, fmt.Errorf("missing selected native test: %s", name)
		}
	}
	if expected != nil {
		for name := range results {
			if !expected[name] {
				return nil, nil, fmt.Errorf("unexpected selected native test outside the reviewed inventory: %s", name)
			}
		}
	}
	return results, projection.Bytes(), nil
}

func (input coverageInput) read(run string) ([]byte, error) {
	expected, err := hex.DecodeString(input.digest)
	if err != nil || len(expected) != sha256.Size || input.size <= 0 || input.size > 64*1024*1024 ||
		!filepath.IsAbs(input.path) || !coverageRun.MatchString(input.origin) {
		return nil, errors.New("complete admitted coverage input binding is required")
	}
	relative, err := filepath.Rel(run, input.path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(run)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	before, err := root.Lstat(relative)
	if err != nil || !before.Mode().IsRegular() || before.Size() != input.size {
		return nil, errors.New("coverage evidence is not the bound regular file")
	}
	file, err := root.Open(relative)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !os.SameFile(before, info) {
		return nil, errors.New("coverage evidence changed while opening")
	}
	raw, err := io.ReadAll(io.LimitReader(file, input.size+1))
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	if int64(len(raw)) != input.size || !bytes.Equal(digest[:], expected) {
		return nil, errors.New("consumed coverage evidence differs from its admission")
	}
	return raw, nil
}

func run(reportPath string, coverage *coverageInput) error {
	if reportPath == "" {
		return errors.New("-report is required")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(reportPath)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return errors.New("report must be outside the source tree")
	}
	if _, err := os.Lstat(absolute); !os.IsNotExist(err) {
		return errors.New("report path must be new")
	}
	logPath := absolute + ".tests.jsonl"
	if _, err := os.Lstat(logPath); !os.IsNotExist(err) {
		return errors.New("test log path must be new")
	}
	before, err := sourceIdentity()
	if err != nil {
		return err
	}
	head, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return errors.New("source HEAD unavailable")
	}
	var requirements requirementMap
	data, err := os.ReadFile("cmd/server/testdata/mcp/official-requirements.json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &requirements); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	var raw []byte
	var expected map[string]bool
	if coverage != nil {
		run := filepath.Dir(absolute)
		work := os.Getenv("FLASHGATE_WORK_ROOT")
		if !filepath.IsAbs(work) || filepath.Dir(run) != filepath.Clean(work) ||
			!coverageRun.MatchString(filepath.Base(run)) ||
			coverage.path != filepath.Join(run, "coverage", runtime.GOOS, "test.log") {
			return errors.New("coverage input must be the bound CANDIDATE run artifact")
		}
		raw, err = coverage.read(run)
		if err != nil {
			return err
		}
		expected, err = nativeTestNames(ctx)
		if err != nil {
			return err
		}
	} else {
		command := exec.CommandContext(ctx, "go", "test", "-mod=vendor", "-count=1", "-json", "./cmd/server", "-run", nativeTestPattern, "-skip=^$")
		command.WaitDelay = time.Second
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		runErr := command.Run()
		raw = stdout.Bytes()
		if err := writeNew(logPath, raw); err != nil {
			return err
		}
		if runErr != nil {
			return errors.New("native tests failed; see the test log (no PASS report written)")
		}
	}
	results, projection, err := testResults(raw, expected)
	if err != nil {
		return err
	}
	if err := requirements.verifyNative(results); err != nil {
		return err
	}
	if expected != nil {
		if err := writeNew(logPath, projection); err != nil {
			return err
		}
	}
	after, err := sourceIdentity()
	if err != nil {
		return err
	}
	if before != after {
		return errors.New("source changed during conformance validation")
	}
	var report strings.Builder
	fmt.Fprintf(&report, "# MCP conformance validation\n\nNative contracts: PASS. Official suite: NOT_RUN. Full MCP conformance is not claimed.\n\nSource HEAD: `%s`\n\nSource inventory SHA-256: `%s` (Git tracked and nonignored new source bytes).\n\nUpstream conformance commit: `%s`. Requirements are pinned per revision.\n", strings.TrimSpace(string(head)), before, requirements.UpstreamCommit)
	fmt.Fprintln(&report, "\n## Native evidence")
	if expected != nil {
		fmt.Fprintf(&report, "\nCoverage origin: `%s`. Original Go test JSONL: %d bytes, SHA-256 `%s`.\n\nThe adjacent `.tests.jsonl` file contains the original selected events. This invocation checked admitted Coverage evidence without executing the focused Go test process.\n", coverage.origin, coverage.size, coverage.digest)
	} else {
		fmt.Fprintf(&report, "\nCommand: `go test -mod=vendor -count=1 -json ./cmd/server -run '%s' -skip=^$`.\n\nThe adjacent `.tests.jsonl` file is the actual Go test evidence.\n", nativeTestPattern)
	}
	fmt.Fprintln(&report, "\nPASS requires every mandatory family and snapshot matrix row to pass, with no failed or skipped contracts.")
	names := make([]string, 0, len(results))
	for name := range results {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&report, "\n- PASS `%s`", name)
	}
	fmt.Fprintln(&report, "\n\n## Snapshot and drift evidence\n\nNormal tests compare exact canonical bytes and never update snapshots. Explicit updates require review. Inventory, schema, annotations, profile, revision and capability mutations are covered by the named completeness tests.")
	paths, err := filepath.Glob("cmd/server/testdata/mcp/*.json")
	if err != nil {
		return err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(&report, "\n- `%s`: `%x`", filepath.ToSlash(path), sha256.Sum256(data))
		if filepath.Base(path) != "official-requirements.json" {
			evidence, err := snapshotEvidence(path, data, results)
			if err != nil {
				return err
			}
			fmt.Fprint(&report, evidence)
		}
	}
	fmt.Fprintln(&report, "\n\n## Official server requirements\n\nOfficial execution is NOT_RUN: the runner uses HTTP and named reference-fixture behaviors. No installation or HTTP bridge was used. NOT_APPLICABLE below describes absent transports/capabilities, not a suite PASS. Native coverage names indicate partial equivalent contracts, never execution of an official scenario. Client-leg scenarios are NOT_APPLICABLE to this server.\n\n| Revision | Scenario | Scoring | Applicability | Official | Native coverage / reason |\n|---|---|---|---|---|---|")
	for _, row := range requirements.Scenarios {
		coverage := row.NativeReason
		if row.NativeCoverage != "" {
			coverage = "PASS `" + row.NativeCoverage + "` (subset). " + coverage
		}
		fmt.Fprintf(&report, "| %s | %s | %s %s | %s | %s | %s |\n", row.Revision, row.Scenario, row.Scoring, row.UpstreamReason, row.Applicability, row.OfficialStatus, coverage)
	}
	fmt.Fprintln(&report, "\n## Limitations\n\nThe official suite was not executed. See the requirement mapping above for applicability and known gaps. The native run includes offline Draft 2020-12 tool schema and instance checks. Repository-wide, platform, coverage, release, and independent review gates remain separate.")
	return writeNew(absolute, []byte(report.String()))
}

func snapshotEvidence(path string, data []byte, results map[string]string) (string, error) {
	var snapshot struct {
		Revision    string `json:"revision"`
		Profile     string `json:"profile"`
		Definitions []struct {
			Name string `json:"name"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if snapshot.Revision == "" || snapshot.Profile == "" ||
		filepath.Base(path) != snapshot.Revision+"-"+snapshot.Profile+".json" {
		return "", fmt.Errorf("snapshot identity differs from matrix filename: %s", path)
	}
	name := "TestMCPConformanceSnapshots/" + snapshot.Revision + "/" + snapshot.Profile
	if results[name] != "PASS" {
		return "", fmt.Errorf("missing native contract evidence: %s", name)
	}
	toolNames := make([]string, 0, len(snapshot.Definitions))
	for _, definition := range snapshot.Definitions {
		toolNames = append(toolNames, definition.Name)
	}
	return fmt.Sprintf("; %s / %s: %d tools (%s)", snapshot.Revision, snapshot.Profile, len(toolNames), strings.Join(toolNames, ", ")), nil
}

func writeNew(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := io.Copy(file, bytes.NewReader(data))
	return errors.Join(writeErr, file.Close())
}
