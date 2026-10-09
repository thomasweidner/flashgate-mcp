package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
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
	if err := validateCandidateTrustBoundary(contents); err != nil {
		return err
	}
	required := []string{
		"CANDIDATE_COMMIT: ${{ github.event.pull_request.head.sha }}",
		"[[ \"$CANDIDATE_REF\" == 'refs/heads/main' && \"$CANDIDATE_REF_TYPE\" == 'branch' ]]",
		"CANDIDATE_COMMIT=\"$(git rev-parse refs/remotes/origin/main^{commit})\"",
		"if [[ \"$CANDIDATE_EVENT\" == 'workflow_dispatch' ]]; then\n            echo 'run_candidate=true' >> \"$GITHUB_OUTPUT\"\n            echo 'Explicit current-version reverification at the validated current main commit.' >> \"$GITHUB_STEP_SUMMARY\"\n            exit 0\n          fi\n          [[ \"$CANDIDATE_EVENT\" == 'pull_request' ]]",

		"  pull_request:\n    branches:\n      - main",
		"permissions:\n  contents: read\n",
		"  workflow_dispatch:\n  pull_request:",
		"[[ \"$(git rev-parse HEAD)\" == \"$CANDIDATE_COMMIT\" ]]",
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
	bash := candidateBash(t)
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
    rev-parse)
      if [[ "$2" == HEAD ]]; then printf '%s\n' "$OBSERVED_HEAD"
      elif [[ "$2" == refs/remotes/origin/main^{commit} ]]; then
        [[ "$MAIN_STATUS" == 0 ]] || return "$MAIN_STATUS"
        printf '%s\n' "$MAIN_HEAD"
      else printf '%s\n' "$PR_BASE_SHA"; fi ;;
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
		{"dispatch non-main branch", "workflow_dispatch", "0", testCandidateCommit, "", "false", "", "0", "", true},
		{"dispatch tag", "workflow_dispatch", "0", testCandidateCommit, "", "false", "", "0", "", true},
		{"dispatch main-shaped tag", "workflow_dispatch", "0", testCandidateCommit, "", "false", "", "0", "", true},
		{"dispatch main lookup failure", "workflow_dispatch", "0", testCandidateCommit, "", "false", "", "0", "", true},
		{"dispatch main invalid SHA", "workflow_dispatch", "0", testCandidateCommit, "", "false", "", "0", "", true},
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
			cmd := exec.Command(bash, "-c", gitFixture+strings.Join(lines, "\n"))
			ref, refType, mainHead, mainStatus := "refs/heads/main", "branch", testCandidateCommit, "0"
			switch tc.name {
			case "dispatch non-main branch":
				ref = "refs/heads/feature"
			case "dispatch tag":
				ref, refType = "refs/tags/main", "tag"
			case "dispatch main-shaped tag":
				refType = "tag"
			case "dispatch main lookup failure":
				mainStatus = "128"
			case "dispatch main invalid SHA":
				mainHead = "invalid"
			}
			cmd.Env = append(os.Environ(), "CANDIDATE_EVENT="+tc.event, "CANDIDATE_COMMIT="+testCandidateCommit,
				"OBSERVED_HEAD="+tc.head, "CANDIDATE_REF="+ref, "CANDIDATE_REF_TYPE="+refType,
				"MAIN_HEAD="+mainHead, "MAIN_STATUS="+mainStatus, "PR_BASE_REF=main", "PR_BASE_SHA="+tc.base, "DIFF_STATUS="+tc.diff,
				"BASE_WORKFLOW="+tc.workflow, "WORKFLOW_STATUS="+tc.workflowStatus,
				"DIRTY="+tc.dirty, "GITHUB_OUTPUT="+filepath.ToSlash(output), "GITHUB_STEP_SUMMARY="+filepath.ToSlash(filepath.Join(dir, "summary")))
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
		{"wrong PR source", "ref: ${{ github.event.pull_request.head.sha }}", "ref: ${{ github.sha }}"},
		{"dispatch event SHA", "ref: refs/heads/main", "ref: ${{ github.sha }}"},
		{"dispatch output SHA", "ref: refs/heads/main", "ref: ${{ needs.prepare.outputs.source_commit }}"},
		{"prepare main guard removed", "    if: github.event_name == 'pull_request' || (github.event_name == 'workflow_dispatch' && github.ref == 'refs/heads/main' && github.ref_type == 'branch')\n", ""},
		{"prepare main guard relaxed", "    if: github.event_name == 'pull_request' || (github.event_name == 'workflow_dispatch' && github.ref == 'refs/heads/main' && github.ref_type == 'branch')", "    if: github.event_name == 'pull_request' || github.event_name == 'workflow_dispatch'"},
		{"tag semantic guard removed", " && github.ref_type == 'branch'", ""},
		{"checkout guard relaxed", "        if: github.event_name == 'workflow_dispatch' && github.ref == 'refs/heads/main' && github.ref_type == 'branch'", "        if: github.event_name == 'workflow_dispatch'"},
		{"Go cache enabled", "          cache: false", "          cache: true"},
		{"Go cache missing", "          cache: false\n", ""},
		{"dispatch main runtime readback removed", "git rev-parse refs/remotes/origin/main^{commit}", "git rev-parse HEAD"},
		{"dispatch runtime ref guard removed", "[[ \"$CANDIDATE_REF\" == 'refs/heads/main' && \"$CANDIDATE_REF_TYPE\" == 'branch' ]]", "true"},
		{"dispatch input", "  workflow_dispatch:", "  workflow_dispatch:\n    inputs:\n      version:"},
		{"wrong dispatch commit", "ref: refs/heads/main", "ref: ${{ github.ref }}"},
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

// Evaluate the closed event/ref predicates rather than matching one YAML spelling.
// Unsupported expressions fail closed; parentheses and whitespace are immaterial.
func candidateConditionAllows(condition string, context map[string]string) (bool, error) {
	expression, err := parser.ParseExpr(strings.ReplaceAll(condition, "'", `"`))
	if err != nil {
		return false, err
	}
	var fieldName func(ast.Expr) string
	fieldName = func(e ast.Expr) string {
		switch e := e.(type) {
		case *ast.Ident:
			return e.Name
		case *ast.SelectorExpr:
			return fieldName(e.X) + "." + e.Sel.Name
		}
		return ""
	}
	stringValue := func(e ast.Expr) (string, error) {
		if literal, ok := e.(*ast.BasicLit); ok && literal.Kind == token.STRING {
			return strconv.Unquote(literal.Value)
		}
		if value, ok := context[fieldName(e)]; ok {
			return value, nil
		}
		return "", errors.New("unsupported candidate condition operand")
	}
	var evaluate func(ast.Expr) (bool, error)
	evaluate = func(e ast.Expr) (bool, error) {
		if paren, ok := e.(*ast.ParenExpr); ok {
			return evaluate(paren.X)
		}
		binary, ok := e.(*ast.BinaryExpr)
		if !ok {
			return false, errors.New("unsupported candidate condition")
		}
		if binary.Op == token.EQL {
			a, err := stringValue(binary.X)
			if err != nil {
				return false, err
			}
			b, err := stringValue(binary.Y)
			return a == b, err
		}
		if binary.Op != token.LAND && binary.Op != token.LOR {
			return false, errors.New("unsupported candidate condition operator")
		}
		a, err := evaluate(binary.X)
		if err != nil {
			return false, err
		}
		b, err := evaluate(binary.Y)
		if err != nil {
			return false, err
		}
		if binary.Op == token.LAND {
			return a && b, nil
		}
		return a || b, nil
	}
	return evaluate(expression)
}

var candidateTrustCases = []struct {
	event, ref, refType string
	prepare, pr, main   bool
}{
	{"pull_request", "refs/pull/303/merge", "branch", true, true, false},
	{"workflow_dispatch", "refs/heads/main", "branch", true, false, true},
	{"workflow_dispatch", "refs/heads/feature", "branch", false, false, false},
	{"workflow_dispatch", "refs/tags/main", "tag", false, false, false},
	{"workflow_dispatch", "refs/heads/main", "tag", false, false, false},
	{"push", "refs/heads/main", "branch", false, false, false},
}

func candidateConditionContext(event, ref, refType string) map[string]string {
	return map[string]string{"github.event_name": event, "github.ref": ref, "github.ref_type": refType}
}

var candidateStepPattern = regexp.MustCompile(`(?m)^      - `)
var candidateIfPattern = regexp.MustCompile(`(?m)^    if: (.+)$`)
var candidateStepIfPattern = regexp.MustCompile(`(?m)^        if: (.+)$`)
var candidateRefPattern = regexp.MustCompile(`(?m)^          ref: (.+)$`)

func candidateCheckoutSteps(contents string) []string {
	var result []string
	for _, step := range candidateStepPattern.Split(contents, -1) {
		if strings.Contains(step, "        uses: actions/checkout@") {
			result = append(result, step)
		}
	}
	return result
}

func validateCandidateTrustBoundary(contents string) error {
	prepare := strings.SplitN(contents, "  build:\n", 2)[0]
	guard := candidateIfPattern.FindStringSubmatch(prepare)
	if len(guard) != 2 {
		return errors.New("candidate prepare requires a pre-checkout event guard")
	}
	for _, tc := range candidateTrustCases {
		allowed, err := candidateConditionAllows(guard[1], candidateConditionContext(tc.event, tc.ref, tc.refType))
		if err != nil || allowed != tc.prepare {
			return errors.New("candidate prepare guard violates the main dispatch boundary")
		}
	}
	steps := candidateCheckoutSteps(contents)
	if len(steps) != 6 {
		return errors.New("each candidate job requires separate PR and main checkouts")
	}
	for index, step := range steps {
		condition := candidateStepIfPattern.FindStringSubmatch(step)
		ref := candidateRefPattern.FindStringSubmatch(step)
		if len(condition) != 2 || len(ref) != 2 || !strings.Contains(step, "          fetch-depth: 0\n") {
			return errors.New("candidate checkout must have a guarded exact ref and full history")
		}
		pr := index%2 == 0
		if pr && ref[1] != "${{ github.event.pull_request.head.sha }}" || !pr && strings.Trim(ref[1], "'\"") != "refs/heads/main" {
			return errors.New("candidate checkout ref crosses the PR/main trust boundary")
		}
		for _, tc := range candidateTrustCases {
			want := tc.main
			if pr {
				want = tc.pr
			}
			allowed, err := candidateConditionAllows(condition[1], candidateConditionContext(tc.event, tc.ref, tc.refType))
			if err != nil || allowed != want {
				return errors.New("candidate checkout guard permits an untrusted event/ref")
			}
		}
	}
	goSetups := 0
	for _, step := range candidateStepPattern.Split(contents, -1) {
		if strings.Contains(step, "        uses: actions/setup-go@") {
			goSetups++
			if strings.Count(step, "          cache: false\n") != 1 || strings.Contains(step, "cache: true") {
				return errors.New("every candidate Go setup must disable caching")
			}
		}
	}
	if goSetups != 3 {
		return errors.New("candidate requires three uncached Go setups")
	}
	return nil
}

func TestCandidateCheckoutBoundaryExecution(t *testing.T) {
	bash := candidateBash(t)
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "current-version-candidate.yml"))
	if err != nil {
		t.Fatal(err)
	}
	contents := strings.ReplaceAll(string(data), "\r\n", "\n")
	guard := candidateIfPattern.FindStringSubmatch(strings.SplitN(contents, "  build:\n", 2)[0])[1]
	// Run the actual job and checkout expressions in Bash. The action's ref is
	// the observable payload selector; no provider or repository code is invoked.
	bashCondition := func(condition string) string {
		return strings.NewReplacer("github.event_name", `"$EVENT"`, "github.ref_type", `"$REF_TYPE"`, "github.ref", `"$REF"`).Replace(condition)
	}
	for _, tc := range candidateTrustCases {
		t.Run(tc.event+"/"+tc.ref+"/"+tc.refType, func(t *testing.T) {
			var script strings.Builder
			script.WriteString("set -Eeuo pipefail\nif [[ " + bashCondition(guard) + " ]]; then\n")
			for _, step := range candidateCheckoutSteps(contents) {
				condition := candidateStepIfPattern.FindStringSubmatch(step)[1]
				ref := candidateRefPattern.FindStringSubmatch(step)[1]
				selector := "main"
				if ref == "${{ github.event.pull_request.head.sha }}" {
					selector = testCandidateCommit
				}
				script.WriteString("if [[ " + bashCondition(condition) + " ]]; then printf '%s\\n' '" + selector + "'; fi\n")
			}
			script.WriteString("fi\n")
			cmd := exec.Command(bash, "-c", script.String())
			cmd.Env = append(os.Environ(), "EVENT="+tc.event, "REF="+tc.ref, "REF_TYPE="+tc.refType)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("expression execution failed: %v %s", err, output)
			}
			want := ""
			if tc.main {
				want = strings.Repeat("main\n", 3)
			} else if tc.pr {
				want = strings.Repeat(testCandidateCommit+"\n", 3)
			}
			if string(output) != want {
				t.Fatalf("payload selectors %q; want %q", output, want)
			}
		})
	}
}

func candidateBash(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		path := `C:/Program Files/Git/bin/bash.exe`
		if _, err := os.Stat(path); err != nil {
			t.Fatal("actual candidate Bash tests require Git Bash on Windows")
		}
		return path
	}
	path, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("actual candidate Bash tests require Bash")
	}
	return path
}

func TestLinuxVerifierArchiveHash(t *testing.T) {
	// Exercise the complete production shell script and real TAR/checksum/hash
	// tools. Metadata and Go/Python inventory gates are controlled PASS fixtures;
	// this is archive-hash regression evidence, not real binary verification.
	bash := candidateBash(t)
	for _, arch := range []string{"x64", "arm64"} {
		for _, mode := range []string{"pass", "original changed", "hash fails", "malformed", "short", "checksum fails"} {
			t.Run(arch+"/"+mode, func(t *testing.T) {
				root := t.TempDir()
				if err := os.Mkdir(filepath.Join(root, "scripts"), 0o700); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"test-release-artifact.sh", "build-input-validation.sh"} {
					data, err := os.ReadFile(filepath.Join("..", "..", "scripts", name))
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(root, "scripts", name), data, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.WriteFile(filepath.Join(root, "scripts", "Test-LinuxMetadata.sh"), []byte("#!/usr/bin/env bash\nprintf metadata > \"$FIXTURE_ROOT/metadata-called\"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				fixture := `set -euo pipefail
cd "$1"
export FIXTURE_ROOT="$PWD" FIXTURE_MODE="$2"
arch="$3"
goarch=amd64
[[ "$arch" != arm64 ]] || goarch=arm64
base="flashgate-mcp_0.3.0_linux_$arch"
mkdir "$base" scratch
for name in LICENSE README.md THIRD-PARTY-NOTICES.md flashgate-mcp; do
    printf '%s\n' "$name" > "$base/$name"
done
tar -czf "$base.tar.gz" "$base/"
sha256sum "$base.tar.gz" > "$base.tar.gz.sha256"
expected="$(sha256sum "$base.tar.gz")"
expected="${expected%% *}"
export TMPDIR="$PWD/scratch"
go() { return 0; }
python3() { return 0; }
cp() {
    command cp "$@" || return
    if [[ "$FIXTURE_MODE" == 'original changed' && "$2" == *.tar.gz ]]; then
        printf changed > "$2"
    fi
}
sha256sum() {
    if [[ "$1" == -- ]]; then
        case "$FIXTURE_MODE" in
            'hash fails') printf '%s\n' 'fixture hash failure' >&2; return 1 ;;
            malformed) printf '%064d  %s\n' 0 "$2" | tr '0' g; return 0 ;;
            short) printf 'abc  %s\n' "$2"; return 0 ;;
        esac
    fi
    if [[ "$FIXTURE_MODE" == 'checksum fails' && "$1" == -c ]]; then return 1; fi
    command sha256sum "$@"
}
export -f go python3 cp sha256sum
set +e
bash scripts/test-release-artifact.sh \
    --archive "$PWD/$base.tar.gz" --checksum "$PWD/$base.tar.gz.sha256" \
    --expected-version 0.3.0 --expected-public-arch "$arch" --expected-goarch "$goarch" \
    --expected-commit 0123456789abcdef0123456789abcdef01234567 \
    --expected-source-time 2026-01-01T00:00:00Z --expected-modified false | tee output.txt
code=${PIPESTATUS[0]}
set -e
printf 'EXPECTED_HASH=%s\nEXIT=%s\n' "$expected" "$code"
`
				cmd := exec.Command(bash, "-c", fixture, "fixture", filepath.ToSlash(root), mode, arch)
				data, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("fixture execution: %v\n%s", err, data)
				}
				text := string(data)
				pass := mode == "pass" || mode == "original changed"
				if strings.Count(text, "Status: ") != 1 || strings.Count(text, "Sha256: ") != 1 {
					t.Fatalf("terminal report is ambiguous:\n%s", text)
				}
				if pass {
					expected := strings.Split(strings.Split(text, "EXPECTED_HASH=")[1], "\n")[0]
					if !strings.Contains(text, "Status: PASS\n") || !strings.Contains(text, "EXIT=0\n") || !strings.Contains(text, "Sha256: "+expected+"\n") {
						t.Fatalf("checked controlled archive hash was lost by tee:\n%s", text)
					}
					if _, err := os.Stat(filepath.Join(root, "metadata-called")); err != nil {
						t.Fatal("metadata fixture was not reached", err)
					}
				} else {
					if !strings.Contains(text, "Status: FAIL\n") || strings.Contains(text, "EXIT=0\n") || !strings.Contains(text, "Sha256: \n") {
						t.Fatalf("invalid hash/checksum did not fail closed:\n%s", text)
					}
					if _, err := os.Stat(filepath.Join(root, "metadata-called")); !os.IsNotExist(err) {
						t.Fatal("hash failure reached metadata gate", err)
					}
				}
			})
		}
	}
}
