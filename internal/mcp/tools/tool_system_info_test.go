package tools

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/systeminfo"
)

type fakeSystemInfoProvider struct {
	info systeminfo.Info
	err  error
}

func (provider fakeSystemInfoProvider) Info() (systeminfo.Info, error) {
	return provider.info, provider.err
}

func TestSystemInfoReturnsOnlyReleasedFacts(t *testing.T) {
	provider := fakeSystemInfoProvider{info: systeminfo.Info{OS: "linux", Architecture: "amd64", Version: "6.1.0"}}
	result, rpcErr := NewSystemInfoTool(provider).Execute(context.Background(), json.RawMessage(`{}`))
	want := map[string]string{"os": "linux", "architecture": "amd64", "version": "6.1.0"}
	if rpcErr != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result=%#v error=%#v", result, rpcErr)
	}
}

func TestSystemInfoReturnsOnlySelectedFields(t *testing.T) {
	provider := fakeSystemInfoProvider{info: systeminfo.Info{OS: "linux", Architecture: "amd64", Version: "6.1.0"}}
	result, rpcErr := NewSystemInfoTool(provider).Execute(context.Background(), json.RawMessage(`{"fields":["version","os"]}`))
	want := map[string]string{"os": "linux", "version": "6.1.0"}
	if rpcErr != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result=%#v error=%#v", result, rpcErr)
	}
}

func TestSystemInfoDoesNotRequireUnselectedFields(t *testing.T) {
	provider := fakeSystemInfoProvider{info: systeminfo.Info{OS: "linux"}}
	result, rpcErr := NewSystemInfoTool(provider).Execute(context.Background(), json.RawMessage(`{"fields":["os"]}`))
	want := map[string]string{"os": "linux"}
	if rpcErr != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("unexpected result=%#v error=%#v", result, rpcErr)
	}
}

func TestSystemInfoDefinition(t *testing.T) {
	definition := NewSystemInfoTool(fakeSystemInfoProvider{}).Definition()
	if definition.Name != "system_info" || definition.Title != "System Info" || definition.Description == "" || definition.OutputSchema == nil {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestSystemInfoRejectsArguments(t *testing.T) {
	for _, raw := range []string{
		`{"hostname":true}`,
		`{"fields":[]}`,
		`{"fields":["hostname"]}`,
		`{"fields":["os","os"]}`,
		`{"fields":null}`,
	} {
		_, rpcErr := NewSystemInfoTool(fakeSystemInfoProvider{}).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("%s: expected invalid params, got %#v", raw, rpcErr)
		}
	}
}

func TestSystemInfoFailsClosed(t *testing.T) {
	cases := []fakeSystemInfoProvider{
		{err: errors.New("unavailable")},
		{info: systeminfo.Info{OS: "linux", Architecture: "amd64"}},
	}
	for _, provider := range cases {
		result, rpcErr := NewSystemInfoTool(provider).Execute(context.Background(), json.RawMessage(`{}`))
		if result != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInternalError || rpcErr.Message != "internal error" {
			t.Fatalf("expected redacted internal error, result=%#v error=%#v", result, rpcErr)
		}
	}
}
