package tools

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	processdomain "github.com/thomasweidner/flashgate-mcp/internal/process"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type fakeProcessLister struct {
	entries []processdomain.Entry
	calls   int
}

func (f *fakeProcessLister) List(context.Context) ([]processdomain.Entry, error) {
	f.calls++
	return append([]processdomain.Entry(nil), f.entries...), nil
}

func TestListProcessesPaginatesWithOpaqueCursor(t *testing.T) {
	lister := &fakeProcessLister{entries: []processdomain.Entry{{PID: 1, Name: "one"}, {PID: 4, Name: "four"}, {PID: 9, Name: "nine"}}}
	tool := NewListProcessesTool(lister)
	result, rpcErr := tool.Execute(context.Background(), json.RawMessage(`{"pageSize":2}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	first := result.(listProcessesResult)
	if !first.More || first.NextCursor == "" || !reflect.DeepEqual(first.Processes, lister.entries[:2]) {
		t.Fatalf("unexpected first page: %+v", first)
	}

	arguments, _ := json.Marshal(map[string]any{"pageSize": 2, "cursor": first.NextCursor})
	result, rpcErr = tool.Execute(context.Background(), arguments)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	second := result.(listProcessesResult)
	if second.More || second.NextCursor != "" || !reflect.DeepEqual(second.Processes, lister.entries[2:]) {
		t.Fatalf("unexpected second page: %+v", second)
	}
}

func TestListProcessesDefinitionHasBoundedSchemas(t *testing.T) {
	definition := NewListProcessesTool(&fakeProcessLister{}).Definition()
	if definition.Name != "list_processes" || definition.OutputSchema == nil {
		t.Fatalf("unexpected definition: %+v", definition)
	}
	input := definition.InputSchema.(map[string]any)
	properties := input["properties"].(map[string]any)
	pageSize := properties["pageSize"].(map[string]any)
	if pageSize["maximum"] != maximumProcessPageSize {
		t.Fatalf("pageSize maximum=%v", pageSize["maximum"])
	}
}

func TestListProcessesRejectsInvalidInputsBeforeObservation(t *testing.T) {
	for _, arguments := range []string{`{"pageSize":0}`, `{"pageSize":201}`, `{"cursor":"not-a-cursor"}`, `{"unknown":true}`} {
		lister := &fakeProcessLister{}
		_, rpcErr := NewListProcessesTool(lister).Execute(context.Background(), json.RawMessage(arguments))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("arguments %s: expected invalid params, got %+v", arguments, rpcErr)
		}
		if lister.calls != 0 {
			t.Fatalf("arguments %s reached process observation", arguments)
		}
	}
}
