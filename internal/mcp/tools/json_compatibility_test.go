package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestJSONAdmissionUTF8ByteBoundaries(t *testing.T) {
	values := []string{"", "\ufffd", "\u2028\u2029", "🙂", "\xc0\xaf", "\xed\xa0\x80", "\xf4\x90\x80\x80", "\xe2\x82", "\xff\xfe"}
	for b := 0; b < 256; b++ {
		values = append(values, "before"+string([]byte{byte(b)})+"after")
	}
	for _, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		remaining := len(encoded)
		if !consumeResultString(&remaining, value) || remaining != 0 {
			t.Fatalf("string %x: exact admission remaining=%d, encoded=%q", value, remaining, encoded)
		}
		remaining = len(encoded) - 1
		if consumeResultString(&remaining, value) {
			t.Fatalf("string %x admitted below encoded size", value)
		}
		for _, created := range []bool{false, true} {
			result := NewToolResult(defaultVariant, createDirectoryResult{Path: value, Created: created})
			wrapped, err := metadataContract(defaultVariant).wrap(context.Background(), result)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := json.Marshal(wrapped)
			if err != nil {
				t.Fatal(err)
			}
			if mkdirReceiptSize(value, created) != int64(len(wire)) ||
				!wrappedResultFits(wrapped, int64(len(wire))) || wrappedResultFits(wrapped, int64(len(wire)-1)) {
				t.Fatalf("mkdir %x/%v: admission differs from %d wire bytes", value, created, len(wire))
			}
		}
	}
}

// Keep the historical benchmark's exact wire contract while removing work
// already performed by Marshal. The reference intentionally retains that work.
func TestStructuredSerializationMatchesCompactReference(t *testing.T) {
	values := []any{
		map[string]string{"path": "<>\"\\\b\f\n\r\t\x00\u2028\u2029\xff\ufffd🙂"},
		json.RawMessage(" { \"raw\" : [ 1, \"<>&\\u2028\" ] } \n"),
		map[string]string{"content": strings.Repeat("\xff\\\"\n", 4096)},
	}
	for _, fixture := range callToolResultBenchmarkFixtures() {
		values = append(values, fixture.value)
	}
	for _, value := range values {
		structured, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, structured); err != nil {
			t.Fatal(err)
		}
		reference, err := json.Marshal(struct {
			Content           []protocol.TextContent `json:"content"`
			StructuredContent json.RawMessage        `json:"structuredContent"`
		}{[]protocol.TextContent{protocol.NewTextContent(compact.String())}, compact.Bytes()})
		if err != nil {
			t.Fatal(err)
		}
		actual, err := serializeStructuredCallToolResult(value)
		if err != nil || !bytes.Equal(actual, reference) {
			t.Fatalf("serialization changed: error=%v, actual bytes=%d, reference bytes=%d", err, len(actual), len(reference))
		}
	}
	if _, err := serializeStructuredCallToolResult(json.RawMessage(`{"invalid":`)); err == nil {
		t.Fatal("invalid raw JSON accepted")
	}
}
