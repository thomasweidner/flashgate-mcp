package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	processdomain "github.com/thomasweidner/flashgate-mcp/internal/process"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type fakeProcessDetailer struct {
	details processdomain.Details
	err     error
	calls   int
}

func (f *fakeProcessDetailer) Details(context.Context, uint32) (processdomain.Details, error) {
	f.calls++
	return f.details, f.err
}

func TestGetProcessDetailsReturnsOnlySelectedFields(t *testing.T) {
	detailer := &fakeProcessDetailer{details: processdomain.Details{PID: 42, Name: "worker", ParentPID: 7, ThreadCount: 3}}
	result, rpcErr := NewGetProcessDetailsTool(detailer).Execute(context.Background(), json.RawMessage(`{"pid":42,"fields":["name","threadCount"]}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	got := result.(getProcessDetailsResult)
	if got.PID != 42 || got.Name == nil || *got.Name != "worker" || got.ThreadCount == nil || *got.ThreadCount != 3 || got.ParentPID != nil {
		t.Fatalf("unexpected selected details: %+v", got)
	}
}

func TestGetProcessDetailsRejectsInvalidInputsBeforeObservation(t *testing.T) {
	for _, arguments := range []string{
		`{"pid":0,"fields":["name"]}`,
		`{"pid":1,"fields":[]}`,
		`{"pid":1,"fields":["commandLine"]}`,
		`{"pid":1,"fields":["name","name"]}`,
		`{"pid":1,"fields":["name"],"unknown":true}`,
	} {
		detailer := &fakeProcessDetailer{}
		_, rpcErr := NewGetProcessDetailsTool(detailer).Execute(context.Background(), json.RawMessage(arguments))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("arguments %s: expected invalid params, got %+v", arguments, rpcErr)
		}
		if detailer.calls != 0 {
			t.Fatalf("arguments %s reached process observation", arguments)
		}
	}
}

func TestGetProcessDetailsMapsAccessErrorsWithoutPlatformDetails(t *testing.T) {
	for _, tc := range []struct {
		err     error
		message string
	}{
		{processdomain.ErrNotFound, "process not found"},
		{processdomain.ErrAccessDenied, "process access denied"},
		{errors.New("/private/host/path"), "process observation failed"},
	} {
		detailer := &fakeProcessDetailer{err: tc.err}
		_, rpcErr := NewGetProcessDetailsTool(detailer).Execute(context.Background(), json.RawMessage(`{"pid":1,"fields":["name"]}`))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInternalError || rpcErr.Message != tc.message {
			t.Fatalf("error %v: got %+v", tc.err, rpcErr)
		}
	}
}

func TestGetProcessDetailsDefinitionHasClosedBoundedSchemas(t *testing.T) {
	definition := NewGetProcessDetailsTool(&fakeProcessDetailer{}).Definition()
	input := definition.InputSchema.(map[string]any)
	if input["additionalProperties"] != false || definition.OutputSchema == nil {
		t.Fatalf("unexpected definition: %+v", definition)
	}
	fields := input["properties"].(map[string]any)["fields"].(map[string]any)
	if fields["maxItems"] != len(processDetailFields) || fields["uniqueItems"] != true {
		t.Fatalf("fields schema is not bounded and unique: %+v", fields)
	}
}
