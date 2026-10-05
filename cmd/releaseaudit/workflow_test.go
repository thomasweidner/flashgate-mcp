package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var permissionLine = regexp.MustCompile(`^[ \t]*permissions[ \t]*:`)

func validateReleasePermissions(contents string) error {
	blockCount := 0
	entryCount := 0
	inBlock := false
	for _, line := range strings.Split(contents, "\n") {
		if permissionLine.MatchString(line) {
			if line != "permissions:" || blockCount != 0 {
				return errors.New("release workflow permissions must be top-level and unique")
			}
			blockCount++
			inBlock = true
			continue
		}
		if !inBlock || strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if line != "  contents: read" || entryCount != 0 {
				return errors.New("release workflow has an unexpected permission entry")
			}
			entryCount++
			continue
		}
		inBlock = false
	}
	if blockCount != 1 || entryCount != 1 {
		return errors.New("release workflow requires exactly contents: read")
	}
	return nil
}

func validateReleaseWorkflow(contents string) error {
	required := []string{
		"  push:\n    tags:\n      - 'v*.*.*'",
		"  workflow_dispatch:",
		"permissions:\n  contents: read\n",
		"flashgate_read_repository_version",
		"go -C . run -mod=vendor ./cmd/releaseaudit source",
		"git tag --points-at HEAD",
		`[[ "$GITHUB_REF_NAME" == "$expected_tag" ]]`,
		"status --porcelain=v1 --untracked-files=all",
	}
	for _, item := range required {
		if !strings.Contains(contents, item) {
			return errors.New("missing release workflow gate: " + item)
		}
	}
	if err := validateReleasePermissions(contents); err != nil {
		return err
	}
	if strings.Contains(contents, "inputs:\n      version:") ||
		strings.Contains(contents, "GITHUB_REF_NAME#v") ||
		strings.Contains(contents, "REQUESTED_VERSION") {
		return errors.New("release workflow has competing version or permission authority")
	}
	return nil
}

func TestReleaseWorkflowPolicy(t *testing.T) {
	name := filepath.Join("..", "..", ".github", "workflows", "release-build.yml")
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	contents := strings.ReplaceAll(string(data), "\r\n", "\n")
	if err := validateReleaseWorkflow(contents); err != nil {
		t.Fatal(err)
	}
	mutations := []struct{ name, old, replacement string }{
		{"top contents write", "  contents: read", "  contents: write"},
		{"top issues write", "  contents: read", "  contents: read\n  issues: write"},
		{"top pull requests write", "  contents: read", "  contents: read\n  pull-requests: write"},
		{"top actions write", "  contents: read", "  contents: read\n  actions: write"},
		{"top id token write", "  contents: read", "  contents: read\n  id-token: write"},
		{"top write all", "permissions:\n  contents: read", "permissions: write-all"},
		{"job contents write", "    runs-on: ubuntu-latest", "    permissions:\n      contents: write\n    runs-on: ubuntu-latest"},
		{"job issues write", "    runs-on: ubuntu-latest", "    permissions:\n      issues: write\n    runs-on: ubuntu-latest"},
		{"second permissions block", "jobs:\n", "permissions:\n  contents: read\n\njobs:\n"},
		{"free version", "  workflow_dispatch:\n", "  workflow_dispatch:\n    inputs:\n      version:\n"},
		{"tag version", "flashgate_read_repository_version", "GITHUB_REF_NAME#v"},
		{"source gate", "go -C . run -mod=vendor ./cmd/releaseaudit source", "echo no-source-gate"},
		{"tag gate", "git tag --points-at HEAD", "echo no-tag-gate"},
		{"clean tree gate", "git status --porcelain=v1 --untracked-files=all", "echo no-clean-tree-gate"},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed := strings.Replace(contents, mutation.old, mutation.replacement, 1)
			if changed == contents {
				t.Fatal("mutation did not change workflow")
			}
			if err := validateReleaseWorkflow(changed); err == nil {
				t.Fatal("unsafe workflow mutation passed")
			}
		})
	}
}

func validateCandidateWorkflow(contents string) error {
	required := []string{
		"  pull_request:\n    branches:\n      - main",
		"permissions:\n  contents: read\n",
		"  workflow_dispatch:\n  pull_request:",
		"ref: ${{ github.event_name == 'pull_request' && github.event.pull_request.head.sha || github.sha }}",
		"CANDIDATE_COMMIT: ${{ github.event_name == 'pull_request' && github.event.pull_request.head.sha || github.sha }}",
		"[[ \"$(git rev-parse HEAD)\" == \"$CANDIDATE_COMMIT\" ]]",
		"if [[ \"$CANDIDATE_EVENT\" == 'workflow_dispatch' ]]; then\n            echo 'run_candidate=true' >> \"$GITHUB_OUTPUT\"\n            echo 'Explicit current-version reverification at the dispatch commit.' >> \"$GITHUB_STEP_SUMMARY\"\n            exit 0\n          fi\n          [[ \"$CANDIDATE_EVENT\" == 'pull_request' ]]",
		"git diff --quiet \"$base\" HEAD -- VERSION",
		"github.event.pull_request.base.ref",
		"github.event.pull_request.base.sha",
		"git rev-parse \"$base^{commit}\"",
		"if git diff --quiet \"$base\" HEAD -- VERSION; then\n            base_workflow=\"$(git ls-tree --full-tree --name-only \"$base\" -- .github/workflows/current-version-candidate.yml)\"\n            if [[ -n \"$base_workflow\" ]]; then\n              [[ \"$base_workflow\" == '.github/workflows/current-version-candidate.yml' ]]\n              echo 'run_candidate=false' >> \"$GITHUB_OUTPUT\"\n              echo 'VERSION unchanged and candidate workflow exists in exact base; matrix skipped.' >> \"$GITHUB_STEP_SUMMARY\"\n              exit 0\n            fi\n            echo 'Candidate workflow absent from exact base; bootstrap matrix required.' >> \"$GITHUB_STEP_SUMMARY\"",
		"run_candidate=true",
		"status=$?\n            [[ \"$status\" -eq 1 ]] || exit \"$status\"\n          fi\n          echo 'run_candidate=true'",
		"if: needs.prepare.outputs.run_candidate == 'true'",
		"flashgate_read_repository_version",
		"./cmd/releaseaudit source",
		"--candidate --report",
		"source_commit=$(git rev-parse HEAD)",
		"ref: ${{ needs.prepare.outputs.source_commit }}",
		"$(git rev-parse HEAD)\" == \"${{ needs.prepare.outputs.source_commit }}",
		"SOURCE_DATE_EPOCH: ${{ needs.prepare.outputs.source_epoch }}",
		"ExpectedModified false",
		"--expected-modified false",
		"./scripts/New-ReleaseArtifact.ps1",
		"scripts/new-release-artifact.sh",
		"./scripts/Test-ReleaseArtifact.ps1",
		"scripts/test-release-artifact.sh",
		"./cmd/releaseaudit compare",
		"            'scan',",
		"            scan",
		"uses: actions/upload-artifact@v7",
		"./cmd/releaseaudit candidate-record",
		"./cmd/releaseaudit candidate-manifest",
		"uses: actions/download-artifact@v7",
		"merge-multiple: true",
		"SourceCommit: ${{ needs.prepare.outputs.source_commit }}",
		"retention-days: 14",
	}
	for _, item := range required {
		if !strings.Contains(contents, item) {
			return errors.New("missing candidate gate: " + item)
		}
	}
	if strings.Count(contents, "./cmd/releaseaudit compare") != 2 {
		return errors.New("candidate requires both platform reproducibility gates")
	}
	if strings.Count(contents, "SourceCommit: %s") != 1 || strings.Count(contents, "SourceCommit: ${{ needs.prepare.outputs.source_commit }}") != 1 {
		return errors.New("candidate verifier evidence must bind the source commit on both platforms")
	}
	if strings.Count(contents, "if: needs.prepare.outputs.run_candidate == 'true'") != 2 {
		return errors.New("candidate build and manifest require the VERSION-change gate")
	}
	if err := validateReleasePermissions(contents); err != nil {
		return err
	}
	for _, forbidden := range []string{
		"inputs:", "GITHUB_REF_NAME#v", "--release", "            -Release", "git tag", "gh release", "softprops/action-gh-release", "contents: write",
	} {
		if strings.Contains(contents, forbidden) {
			return errors.New("candidate workflow contains forbidden release/version behavior: " + forbidden)
		}
	}
	for _, target := range []string{
		"platform: windows\n            goarch: amd64\n            public_arch: x64",
		"platform: windows\n            goarch: arm64\n            public_arch: arm64",
		"platform: linux\n            goarch: amd64\n            public_arch: x64",
		"platform: linux\n            goarch: arm64\n            public_arch: arm64",
	} {
		if !strings.Contains(contents, target) {
			return errors.New("missing candidate matrix target: " + target)
		}
	}
	return nil
}

func TestCandidateEventGateExecution(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("actual Bash gate execution is validated by the native Linux gate")
	}
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "current-version-candidate.yml"))
	if err != nil {
		t.Fatal(err)
	}
	contents := strings.ReplaceAll(string(data), "\r\n", "\n")
	start := strings.Index(contents, "      - name: Decide candidate gate\n")
	if start < 0 {
		t.Fatal("candidate gate missing")
	}
	block := strings.SplitN(contents[start:], "        run: |\n", 2)
	if len(block) != 2 {
		t.Fatal("gate script missing")
	}
	var lines []string
	for _, line := range strings.Split(block[1], "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "          ") {
			break
		}
		lines = append(lines, strings.TrimPrefix(line, "          "))
	}
	// Mock only Git's read-only observations. Execute the real workflow script,
	// so dispatch independence from VERSION comparison and PR branches are tested.
	gitFixture := `git() {
  case "$1" in
    rev-parse) if [[ "$2" == HEAD ]]; then printf '%s\n' "$OBSERVED_HEAD"; else printf '%s\n' "$PR_BASE_SHA"; fi ;;
    status) [[ "$DIRTY" == false ]] || printf ' M VERSION\n' ;;
    diff) return "$DIFF_STATUS" ;;
    ls-tree)
      [[ "$2" == --full-tree && "$3" == --name-only && "$4" == "$PR_BASE_SHA" && "$5" == -- && "$6" == .github/workflows/current-version-candidate.yml ]] || return 99
      if [[ "$WORKFLOW_STATUS" != 0 ]]; then return "$WORKFLOW_STATUS"; fi
      [[ -z "$BASE_WORKFLOW" ]] || printf '%s\n' "$BASE_WORKFLOW"
      return 0 ;;
    *) return 99 ;;
  esac
}
`
	for _, tc := range []struct {
		name, event, diff, head, base, dirty, workflow, workflowStatus, want string
		fail                                                                 bool
	}{
		{"dispatch unchanged", "workflow_dispatch", "0", testCandidateCommit, "", "false", "", "128", "run_candidate=true\n", false},
		{"dispatch comparison unavailable", "workflow_dispatch", "2", testCandidateCommit, "", "false", "", "128", "run_candidate=true\n", false},
		{"PR unchanged workflow present", "pull_request", "0", testCandidateCommit, testCandidateCommit, "false", ".github/workflows/current-version-candidate.yml", "0", "run_candidate=false\n", false},
		{"PR unchanged bootstrap", "pull_request", "0", testCandidateCommit, testCandidateCommit, "false", "", "0", "run_candidate=true\n", false},
		{"PR workflow query error", "pull_request", "0", testCandidateCommit, testCandidateCommit, "false", "", "128", "", true},
		{"PR unexpected workflow query result", "pull_request", "0", testCandidateCommit, testCandidateCommit, "false", "unexpected/path", "0", "", true},
		{"PR changed", "pull_request", "1", testCandidateCommit, testCandidateCommit, "false", "", "128", "run_candidate=true\n", false},
		{"PR comparison error", "pull_request", "2", testCandidateCommit, testCandidateCommit, "false", "", "0", "", true},
		{"dispatch wrong commit", "workflow_dispatch", "0", strings.Repeat("a", 40), "", "false", "", "0", "", true},
		{"PR wrong commit", "pull_request", "0", strings.Repeat("a", 40), testCandidateCommit, "false", "", "0", "", true},
		{"dispatch dirty", "workflow_dispatch", "0", testCandidateCommit, "", "true", "", "0", "", true},
		{"PR invalid base", "pull_request", "0", testCandidateCommit, "invalid", "false", "", "0", "", true},
		{"unknown event", "push", "1", testCandidateCommit, "", "false", "", "0", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			output := filepath.Join(dir, "output")
			cmd := exec.Command("bash", "-c", gitFixture+strings.Join(lines, "\n"))
			cmd.Env = append(os.Environ(), "CANDIDATE_EVENT="+tc.event, "CANDIDATE_COMMIT="+testCandidateCommit,
				"OBSERVED_HEAD="+tc.head, "PR_BASE_REF=main", "PR_BASE_SHA="+tc.base, "DIFF_STATUS="+tc.diff,
				"BASE_WORKFLOW="+tc.workflow, "WORKFLOW_STATUS="+tc.workflowStatus,
				"DIRTY="+tc.dirty, "GITHUB_OUTPUT="+output, "GITHUB_STEP_SUMMARY="+filepath.Join(dir, "summary"))
			log, err := cmd.CombinedOutput()
			if (err != nil) != tc.fail {
				t.Fatalf("unexpected execution: %v %s", err, log)
			}
			result, readErr := os.ReadFile(output)
			if readErr != nil && !os.IsNotExist(readErr) {
				t.Fatal(readErr)
			}
			if string(result) != tc.want {
				t.Fatalf("decision %q; want %q", result, tc.want)
			}
		})
	}
}

func TestCandidateWorkflowPolicy(t *testing.T) {
	name := os.Getenv("FLASHGATE_CANDIDATE_WORKFLOW")
	if name == "" {
		name = filepath.Join("..", "..", ".github", "workflows", "current-version-candidate.yml")
	}
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	contents := strings.ReplaceAll(string(data), "\r\n", "\n")
	if err := validateCandidateWorkflow(contents); err != nil {
		t.Fatal(err)
	}
	mutations := []struct{ name, old, replacement string }{
		{"free version", "  pull_request:\n", "  pull_request:\n    inputs:\n"},
		{"tag version", "flashgate_read_repository_version", "GITHUB_REF_NAME#v"},
		{"missing changelog gate", "--candidate --report", "--report"},
		{"wrong PR source", "ref: ${{ github.event_name == 'pull_request' && github.event.pull_request.head.sha || github.sha }}", "ref: ${{ github.sha }}"},
		{"dispatch input", "  workflow_dispatch:", "  workflow_dispatch:\n    inputs:\n      version:"},
		{"wrong dispatch commit", "|| github.sha }}", "|| github.ref }}"},
		{"dispatch skipped", "if [[ \"$CANDIDATE_EVENT\" == 'workflow_dispatch' ]]; then\n            echo 'run_candidate=true'", "if [[ \"$CANDIDATE_EVENT\" == 'workflow_dispatch' ]]; then\n            echo 'run_candidate=false'"},
		{"dispatch falls through PR comparison", "            exit 0\n          fi\n          [[ \"$CANDIDATE_EVENT\" == 'pull_request' ]]", "          fi\n          [[ \"$CANDIDATE_EVENT\" == 'pull_request' ]]"},
		{"all unchanged PRs run", "if [[ -n \"$base_workflow\" ]]; then", "if false; then"},
		{"bootstrap removed", "base_workflow=\"$(git ls-tree --full-tree --name-only \"$base\" -- .github/workflows/current-version-candidate.yml)\"", "base_workflow='.github/workflows/current-version-candidate.yml'"},
		{"bootstrap query errors suppressed", "git ls-tree --full-tree --name-only \"$base\" -- .github/workflows/current-version-candidate.yml)", "git ls-tree --full-tree --name-only \"$base\" -- .github/workflows/current-version-candidate.yml || true)"},
		{"bootstrap checks working tree", "git ls-tree --full-tree --name-only \"$base\"", "git ls-tree --full-tree --name-only HEAD"},
		{"root version removed", "flashgate_read_repository_version", "flashgate_read_input_version"},
		{"source commit check removed", "[[ \"$(git rev-parse HEAD)\" == \"$CANDIDATE_COMMIT\" ]]", "true"},
		{"write permission", "contents: read", "contents: write"},
		{"unconditional matrix", "if: needs.prepare.outputs.run_candidate == 'true'", "if: always()"},
		{"release publication", "retention-days: 14", "retention-days: 14\n      - run: gh release create"},
		{"release mode", "--candidate --report", "--release --report"},
		{"wrong arch", "platform: windows\n            goarch: amd64\n            public_arch: x64", "platform: windows\n            goarch: arm64\n            public_arch: x64"},
		{"missing verifier", "./scripts/Test-ReleaseArtifact.ps1", "./scripts/build.ps1"},
		{"missing compare", "./cmd/releaseaudit compare", "./cmd/releaseaudit inventory"},
		{"missing scan", "            'scan',", "            'inventory',"},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed := strings.Replace(contents, mutation.old, mutation.replacement, 1)
			if changed == contents {
				t.Fatal("mutation did not change workflow")
			}
			if err := validateCandidateWorkflow(changed); err == nil {
				t.Fatal("unsafe workflow mutation passed")
			}
		})
	}
}
