package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validateSecurityWorkflow(text string, publisher bool) error {
	stepName := "      - name: Informative dependency security assessment\n"
	if publisher {
		stepName = "      - name: Informative security assessment after confirmed publication\n"
	}
	start := strings.Index(text, stepName)
	if start < 0 {
		return errors.New("missing informative assessment")
	}
	end := strings.Index(text[start+len(stepName):], "      - name:")
	if end < 0 {
		return errors.New("missing result summary")
	}
	step := text[start : start+len(stepName)+end]
	for _, required := range []string{"continue-on-error: true", "security-assess", "--online", "--triage", "--manifest-sha256", "--index-sha256", "--output"} {
		if !strings.Contains(step, required) {
			return errors.New("assessment boundary missing: " + required)
		}
	}
	if strings.Contains(step, "GH_TOKEN") || strings.Contains(step, "github.token") || strings.Contains(step, "permissions:") {
		return errors.New("assessment credential or permission expansion")
	}
	allowed := map[string]bool{"Informative dependency security assessment": true, "Summarize informative security result": true, "Retain temporary security assessment": true}
	if publisher {
		allowed = map[string]bool{"Informative security assessment after confirmed publication": true, "Summarize informative post-publication security result": true, "Retain temporary post-publication security assessment": true}
	}
	for _, block := range strings.Split(text, "      - name: ") {
		if strings.Contains(block, "continue-on-error:") {
			name, _, _ := strings.Cut(block, "\n")
			if !allowed[name] {
				return errors.New("existing gate made nonblocking")
			}
		}
	}
	if publisher {
		if !strings.Contains(step, "if: success() && steps.publication.outcome == 'success'") || strings.Index(text, "publication-publish") > start {
			return errors.New("assessment precedes successful publication")
		}
		if strings.Count(text, "publication-publish") != 1 || strings.Count(text, "contents: write") != 1 {
			return errors.New("publication write contract changed")
		}
	} else {
		if strings.Index(text, "./cmd/releaseaudit supply-chain-verify") > start || strings.Index(text, "Upload complete verified supply-chain bundle") > start {
			return errors.New("assessment precedes verification")
		}
		if strings.Count(text, "matrix:") != 1 || strings.Count(text, "supply-chain-generate") != 1 || strings.Contains(text, "contents: write") {
			return errors.New("candidate authority expanded")
		}
		if !strings.Contains(text, "workflow_dispatch:") {
			return errors.New("manual dispatch removed")
		}
	}
	for _, name := range []string{"Summarize informative", "Retain temporary"} {
		pos := strings.Index(text[start:], "      - name: "+name)
		if pos < 0 {
			return errors.New("missing temporary result consumer")
		}
		block := text[start+pos:]
		if next := strings.Index(block[10:], "      - name:"); next >= 0 {
			block = block[:10+next]
		}
		if !strings.Contains(block, "continue-on-error: true") {
			return errors.New("result consumer can block")
		}
		if publisher && !strings.Contains(block, "steps.publication.outcome == 'success'") {
			return errors.New("consumer unbound to publication success")
		}
	}
	return nil
}

func TestSecurityWorkflowNonblocking(t *testing.T) {
	for _, publisher := range []bool{false, true} {
		name := "current-version-candidate.yml"
		if publisher {
			name = "publish-prerelease.yml"
		}
		data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ReplaceAll(string(data), "\r\n", "\n")
		if err := validateSecurityWorkflow(text, publisher); err != nil {
			t.Fatal(name, err)
		}
		gate := "Generate and independently verify supply-chain evidence"
		if publisher {
			gate = "Publish once after independent draft asset readback"
		}
		badGate := strings.Replace(text, "      - name: "+gate+"\n", "      - name: "+gate+"\n        continue-on-error: true\n", 1)
		if validateSecurityWorkflow(badGate, publisher) == nil {
			t.Fatal("blocking integrity/publication gate weakened")
		}
		for _, old := range []string{"continue-on-error: true", "--online", "--manifest-sha256", "--index-sha256"} {
			start := strings.Index(text, "      - name: Informative")
			bad := text[:start] + strings.Replace(text[start:], old, "REMOVED", 1)
			if err := validateSecurityWorkflow(bad, publisher); err == nil {
				t.Fatal("unsafe workflow accepted", name, old)
			}
		}
		if publisher {
			bad := strings.Replace(text, "if: success() && steps.publication.outcome == 'success'", "if: always()", 1)
			if validateSecurityWorkflow(bad, true) == nil {
				t.Fatal("failed publication can trigger assessment")
			}
		}
	}
}
