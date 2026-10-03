package benchmark

import (
	"strings"
	"testing"
)

func TestDecodeStrictJSONRejectsMalformedUnicode(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "invalid raw UTF-8 in nested value", data: []byte{'{', '"', 'v', 'a', 'l', 'u', 'e', '"', ':', '"', 0xff, '"', '}'}},
		{name: "unpaired high surrogate in property", data: []byte(`{"\uD800":"value"}`)},
		{name: "unpaired high surrogate in nested value", data: []byte(`{"value":"\uD800"}`)},
		{name: "unpaired low surrogate in array", data: []byte(`{"values":["\uDC00"]}`)},
		{name: "high surrogate followed by non-low surrogate", data: []byte(`{"value":"\uD800\u0041"}`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var destination any
			err := decodeStrictJSON(tc.data, &destination)
			if err == nil || !strings.Contains(err.Error(), "invalid JSON Unicode") {
				t.Fatalf("decodeStrictJSON error=%v, want invalid JSON Unicode", err)
			}
		})
	}
}

func TestDecodeStrictJSONAcceptsValidUnicode(t *testing.T) {
	tests := [][]byte{
		[]byte(`{"value":"replacement: �"}`),
		[]byte(`{"value":"escaped replacement: \uFFFD"}`),
		[]byte(`{"value":"supplementary: \uD83D\uDE80"}`),
		[]byte(`{"value":"literal escape: \\uD800"}`),
	}
	for _, data := range tests {
		var destination any
		if err := decodeStrictJSON(data, &destination); err != nil {
			t.Fatalf("decodeStrictJSON(%s): %v", data, err)
		}
	}
}
