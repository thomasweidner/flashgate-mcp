package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This is a closed regression contract for the maintained entry point, not a
// general YAML parser. The shell gate separately checks executable Bash blocks.
func validatePublicationWorkflow(contents string) error {
	required := []string{
		"on:\n  workflow_dispatch:\n    inputs:\n",
		"run-name: BL-363 publication ${{ fromJSON(inputs.authorization).originalApproval }}",
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
	}
	for _, m := range mutations {
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
