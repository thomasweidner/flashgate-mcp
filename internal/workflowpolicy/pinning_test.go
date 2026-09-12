package workflowpolicy

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	externalUse = regexp.MustCompile(`^([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_./-]+)?)@([^[:space:]#]+)(?:[[:space:]]*#[[:space:]]*(.+))?$`)
	fullSHA     = regexp.MustCompile(`^[0-9a-f]{40}$`)
	reviewTag   = regexp.MustCompile(`^v[0-9]+(?:\.[0-9]+(?:\.[0-9]+)?)?(?:[[:space:]].*)?$`)
)

func TestWorkflowsPinExternalActions(t *testing.T) {
	root := filepath.Join("..", "..", ".github", "workflows")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yml" && filepath.Ext(entry.Name()) != ".yaml") {
			continue
		}
		path := filepath.Join(root, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(file)
		for line := 1; scanner.Scan(); line++ {
			value, ok := usesValue(scanner.Text())
			if !ok || strings.HasPrefix(value, "./") {
				continue
			}
			checked++
			if err := validateExternalUse(value); err != nil {
				t.Errorf("%s:%d: %v", path, line, err)
			}
		}
		if err := scanner.Err(); err != nil {
			t.Error(err)
		}
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}
	if checked == 0 {
		t.Fatal("no external action references found")
	}
}

func TestValidateExternalUse(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{"pinned", "actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6", true},
		{"floating tag", "actions/checkout@v6", false},
		{"branch", "owner/action@main # v1", false},
		{"short sha", "owner/action@d23441a # v1", false},
		{"uppercase sha", "owner/action@D23441A48E516B6C34AEA4FA41551A30E30AF803 # v6", false},
		{"missing review tag", "owner/action@d23441a48e516b6c34aea4fa41551a30e30af803", false},
		{"unreviewable comment", "owner/action@d23441a48e516b6c34aea4fa41551a30e30af803 # current", false},
		{"unreviewed docker action", "docker://alpine:latest", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateExternalUse(tt.value) == nil; got != tt.valid {
				t.Fatalf("validateExternalUse(%q) valid = %v, want %v", tt.value, got, tt.valid)
			}
		})
	}
}

func usesValue(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "uses:") {
		return "", false
	}
	value := strings.TrimSpace(strings.TrimPrefix(trimmed, "uses:"))
	return value, value != ""
}

func validateExternalUse(value string) error {
	match := externalUse.FindStringSubmatch(value)
	if match == nil {
		return fmt.Errorf("external action reference %q must use owner/repository@<full-sha> # <reviewed-version>", value)
	}
	if !fullSHA.MatchString(match[2]) {
		return fmt.Errorf("external action %s must be pinned to a lowercase 40-character commit SHA", match[1])
	}
	if !reviewTag.MatchString(strings.TrimSpace(match[3])) {
		return fmt.Errorf("external action %s must retain a reviewed vN or vN.N.N comment", match[1])
	}
	return nil
}
