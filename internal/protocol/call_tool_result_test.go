package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestNewTextContent(t *testing.T) {
	content := NewTextContent("hello")
	if content.Type != "text" || content.Text != "hello" {
		t.Fatalf("unexpected text content: %#v", content)
	}
	encoded, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"type":"text","text":"hello"}` {
		t.Fatalf("unexpected text content JSON: %s", encoded)
	}
}

func TestNewCallToolResultMarshal(t *testing.T) {
	structured := json.RawMessage(`{"path":"Unicode-ÄÖÜ\\grüße.txt","content":"line 1\nline 2"}`)
	result, err := NewCallToolResult(structured)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content == nil || len(result.Content) != 1 {
		t.Fatalf("expected one non-nil content block, got %#v", result.Content)
	}
	if result.IsError {
		t.Fatal("successful result must not be an error")
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["isError"]; ok {
		t.Fatal("isError=false must be omitted")
	}
	var textValue, structuredValue any
	if err := json.Unmarshal([]byte(result.Content[0].(TextContent).Text), &textValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(result.StructuredContent, &structuredValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(textValue, structuredValue) {
		t.Fatalf("text and structured values differ: %#v != %#v", textValue, structuredValue)
	}
}

func TestCallToolResultIsErrorMarshal(t *testing.T) {
	result, err := NewCallToolResult(json.RawMessage(`{"message":"failed"}`))
	if err != nil {
		t.Fatal(err)
	}
	result.IsError = true
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if string(fields["isError"]) != "true" {
		t.Fatalf("expected isError=true, got %s", fields["isError"])
	}
}

func TestNewCallToolResultCompactsAndOwnsStructuredContent(t *testing.T) {
	input := json.RawMessage(" \n { \"empty\" : {} } \t")
	result, err := NewCallToolResult(input)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = 'x'

	if result.Content[0].(TextContent).Text != `{"empty":{}}` || string(result.StructuredContent) != `{"empty":{}}` {
		t.Fatalf("unexpected compact result: %#v", result)
	}
}

func TestNewCallToolResultRejectsNonObjectJSON(t *testing.T) {
	for _, raw := range []json.RawMessage{
		nil,
		json.RawMessage(``),
		json.RawMessage(`null`),
		json.RawMessage(`[]`),
		json.RawMessage(`"text"`),
		json.RawMessage(`1`),
		json.RawMessage(`true`),
		json.RawMessage(`{"invalid":`),
	} {
		if result, err := NewCallToolResult(raw); err == nil {
			t.Fatalf("expected %q to be rejected, got %#v", raw, result)
		}
	}
}
