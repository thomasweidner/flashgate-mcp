package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	processdomain "github.com/thomasweidner/flashgate-mcp/internal/process"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type fakeTreeObserver struct {
	snapshot processdomain.TreeSnapshot
	err      error
	calls    int
}

func (f *fakeTreeObserver) Tree(context.Context) (processdomain.TreeSnapshot, error) {
	f.calls++
	return f.snapshot, f.err
}

func TestGetProcessTreeReturnsBoundedDeterministicDescendants(t *testing.T) {
	observer := &fakeTreeObserver{snapshot: processdomain.TreeSnapshot{Processes: []processdomain.Details{
		{PID: 9, ParentPID: 2, Name: "grandchild"}, {PID: 5, ParentPID: 1, Name: "later"},
		{PID: 1, ParentPID: 0, Name: "root"}, {PID: 2, ParentPID: 1, Name: "first"},
	}}}
	value, rpcErr := NewGetProcessTreeTool(observer).Execute(t.Context(), json.RawMessage(`{"pid":1,"maxDepth":1}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	got := value.(getProcessTreeResult)
	if len(got.Processes) != 3 || got.Processes[0].PID != 1 || got.Processes[1].PID != 2 || got.Processes[2].PID != 5 || !got.Truncated || got.Partial {
		t.Fatalf("unexpected tree: %+v", got)
	}
}

func TestGetProcessTreeRejectsInvalidInputBeforeObservation(t *testing.T) {
	for _, raw := range []string{`{"pid":0}`, `{"pid":1,"maxDepth":-1}`, `{"pid":1,"maxDepth":9}`, `{"pid":1,"extra":true}`} {
		observer := &fakeTreeObserver{}
		_, rpcErr := NewGetProcessTreeTool(observer).Execute(t.Context(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams || observer.calls != 0 {
			t.Fatalf("%s: error=%+v calls=%d", raw, rpcErr, observer.calls)
		}
	}
}

func TestGetProcessTreeMapsSnapshotFailuresSafely(t *testing.T) {
	observer := &fakeTreeObserver{err: errors.New("/private/host/path")}
	_, rpcErr := NewGetProcessTreeTool(observer).Execute(t.Context(), json.RawMessage(`{"pid":1}`))
	if rpcErr == nil || rpcErr.Message != "process observation failed" {
		t.Fatalf("unexpected error: %+v", rpcErr)
	}
	observer = &fakeTreeObserver{snapshot: processdomain.TreeSnapshot{Processes: []processdomain.Details{{PID: 2, Name: "other"}}}}
	_, rpcErr = NewGetProcessTreeTool(observer).Execute(t.Context(), json.RawMessage(`{"pid":1}`))
	if rpcErr == nil || rpcErr.Message != "process not found" {
		t.Fatalf("unexpected missing error: %+v", rpcErr)
	}
}

func TestGetProcessTreeDefinitionIsClosedAndBounded(t *testing.T) {
	definition := NewGetProcessTreeTool(&fakeTreeObserver{}).Definition()
	input := definition.InputSchema.(map[string]any)
	depth := input["properties"].(map[string]any)["maxDepth"].(map[string]any)
	if input["additionalProperties"] != false || depth["maximum"] != maximumProcessTreeDepth || definition.OutputSchema == nil {
		t.Fatalf("unexpected definition: %+v", definition)
	}
}

func TestGetProcessTreeEnforcesNodeLimitAndReportsPartialSnapshot(t *testing.T) {
	processes := []processdomain.Details{{PID: 1, Name: "root"}}
	for pid := uint32(2); pid <= maximumProcessTreeNodes+10; pid++ {
		processes = append(processes, processdomain.Details{PID: pid, ParentPID: 1, Name: "child"})
	}
	observer := &fakeTreeObserver{snapshot: processdomain.TreeSnapshot{Processes: processes, Partial: true}}
	value, rpcErr := NewGetProcessTreeTool(observer).Execute(t.Context(), json.RawMessage(`{"pid":1,"maxDepth":1}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	got := value.(getProcessTreeResult)
	if len(got.Processes) != maximumProcessTreeNodes || !got.Truncated || !got.Partial {
		t.Fatalf("expected bounded partial result, got %+v", got)
	}
}
