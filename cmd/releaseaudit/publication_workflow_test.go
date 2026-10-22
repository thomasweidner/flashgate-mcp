package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This is a closed regression contract for the maintained entry point, not a
// general YAML parser or GitHub expression engine. It fixes the complete guarded
// expression; local history tests exercise checkAttempt, not hosted rendering.
// The shell gate separately checks executable Bash blocks.
func validatePublicationWorkflow(contents string) error {
	required := []string{
		"on:\n  workflow_dispatch:\n    inputs:\n",
		"run-name: ${{ github.event_name == 'workflow_dispatch' && inputs.authorization != '' && format('BL-363 publication {0}', fromJSON(inputs.authorization).originalApproval) || 'BL-363 dispatch required' }}",
		"permissions:\n  contents: read\n",
		"concurrency:\n  group: flashgate-public-prerelease\n  cancel-in-progress: false",
		"    if: github.event_name == 'workflow_dispatch' && github.ref == 'refs/heads/main' && github.run_attempt == 1 && github.repository == 'thomasweidner/flashgate-mcp'",
		"    permissions:\n      contents: write\n      actions: read\n",
		"persist-credentials: false",
		"PUBLICATION_AUTHORIZATION: ${{ inputs.authorization }}",
		"hashlib.sha256(data).hexdigest() == os.environ['FLASHGATE_AUTHORIZATION_SHA256']",
		"ref: ${{ steps.authorization.outputs.commit }}",
		"publication-download", "publication-preflight", "publication-publish",
		"--authorization-sha256", "--output \"$FLASHGATE_WORK_ROOT/attempt.jsonl\"",
		"      - name: Retain plan and one-shot journal for reconciliation\n        if: always()",
	}
	for _, item := range required {
		if !strings.Contains(contents, item) {
			return errors.New("missing publication workflow boundary")
		}
	}
	for _, item := range []string{"  push:", "  pull_request:", "workflow_run:", "write-all", "issues:", "pull-requests:", "administration:", "continue-on-error:", "retry", "gh release", "curl ", "git tag ", "New-ReleaseArtifact", "new-release-artifact", "./cmd/server", "make_latest: true", "secrets."} {
		if strings.Contains(contents, item) {
			return errors.New("forbidden publication workflow alternative")
		}
	}
	if strings.Count(contents, "permissions:") != 2 || strings.Count(contents, "contents: write") != 1 || strings.Count(contents, "GH_TOKEN: ${{ github.token }}") != 2 || strings.Count(contents, "persist-credentials: false") != 2 {
		return errors.New("publication permission/credential scope drift")
	}
	// Job env is evaluated before a runner is assigned. runner.temp belongs only
	// to the initializer's step env; all consumers share its GITHUB_ENV values.
	_, jobEnv, ok := strings.Cut(contents, "    env:\n")
	if !ok {
		return errors.New("missing publication job environment")
	}
	jobEnv, _, ok = strings.Cut(jobEnv, "    steps:\n")
	if !ok || jobEnv != "      FLASHGATE_AUTHORIZATION_SHA256: ${{ inputs.authorization_sha256 }}\n" {
		return errors.New("publication job environment context drift")
	}
	step := func(name string) string {
		_, body, ok := strings.Cut(contents, "      - name: "+name+"\n")
		if !ok {
			return ""
		}
		body, _, _ = strings.Cut(body, "\n      - name:")
		return body
	}
	initialize := step("Initialize exclusive publication workspace")
	for _, item := range []string{
		"        id: workspace\n",
		"        env:\n          PUBLICATION_RUNNER_TEMP: ${{ runner.temp }}\n",
		"        shell: python\n",
		"root = pathlib.Path(os.environ['PUBLICATION_RUNNER_TEMP']) / 'publication'",
		"root.mkdir(mode=0o700, exist_ok=False)",
		"for name in ('go-cache', 'go-tmp', 'tmp'):\n              (root / name).mkdir(mode=0o700)",
		"'FLASHGATE_WORK_ROOT': root,", "'GOCACHE': root / 'go-cache',",
		"'GOTMPDIR': root / 'go-tmp',", "'TMPDIR': root / 'tmp',",
		"with open(os.environ['GITHUB_ENV'], 'a', encoding='utf-8', newline='\\n') as output:",
		"output.write(name + '=' + str(path) + '\\n')",
	} {
		if !strings.Contains(initialize, item) {
			return errors.New("publication runtime workspace binding drift")
		}
	}
	authorization := step("Bind authorization without shell interpolation")
	if strings.Count(contents, "runner.temp") != 1 || strings.Contains(initialize, "GITHUB_OUTPUT") || strings.Contains(initialize, "inputs.") || strings.Contains(initialize, "exist_ok=True") || strings.Contains(authorization, ".mkdir(") || !strings.Contains(authorization, "assert root.is_dir() and not root.is_symlink()") || !strings.Contains(authorization, "(root / 'authorization.json').open('xb')") {
		return errors.New("publication workspace initialization/authorization drift")
	}
	if strings.Index(contents, initialize) > strings.Index(contents, authorization) || strings.Contains(contents[:strings.Index(contents, initialize)], "FLASHGATE_WORK_ROOT") {
		return errors.New("publication workspace used before initialization")
	}
	retention := step("Retain plan and one-shot journal for reconciliation")
	if !strings.Contains(retention, "if: always() && steps.workspace.outcome == 'success'") || !strings.Contains(retention, "${{ env.FLASHGATE_WORK_ROOT }}/plan.json\n            ${{ env.FLASHGATE_WORK_ROOT }}/attempt.jsonl") {
		return errors.New("publication retention root drift")
	}
	if strings.Count(contents, "GITHUB_OUTPUT") != 1 || !strings.Contains(authorization, "output.write('commit=' + commit + '\\n')") || strings.Count(contents, "publication-download \\\n") != 1 || strings.Count(contents, "publication-preflight \\\n") != 1 || strings.Count(contents, "publication-publish \\\n") != 1 {
		return errors.New("publication output/one-shot command drift")
	}
	return nil
}

func TestPublicationWorkflowPolicy(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "publish-prerelease.yml"))
	if err != nil {
		t.Fatal(err)
	}
	contents := strings.ReplaceAll(string(data), "\r\n", "\n")
	if err := validatePublicationWorkflow(contents); err != nil {
		t.Fatal(err)
	}
	mutations := [][2]string{
		{"  workflow_dispatch:", "  push:\n  workflow_dispatch:"},
		{"  contents: read", "  contents: write"},
		{"      contents: write", "      write-all: true"},
		{"github.run_attempt == 1", "github.run_attempt >= 1"},
		{"github.ref == 'refs/heads/main'", "true"},
		{"cancel-in-progress: false", "cancel-in-progress: true"},
		{"persist-credentials: false", "persist-credentials: true"},
		{"publication-preflight", "source"},
		{"publication-download", "new-release-artifact"},
		{"./cmd/releaseaudit", "./cmd/server"},
		{"--authorization-sha256", "--untrusted-sha256"},
		{"if: always()", "if: success()"},
		{"      actions: read", "      actions: write\n      issues: write"},
		{"PUBLICATION_AUTHORIZATION: ${{ inputs.authorization }}", "PUBLICATION_AUTHORIZATION: empty"},
		{"      FLASHGATE_AUTHORIZATION_SHA256:", "      GOCACHE: ${{ runner.temp }}/publication/go-cache\n      FLASHGATE_AUTHORIZATION_SHA256:"},
		{"PUBLICATION_RUNNER_TEMP: ${{ runner.temp }}", "PUBLICATION_RUNNER_TEMP: fixed"},
		{"github.event_name == 'workflow_dispatch' && inputs.authorization != '' && format(", "format("},
		{"inputs.authorization != '' && format(", "format("},
		{"'BL-363 dispatch required'", "'BL-363 publication approval'"},
		{"root.mkdir(mode=0o700, exist_ok=False)", "root.mkdir(mode=0o700, exist_ok=True)"},
		{"'GOCACHE': root / 'go-cache'", "'GOCACHE': root / 'tmp'"},
		{"encoding='utf-8'", "encoding='ascii'"},
		{"assert root.is_dir() and not root.is_symlink()", "root.mkdir(exist_ok=False)"},
		{"${{ env.FLASHGATE_WORK_ROOT }}/attempt.jsonl", "${{ runner.temp }}/different/attempt.jsonl"},
		{" && steps.workspace.outcome == 'success'", ""},
		{"output.write('commit=' + commit + '\\n')", "output.write(data.decode('utf-8'))"},
	}
	for _, m := range mutations {
		if !strings.Contains(contents, m[0]) {
			t.Fatal("workflow mutation did not match", m)
		}
		if validatePublicationWorkflow(strings.ReplaceAll(contents, m[0], m[1])) == nil {
			t.Fatal("workflow mutation accepted", m)
		}
	}
	for _, name := range []string{"release-build.yml", "current-version-candidate.yml"} {
		data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := validateReleasePermissions(strings.ReplaceAll(string(data), "\r\n", "\n")); err != nil {
			t.Fatalf("build permission regression %s: %v", name, err)
		}
		if strings.Contains(string(data), "publication-publish") {
			t.Fatal("build path gained publication")
		}
	}
}

func TestPublicationWorkflowRunNameApprovalHistory(t *testing.T) {
	// Full-length canonical references must survive unchanged. These are actual
	// checkAttempt calls through a local read-only provider double, not a model
	// of GitHub's expression evaluator or a simulated publication.
	approval := "Approval." + strings.Repeat("a", 86) + "Z"
	const fallback = "BL-363 dispatch required"
	type run struct {
		ID      int64  `json:"id"`
		Attempt int    `json:"run_attempt"`
		Title   string `json:"display_title"`
		Event   string `json:"event"`
		Branch  string `json:"head_branch"`
	}
	current := run{999, 1, "BL-363 publication " + approval, "workflow_dispatch", "main"}
	for _, tc := range []struct {
		name string
		runs []run
		want bool
	}{
		{"exact-original-approval", []run{current}, true},
		{"fallback-is-not-current-approval", []run{{999, 1, fallback, "workflow_dispatch", "main"}}, false},
		{"non-dispatch-fallback-ignored", []run{{998, 1, fallback, "push", "main"}, current}, true},
		{"consumed-original-approval", []run{{998, 1, current.Title, "workflow_dispatch", "main"}, current}, false},
		{"truncated-approval", []run{{999, 1, strings.TrimSuffix(current.Title, "Z"), "workflow_dispatch", "main"}}, false},
		{"rerun", []run{{999, 2, current.Title, "workflow_dispatch", "main"}}, false},
		{"non-dispatch-cannot-claim-approval", []run{{999, 1, current.Title, "push", "main"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reads := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reads++
				if r.Method != http.MethodGet || r.URL.Path != "/repos/thomasweidner/flashgate-mcp/actions/workflows/publish-prerelease.yml/runs" || r.URL.RawQuery != "per_page=100&page=1" {
					t.Errorf("unexpected history operation: %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"workflow_runs": tc.runs})
			}))
			defer server.Close()
			g := &githubPublication{client: server.Client(), token: "test-only", runID: 999, api: server.URL}
			if err := g.checkAttempt(context.Background(), approval); (err == nil) != tc.want {
				t.Fatalf("history acceptance = %v, want %v: %v", err == nil, tc.want, err)
			}
			if reads != 1 {
				t.Fatalf("history reads = %d, want 1", reads)
			}
		})
	}
}
