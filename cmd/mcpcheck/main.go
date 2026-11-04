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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
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

func main() {
	report := flag.String("report", "", "new output Markdown path outside the source tree (required)")
	flag.Parse()
	if err := run(*report); err != nil {
		fmt.Fprintln(os.Stderr, "mcpcheck:", err)
		os.Exit(1)
	}
}

func sourceIdentity() (string, error) {
	command := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	raw, err := command.Output()
	if err != nil {
		return "", errors.New("Git source inventory unavailable")
	}
	paths := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
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

// A skipped or failed contract cannot be rendered as a successful native run.
func testResults(raw []byte) (map[string]string, error) {
	results := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	packagePass := false
	for scanner.Scan() {
		var event struct{ Action, Test, Package string }
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, errors.New("invalid go test JSON evidence")
		}
		if event.Action == "fail" || event.Action == "skip" {
			return nil, errors.New("native contract failed or skipped; inspect test evidence")
		}
		if event.Action == "pass" {
			if event.Test == "" {
				packagePass = true
			} else {
				results[event.Test] = "PASS"
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !packagePass {
		return nil, errors.New("missing package completion evidence")
	}
	for _, name := range []string{"TestMCPConformanceSnapshots", "TestMCPConformanceCompletenessMutations", "TestMCPConformanceWireNegatives", "TestMCPConformanceLegacyNegotiation", "TestMCPConformanceCrossRevisionStateless", "TestMCPConformanceSTDIOProcess", "TestMCPConformanceInventory", "TestStatelessCandidateWire", "TestStatelessMetadataAndProductionGate"} {
		if results[name] != "PASS" {
			return nil, fmt.Errorf("missing native contract evidence: %s", name)
		}
	}
	return results, nil
}

func run(reportPath string) error {
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
	command := exec.CommandContext(ctx, "go", "test", "-mod=vendor", "-count=1", "-json", "./cmd/server", "-run", "Test(MCPConformance|Stateless|ProtocolRevision|Filesystem.*Wire|ToolsListWire)")
	command.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := command.Run()
	if err := writeNew(logPath, stdout.Bytes()); err != nil {
		return err
	}
	if runErr != nil {
		return errors.New("native tests failed; see the test log (no PASS report written)")
	}
	results, err := testResults(stdout.Bytes())
	if err != nil {
		return err
	}
	after, err := sourceIdentity()
	if err != nil {
		return err
	}
	if before != after {
		return errors.New("source changed during conformance validation")
	}
	var report strings.Builder
	fmt.Fprintf(&report, "# MCP conformance validation\n\nNative contracts: PASS. Official suite: NOT_RUN. Full MCP conformance is not claimed.\n\nSource HEAD: `%s`\n\nSource inventory SHA-256: `%s` (Git tracked and nonignored new source bytes).\n\nUpstream conformance commit: `%s`. Requirements are pinned per revision; 2025-11-25 is a reconstruction, 2026-07-28 is the frozen release set.\n\n", strings.TrimSpace(string(head)), before, requirements.UpstreamCommit)
	fmt.Fprintln(&report, "Production: 2025-11-25 only. Candidate: 2026-07-28 under injected test policy only. Profiles: default and read-only. Capabilities: tools only; no extensions, resources, prompts or change stream. Exact tool sets and counts are derived from the validated snapshots below.")
	fmt.Fprintln(&report, "\n## Native evidence\n\nCommand: `go test -mod=vendor -count=1 -json ./cmd/server -run 'Test(MCPConformance|Stateless|ProtocolRevision|Filesystem.*Wire|ToolsListWire)'`.\n\nThe adjacent `.tests.jsonl` file is the actual Go test evidence. PASS requires no failed/skipped tests and all mandatory contract families. This report does not establish the repository-wide, platform, coverage or independent review gates.")
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
			var snapshot struct {
				Revision    string `json:"revision"`
				Profile     string `json:"profile"`
				Definitions []struct {
					Name string `json:"name"`
				} `json:"definitions"`
			}
			if err := json.Unmarshal(data, &snapshot); err != nil {
				return err
			}
			toolNames := make([]string, 0, len(snapshot.Definitions))
			for _, definition := range snapshot.Definitions {
				toolNames = append(toolNames, definition.Name)
			}
			fmt.Fprintf(&report, "; %s / %s: %d tools (%s)", snapshot.Revision, snapshot.Profile, len(toolNames), strings.Join(toolNames, ", "))
		}
	}
	fmt.Fprintln(&report, "\n\n## Official server requirements\n\nOfficial execution is NOT_RUN: the runner uses HTTP and named reference-fixture behaviors. No installation or HTTP bridge was used. NOT_APPLICABLE below describes absent transports/capabilities, not a suite PASS. Native coverage names indicate partial equivalent contracts, never execution of an official scenario. Client-leg scenarios are NOT_APPLICABLE to this server.\n\n| Revision | Scenario | Scoring | Applicability | Official | Native coverage / reason |\n|---|---|---|---|---|---|")
	for _, row := range requirements.Scenarios {
		coverage := row.NativeReason
		if row.NativeCoverage != "" {
			if results[row.NativeCoverage] != "PASS" {
				return fmt.Errorf("official mapping names unexecuted native test: %s", row.NativeCoverage)
			}
			coverage = "PASS `" + row.NativeCoverage + "` (subset). " + coverage
		}
		fmt.Fprintf(&report, "| %s | %s | %s %s | %s | %s | %s |\n", row.Revision, row.Scenario, row.Scoring, row.UpstreamReason, row.Applicability, row.OfficialStatus, coverage)
	}
	fmt.Fprintln(&report, "\n## Limitations\n\nThe official suite was not executed. Legacy ping is not registered; that applicable existing gap prevents any full-conformance claim. Full JSON Schema 2020-12 validation remains BL-212; fingerprints/cache semantics remain BL-219; dedicated CI drift enforcement remains BL-257. Candidate production activation still requires all its owner gates. No release approval or independent terminal review is implied.")
	return writeNew(absolute, []byte(report.String()))
}

func writeNew(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := io.Copy(file, bytes.NewReader(data))
	return errors.Join(writeErr, file.Close())
}
