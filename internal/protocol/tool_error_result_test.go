package protocol

import (
	"encoding/json"
	"testing"
)

func TestCallToolErrorResultWireContract(t *testing.T) {
	result, err := NewCallToolErrorResult("invalid_path", "invalid path")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"content":[{"type":"text","text":"{\"category\":\"invalid_path\",\"message\":\"invalid path\"}"}],"isError":true}`
	if string(raw) != want {
		t.Fatalf("got %s want %s", raw, want)
	}
	for _, input := range [][2]string{{"", "message"}, {"category", ""}, {" ", "message"}, {"category", " \t"}} {
		if _, err := NewCallToolErrorResult(input[0], input[1]); err == nil {
			t.Fatal("expected invariant rejection")
		}
	}
}
