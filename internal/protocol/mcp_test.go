package protocol

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProtocolVersionIsSet(t *testing.T) {
	t.Parallel()

	if ProtocolVersion == "" {
		t.Fatal("expected protocol version to be set")
	}
}

func TestProtocolVersionMatchesReleasedMatrix(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "mcp-protocol-matrix.json"))
	if err != nil {
		t.Fatal(err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var matrix struct {
		SchemaVersion string `json:"schemaVersion"`
		ProductTarget string `json:"productTarget"`
		Revisions     []struct {
			ProtocolVersion string   `json:"protocolVersion"`
			Status          string   `json:"status"`
			Extensions      []string `json:"extensions"`
			Transports      []string `json:"transports"`
		} `json:"revisions"`
		UnsupportedRevisionPolicy string `json:"unsupportedRevisionPolicy"`
		ExtensionPolicy           string `json:"extensionPolicy"`
	}
	if err := decoder.Decode(&matrix); err != nil {
		t.Fatalf("decode protocol matrix: %v", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("protocol matrix contains trailing JSON data: %v", err)
	}

	if matrix.SchemaVersion != "flashgate-mcp-protocol-matrix/v1" || matrix.ProductTarget != "1.0" {
		t.Fatalf("unexpected matrix identity: schema=%q target=%q", matrix.SchemaVersion, matrix.ProductTarget)
	}
	if len(matrix.Revisions) != 1 {
		t.Fatalf("matrix advertises %d revisions, want exactly one", len(matrix.Revisions))
	}
	revision := matrix.Revisions[0]
	if revision.ProtocolVersion != ProtocolVersion || revision.Status != "supported" {
		t.Fatalf("matrix revision=%#v does not match runtime version %q", revision, ProtocolVersion)
	}
	if len(revision.Extensions) != 0 || !reflect.DeepEqual(revision.Transports, []string{"stdio"}) {
		t.Fatalf("matrix advertises unimplemented capabilities: %#v", revision)
	}
	if matrix.UnsupportedRevisionPolicy != "respond-with-supported-version" || matrix.ExtensionPolicy != "none-advertised" {
		t.Fatalf("unexpected compatibility policy: %#v", matrix)
	}
}
