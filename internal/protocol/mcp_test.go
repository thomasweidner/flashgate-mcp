package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestProtocolVersionIsSet(t *testing.T) {
	t.Parallel()

	if ProtocolVersion == "" {
		t.Fatal("expected protocol version to be set")
	}
}

type protocolMatrix struct {
	SchemaVersion string `json:"schemaVersion"`
	ProductTarget string `json:"productTarget"`
	Revisions     []struct {
		ProtocolVersion            string   `json:"protocolVersion"`
		Opening                    string   `json:"opening"`
		UnsupportedVersionBehavior string   `json:"unsupportedVersionBehavior"`
		Extensions                 []string `json:"extensions"`
		Transports                 []string `json:"transports"`
	} `json:"revisions"`
}

func validateProtocolMatrix(data []byte) error {
	if !utf8.Valid(data) {
		return errors.New("matrix is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var matrix protocolMatrix
	if err := decoder.Decode(&matrix); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("trailing JSON: %v", err)
	}
	if matrix.SchemaVersion != "flashgate-mcp-protocol-matrix/v1" || matrix.ProductTarget != "1.0" {
		return errors.New("unexpected matrix schema or product target")
	}
	seen := make(map[string]bool)
	for _, revision := range matrix.Revisions {
		if seen[revision.ProtocolVersion] {
			return errors.New("duplicate revision")
		}
		seen[revision.ProtocolVersion] = true
	}
	if len(matrix.Revisions) != 1 {
		return errors.New("expected exactly one advertised revision")
	}
	revision := matrix.Revisions[0]
	if ProtocolVersion != "2025-11-25" || revision.ProtocolVersion != ProtocolVersion ||
		revision.Opening != "initialize" ||
		revision.UnsupportedVersionBehavior != "respond-with-supported-revision" ||
		revision.Extensions == nil || len(revision.Extensions) != 0 ||
		len(revision.Transports) != 1 || revision.Transports[0] != "stdio" {
		return errors.New("matrix advertises an unsupported protocol contract")
	}
	return nil
}

func TestAdvertisedProtocolMatrix(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "mcp-protocol-matrix.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateProtocolMatrix(data); err != nil {
		t.Fatalf("invalid advertised matrix: %v", err)
	}
}

func TestProtocolMatrixRejectsUnsupportedContracts(t *testing.T) {
	t.Parallel()
	const valid = `{"schemaVersion":"flashgate-mcp-protocol-matrix/v1","productTarget":"1.0","revisions":[{"protocolVersion":"2025-11-25","opening":"initialize","unsupportedVersionBehavior":"respond-with-supported-revision","extensions":[],"transports":["stdio"]}]}`
	if err := validateProtocolMatrix([]byte(valid)); err != nil {
		t.Fatalf("fixture must be valid: %v", err)
	}
	tests := []struct{ name, old, replacement string }{
		{"unknown top-level field", `"productTarget":"1.0"`, `"productTarget":"1.0","unknown":true`},
		{"unknown revision field", `"opening":"initialize"`, `"opening":"initialize","unknown":true`},
		{"wrong schema", `flashgate-mcp-protocol-matrix/v1`, `other/v1`},
		{"wrong product target", `"productTarget":"1.0"`, `"productTarget":"2.0"`},
		{"missing revisions", `,"revisions":[{"protocolVersion":"2025-11-25","opening":"initialize","unsupportedVersionBehavior":"respond-with-supported-revision","extensions":[],"transports":["stdio"]}]`, ``},
		{"zero revisions", `"revisions":[{"protocolVersion":"2025-11-25","opening":"initialize","unsupportedVersionBehavior":"respond-with-supported-revision","extensions":[],"transports":["stdio"]}]`, `"revisions":[]`},
		{"wrong revision", `"protocolVersion":"2025-11-25"`, `"protocolVersion":"2026-07-28"`},
		{"extra extension", `"extensions":[]`, `"extensions":["example/extension"]`},
		{"wrong transport", `"transports":["stdio"]`, `"transports":["http"]`},
		{"wrong opening", `"opening":"initialize"`, `"opening":"server/discover"`},
		{"wrong unsupported behavior", `"unsupportedVersionBehavior":"respond-with-supported-revision"`, `"unsupportedVersionBehavior":"error"`},
		{"null extensions", `"extensions":[]`, `"extensions":null`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mutated := strings.Replace(valid, tc.old, tc.replacement, 1)
			if mutated == valid {
				t.Fatal("fixture mutation had no effect")
			}
			if err := validateProtocolMatrix([]byte(mutated)); err == nil {
				t.Fatal("unsupported matrix was accepted")
			}
		})
	}
	for name, data := range map[string][]byte{
		"invalid UTF-8":      append([]byte(valid), 0xff),
		"null revisions":     []byte(`{"schemaVersion":"flashgate-mcp-protocol-matrix/v1","productTarget":"1.0","revisions":null}`),
		"trailing JSON":      []byte(valid + ` {}`),
		"duplicate revision": []byte(strings.Replace(valid, `}]}`, `},{"protocolVersion":"2025-11-25","opening":"initialize","unsupportedVersionBehavior":"respond-with-supported-revision","extensions":[],"transports":["stdio"]}]}`, 1)),
		"multiple revisions": []byte(strings.Replace(valid, `}]}`, `},{"protocolVersion":"2026-07-28","opening":"server/discover","unsupportedVersionBehavior":"error","extensions":[],"transports":["stdio"]}]}`, 1)),
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateProtocolMatrix(data); err == nil {
				t.Fatal("unsupported matrix was accepted")
			}
		})
	}
}
