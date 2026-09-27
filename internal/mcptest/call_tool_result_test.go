package mcptest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodeCallToolResultAcceptsStructuredTextResult(t *testing.T) {
	for _, test := range []struct {
		raw     json.RawMessage
		isError bool
	}{
		{raw: json.RawMessage(`{"content":[{"type":"text","text":"{}"}],"structuredContent":{}}`)},
		{raw: json.RawMessage(`{"content":[{"type":"text","text":"{\"category\":\"invalid_path\",\"message\":\"failed\"}"}],"isError":true}`), isError: true},
	} {
		decoded, err := DecodeCallToolResult(test.raw)
		if err != nil {
			t.Fatalf("expected valid CallToolResult, got %v", err)
		}
		if decoded.HasStructuredContent == test.isError || decoded.IsError != test.isError {
			t.Fatalf("unexpected decoded result: %#v", decoded)
		}
	}
}

func TestDecodeCallToolResultRejectsLegacyUnwrappedCaptures(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "legacy-unwrapped-call-tool-results.json"))
	if err != nil {
		t.Fatal(err)
	}
	var captures []json.RawMessage
	if err := json.Unmarshal(raw, &captures); err != nil {
		t.Fatal(err)
	}
	for _, capture := range captures {
		if _, err := DecodeCallToolResult(capture); err == nil {
			t.Fatalf("expected legacy capture to be rejected: %s", capture)
		}
	}
}

func TestDecodeCallToolResultRejectsInvalidForms(t *testing.T) {
	tests := []string{
		`{"content":[{"type":"text","text":"{}"}],"structuredContent":{},"isError":true}`,
		`{"content":[{"type":"text","text":"{}"}],"isError":true}`,
		`{"content":[{"type":"text","text":"{}"}],"isError":null}`,
		`{"content":[{"type":"text","text":"{\"category\":\"x\",\"message\":\"m\"}"}],"isError":false}`,

		`[]`,
		`{}`,
		`{"content":{},"structuredContent":{}}`,
		`{"content":null,"structuredContent":{}}`,
		`{"content":[],"structuredContent":{}}`,
		`{"content":[{"type":"text","text":"{}"},{"type":"text","text":"{}"}],"structuredContent":{}}`,
		`{"content":[{"text":"{}"}],"structuredContent":{}}`,
		`{"content":[{"type":"text"}],"structuredContent":{}}`,
		`{"content":[{"type":"image","text":"{}"}],"structuredContent":{}}`,
		`{"content":[{"type":"text","text":1}],"structuredContent":{}}`,
		`{"content":[{"type":"text","text":"{}","extra":true}],"structuredContent":{}}`,
		`{"content":[{"type":"text","text":"not-json"}],"structuredContent":{}}`,
		`{"content":[{"type":"text","text":"[]"}],"structuredContent":[]}`,
		`{"content":[{"type":"text","text":"null"}],"structuredContent":null}`,
		`{"content":[{"type":"text","text":"{}"}]}`,
		`{"content":[{"type":"text","text":"{}"}],"structuredContent":null}`,
		`{"content":[{"type":"text","text":"{}"}],"structuredContent":[]}`,
		`{"content":[{"type":"text","text":"{}"}],"structuredContent":"{}"}`,
		`{"content":[{"type":"text","text":"{}"}],"structuredContent":{"different":true}}`,
		`{"content":[{"type":"text","text":"{}"}],"structuredContent":{},"isError":"false"}`,
		`{"content":[{"type":"text","text":"{}"}],"structuredContent":{},"isError":0}`,
		`{"content":[{"type":"text","text":"{}"}],"structuredContent":{},"entries":[]}`,
		`{"content":[{"type":"text","text":"{}"}],"structuredContent":{},"_meta":{}}`,
	}
	for _, raw := range tests {
		if _, err := DecodeCallToolResult(json.RawMessage(raw)); err == nil {
			t.Fatalf("expected invalid result to be rejected: %s", raw)
		}
	}
}
