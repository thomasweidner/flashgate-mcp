package tools

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const catalogFingerprintDomain = "flashgate-mcp/catalog-fingerprint/v1"

// CatalogContext contains every adapter input that can change an effective
// tool catalog without necessarily changing a tool definition.
type CatalogContext struct {
	ProtocolRevision        string
	Extensions              []string
	Profile                 string
	Capabilities            []string
	RiskPolicyVersion       string
	SchemaVersion           string
	ConfigurationGeneration string
}

// CatalogFingerprint returns a deterministic, non-reversible identifier for
// the effective catalog. Callers must supply opaque versions/generations, not
// sensitive configuration values.
func (r *Registry) CatalogFingerprint(context CatalogContext) (string, error) {
	if context.ProtocolRevision == "" || context.Profile == "" ||
		context.RiskPolicyVersion == "" || context.SchemaVersion == "" ||
		context.ConfigurationGeneration == "" {
		return "", errors.New("catalog fingerprint context is incomplete")
	}

	extensions, err := canonicalSet(context.Extensions)
	if err != nil {
		return "", err
	}
	capabilities, err := canonicalSet(context.Capabilities)
	if err != nil {
		return "", err
	}

	definitions := r.definitions()
	payload := struct {
		Domain                  string         `json:"domain"`
		ProtocolRevision        string         `json:"protocolRevision"`
		Extensions              []string       `json:"extensions"`
		Profile                 string         `json:"profile"`
		Capabilities            []string       `json:"capabilities"`
		RiskPolicyVersion       string         `json:"riskPolicyVersion"`
		SchemaVersion           string         `json:"schemaVersion"`
		ConfigurationGeneration string         `json:"configurationGeneration"`
		Tools                   []protocolTool `json:"tools"`
	}{
		Domain: catalogFingerprintDomain, ProtocolRevision: context.ProtocolRevision,
		Extensions: extensions, Profile: context.Profile, Capabilities: capabilities,
		RiskPolicyVersion: context.RiskPolicyVersion, SchemaVersion: context.SchemaVersion,
		ConfigurationGeneration: context.ConfigurationGeneration, Tools: definitions,
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", errors.New("catalog definitions are not JSON serializable")
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// protocolTool is an alias that keeps the productive protocol DTO as the
// single definition source while allowing a named slice in the hash payload.
type protocolTool = protocol.Tool

func (r *Registry) definitions() []protocolTool {
	registered := r.List()
	definitions := make([]protocolTool, 0, len(registered))
	for _, tool := range registered {
		definitions = append(definitions, tool.Definition())
	}
	return definitions
}

func canonicalSet(values []string) ([]string, error) {
	result := append([]string(nil), values...)
	sort.Strings(result)
	for index, value := range result {
		if value == "" {
			return nil, errors.New("catalog fingerprint set contains an empty value")
		}
		if index > 0 && result[index-1] == value {
			return nil, errors.New("catalog fingerprint set contains a duplicate value")
		}
	}
	return result, nil
}
