package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
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
