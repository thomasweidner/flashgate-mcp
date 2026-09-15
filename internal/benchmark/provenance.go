package benchmark

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// ValidateAuthoritativeProvenance rejects incomplete, malformed, mismatched or
// temporally inconsistent evidence before a result can become a baseline.
func ValidateAuthoritativeProvenance(provenance AuthoritativeProvenance, binaryPath string) error {
	fields := map[string]string{
		"binary_sha256":             provenance.BinarySHA256,
		"source_tree_sha256":        provenance.SourceTreeSHA256,
		"build_inputs_sha256":       provenance.BuildInputsSHA256,
		"controller_sha256":         provenance.ControllerSHA256,
		"workspace_identity_sha256": provenance.WorkspaceIdentitySHA256,
		"preflight_evidence_sha256": provenance.PreflightEvidenceSHA256,
		"final_host_gate_sha256":    provenance.FinalHostGateSHA256,
	}
	for name, value := range fields {
		decoded, err := hex.DecodeString(value)
		if err != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != value {
			return fmt.Errorf("%s must be a lowercase SHA-256 digest", name)
		}
	}
	if provenance.PreparationStartedAtUTC.IsZero() || provenance.PreparationCompletedAtUTC.IsZero() || provenance.MeasurementStartedAtUTC.IsZero() || provenance.MeasurementCompletedAtUTC.IsZero() {
		return fmt.Errorf("all provenance timestamps are required")
	}
	if provenance.PreparationCompletedAtUTC.Before(provenance.PreparationStartedAtUTC) ||
		provenance.MeasurementStartedAtUTC.Before(provenance.PreparationCompletedAtUTC) ||
		provenance.MeasurementCompletedAtUTC.Before(provenance.MeasurementStartedAtUTC) {
		return fmt.Errorf("provenance timestamps are not ordered")
	}
	file, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("open measured binary: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash measured binary: %w", err)
	}
	if actual := hex.EncodeToString(hash.Sum(nil)); actual != provenance.BinarySHA256 {
		return fmt.Errorf("measured binary SHA-256 mismatch")
	}
	return nil
}
