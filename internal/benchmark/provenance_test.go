package benchmark

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateAuthoritativeProvenance(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "flashgate-mcp")
	content := []byte("measured binary")
	if err := os.WriteFile(binary, content, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	hash := hex.EncodeToString(digest[:])
	start := time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)
	valid := AuthoritativeProvenance{
		BinarySHA256: hash, SourceTreeSHA256: hash, BuildInputsSHA256: hash,
		ControllerSHA256: hash, WorkspaceIdentitySHA256: hash,
		PreflightEvidenceSHA256: hash, FinalHostGateSHA256: hash,
		PreparationStartedAtUTC: start, PreparationCompletedAtUTC: start.Add(time.Minute),
		MeasurementStartedAtUTC: start.Add(2 * time.Minute), MeasurementCompletedAtUTC: start.Add(3 * time.Minute),
	}
	if err := ValidateAuthoritativeProvenance(valid, binary); err != nil {
		t.Fatalf("valid provenance rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*AuthoritativeProvenance)
		want   string
	}{
		{"missing digest", func(p *AuthoritativeProvenance) { p.ControllerSHA256 = "" }, "controller_sha256"},
		{"uppercase digest", func(p *AuthoritativeProvenance) { p.SourceTreeSHA256 = strings.ToUpper(hash) }, "source_tree_sha256"},
		{"unordered timestamps", func(p *AuthoritativeProvenance) { p.MeasurementStartedAtUTC = start }, "not ordered"},
		{"binary mismatch", func(p *AuthoritativeProvenance) { p.BinarySHA256 = strings.Repeat("0", 64) }, "binary SHA-256 mismatch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			err := ValidateAuthoritativeProvenance(candidate, binary)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want %q", err, test.want)
			}
		})
	}
}
