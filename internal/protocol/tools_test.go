package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestToolMarshal(t *testing.T) {
	t.Parallel()

	tool := Tool{
		Name:        "list_directory",
		Title:       "List Files",
		Description: "Lists files and directories.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"path"},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"entries": map[string]any{"type": "array"},
			},
			"required":             []string{"entries"},
			"additionalProperties": false,
		},
	}

	encoded, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}

	if decoded["name"] != "list_directory" {
		t.Fatalf("expected name %q, got %v", "list_directory", decoded["name"])
	}

	if decoded["title"] != "List Files" {
		t.Fatalf("expected title %q, got %v", "List Files", decoded["title"])
	}

	if decoded["description"] != "Lists files and directories." {
		t.Fatalf("unexpected description: %v", decoded["description"])
	}

	if _, ok := decoded["inputSchema"].(map[string]any); !ok {
		t.Fatalf("expected inputSchema object, got %#v", decoded["inputSchema"])
	}

	if _, ok := decoded["outputSchema"].(map[string]any); !ok {
		t.Fatalf("expected outputSchema object, got %#v", decoded["outputSchema"])
	}

	var roundTrip Tool
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("expected outputSchema to unmarshal, got %v", err)
	}
	if !reflect.DeepEqual(normalizeJSONValue(t, tool.OutputSchema), normalizeJSONValue(t, roundTrip.OutputSchema)) {
		t.Fatalf("outputSchema changed after round trip: got %#v want %#v", roundTrip.OutputSchema, tool.OutputSchema)
	}
}

func TestToolMarshalOmitsEmptyTitle(t *testing.T) {
	t.Parallel()

	tool := Tool{
		Name:        "list_directory",
		Description: "Lists files and directories.",
		InputSchema: map[string]any{
			"type": "object",
		},
	}

	encoded, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("expected valid json, got %v", err)
	}

	if _, exists := decoded["title"]; exists {
		t.Fatal("did not expect title field")
	}

	if _, exists := decoded["outputSchema"]; exists {
		t.Fatal("did not expect outputSchema field")
	}
}

func TestNewCallToolErrorResult(t *testing.T) {
	t.Parallel()

	result, err := NewCallToolErrorResult("not_found", "filesystem error: not found")
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatal("expected isError=true")
	}
	want := `{"category":"not_found","message":"filesystem error: not found"}`
	if string(result.StructuredContent) != want || len(result.Content) != 1 || result.Content[0].Text != want {
		t.Fatalf("unexpected normalized tool error: %#v", result)
	}
}

func normalizeJSONValue(t *testing.T, value any) any {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		t.Fatal(err)
	}
	return normalized
}
