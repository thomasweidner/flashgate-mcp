package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const mcpGateStep = `      - name: Check MCP conformance and snapshots
        shell: bash
        run: go run -mod=vendor ./cmd/mcpcheck -report "$RUNNER_TEMP/mcp-conformance.md"`

func validateMCPWorkflow(contents string) error {
	contents = strings.ReplaceAll(contents, "\r\n", "\n")
	const triggers = "\non:\n  push:\n    branches:\n      - main\n  pull_request:\n    branches:\n      - main\n"
	if !strings.Contains(contents, triggers) || !strings.Contains(contents, "\npermissions:\n  contents: read\n") ||
		strings.Contains(contents, "-update-mcp-snapshots") {
		return errors.New("MCP CI trigger, permissions, or compare-only contract changed")
	}
	_, jobs, _ := strings.Cut(contents, "\njobs:\n")
	_, testJob, _ := strings.Cut(jobs, "\n  test:\n")
	testJob = "\n" + testJob
	nextJob := regexp.MustCompile(`(?m)^  [a-zA-Z0-9_-]+:`).FindStringIndex(testJob)
	if nextJob != nil {
		testJob = testJob[:nextJob[0]]
	}
	if !strings.Contains(testJob, "    runs-on: ${{ matrix.os }}\n") ||
		!strings.Contains(testJob, "          - os: windows-latest\n            platform: windows\n") ||
		!strings.Contains(testJob, "          - os: ubuntu-latest\n            platform: linux\n") ||
		strings.Contains(testJob, "\n    continue-on-error:") || strings.Contains(testJob, "\n    if:") {
		return errors.New("MCP gate requires both blocking test matrix jobs")
	}
	const stepPrefix = "\n      - name: "
	const gateName = "Check MCP conformance and snapshots\n"
	_, afterGate, _ := strings.Cut(testJob, stepPrefix+gateName)
	body, _, _ := strings.Cut(afterGate, stepPrefix)
	if strings.Count(contents, stepPrefix+gateName) != 1 ||
		strings.TrimRight("      - name: "+gateName+body, "\n") != mcpGateStep {
		return errors.New("MCP gate must run the unmodified native runner unconditionally in both matrix jobs")
	}
	return nil
}

func TestMCPWorkflowPolicy(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	contents := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if err := validateMCPWorkflow(contents); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct{ name, before, after string }{
		{"removed", mcpGateStep, ""},
		{"conditional", mcpGateStep, mcpGateStep + "\n        if: matrix.os == 'windows-latest'"},
		{"nonblocking step", mcpGateStep, mcpGateStep + "\n        continue-on-error: true"},
		{"nonblocking job", "\n  test:\n", "\n  test:\n    continue-on-error: true\n"},
		{"conditional job", "\n  test:\n", "\n  test:\n    if: false\n"},
		{"shell masks failure", mcpGateStep, mcpGateStep + " || true"},
		{"updates snapshots", mcpGateStep, mcpGateStep + " -update-mcp-snapshots"},
		{"windows removed", "- os: windows-latest", "- os: macos-latest"},
		{"linux removed", "- os: ubuntu-latest", "- os: macos-latest"},
		{"pull request removed", "  pull_request:", "  workflow_dispatch:"},
		{"write permission", "  contents: read", "  contents: write"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			mutated := strings.Replace(contents, fixture.before, fixture.after, 1)
			if mutated == contents || validateMCPWorkflow(mutated) == nil {
				t.Fatal("weakened workflow accepted or mutation not applied")
			}
		})
	}
}
