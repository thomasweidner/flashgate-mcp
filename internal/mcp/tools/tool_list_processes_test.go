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
	if !first.More || first.NextCursor == "" || !reflect.DeepEqual(first.Processes, selectedProcessEntries(lister.entries[:2], true, true)) {
		t.Fatalf("unexpected first page: %+v", first)
	}

	arguments, _ := json.Marshal(map[string]any{"pageSize": 2, "cursor": first.NextCursor})
	result, rpcErr = tool.Execute(context.Background(), arguments)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	second := result.(listProcessesResult)
	if second.More || second.NextCursor != "" || !reflect.DeepEqual(second.Processes, selectedProcessEntries(lister.entries[2:], true, true)) {
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
	fields := properties["fields"].(map[string]any)
	if fields["minItems"] != 1 || fields["maxItems"] != 2 || fields["uniqueItems"] != true {
		t.Fatalf("unexpected fields schema: %#v", fields)
	}
}

func TestListProcessesReturnsOnlySelectedFields(t *testing.T) {
	for _, tc := range []struct {
		name      string
		arguments string
		wantPID   bool
		wantName  bool
	}{
		{name: "pid only", arguments: `{"fields":["pid"]}`, wantPID: true},
		{name: "name only", arguments: `{"fields":["name"]}`, wantName: true},
		{name: "explicit both", arguments: `{"fields":["name","pid"]}`, wantPID: true, wantName: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := []processdomain.Entry{{PID: 7, Name: "seven"}}
			result, rpcErr := NewListProcessesTool(&fakeProcessLister{entries: entries}).Execute(context.Background(), json.RawMessage(tc.arguments))
			if rpcErr != nil {
				t.Fatal(rpcErr)
			}
			got := result.(listProcessesResult).Processes
			if !reflect.DeepEqual(got, selectedProcessEntries(entries, tc.wantPID, tc.wantName)) {
				t.Fatalf("unexpected selected fields: %#v", got)
			}
			encoded, err := json.Marshal(got[0])
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if _, ok := fields["pid"]; ok != tc.wantPID {
				t.Fatalf("pid presence=%v, want %v: %s", ok, tc.wantPID, encoded)
			}
			if _, ok := fields["name"]; ok != tc.wantName {
				t.Fatalf("name presence=%v, want %v: %s", ok, tc.wantName, encoded)
			}
		})
	}
}

func TestListProcessesRejectsInvalidInputsBeforeObservation(t *testing.T) {
	for _, arguments := range []string{`{"pageSize":0}`, `{"pageSize":201}`, `{"cursor":"not-a-cursor"}`, `{"fields":[]}`, `{"fields":["pid","pid"]}`, `{"fields":["commandLine"]}`, `{"unknown":true}`} {
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

func selectedProcessEntries(entries []processdomain.Entry, includePID, includeName bool) []processListEntry {
	result := make([]processListEntry, 0, len(entries))
	for _, entry := range entries {
		item := processListEntry{}
		if includePID {
			pid := entry.PID
			item.PID = &pid
		}
		if includeName {
			name := entry.Name
			item.Name = &name
		}
		result = append(result, item)
	}
	return result
}
