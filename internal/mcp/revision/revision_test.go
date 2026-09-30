package revision

import (
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestInspectRequiresValidPerRequestMetadata(t *testing.T) {
	bad := []string{
		`{"_meta":null}`,
		`{"_meta":[]}`,
		`{"_meta":{"io.modelcontextprotocol/clientCapabilities":{}}}`,
		`{"_meta":{"io.modelcontextprotocol/protocolVersion":1,"io.modelcontextprotocol/clientCapabilities":{}}}`,
		`{"_meta":{"io.modelcontextprotocol/protocolVersion":null,"io.modelcontextprotocol/clientCapabilities":{}}}`,
		`{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}`,
		`{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":[]}}`,
		`{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":[],"io.modelcontextprotocol/clientInfo":{}}}`,
		`{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{},"io.modelcontextprotocol/clientInfo":{"name":1,"version":"1"}}}`,
	}
	for _, raw := range bad {
		_, rpcErr := Inspect(json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
			t.Fatalf("expected invalid params for %s: %#v", raw, rpcErr)
		}
	}
	valid := `{"name":"read_file","_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{},"io.modelcontextprotocol/clientInfo":{"name":"client","version":"1"}}}`
	request, rpcErr := Inspect(json.RawMessage(valid))
	if rpcErr != nil || request.Version != Stateless || string(request.Params) != `{"name":"read_file"}` {
		t.Fatalf("unexpected stateless request: %#v %#v", request, rpcErr)
	}
	for _, version := range []string{"2025-11-25", "future", "draft-x", "2026-99-99", "unknown-revision", ""} {
		encoded, _ := json.Marshal(map[string]any{"_meta": map[string]any{
			protocolVersionKey: version, clientCapabilitiesKey: map[string]any{},
		}})
		request, rpcErr := Inspect(encoded)
		if rpcErr != nil || !request.StatelessEnvelope || request.Version != version {
			t.Fatalf("string version %q was rejected before policy evaluation: %#v %#v", version, request, rpcErr)
		}
	}
}

func TestUnsupportedContainsOnlyStatelessSelectableRevisions(t *testing.T) {
	for _, policy := range []Policy{ProductionPolicy(), NewPolicy(protocol.ProtocolVersion, Stateless)} {
		for _, requested := range []string{"2025-11-25", "future", "draft-x", "2026-99-99"} {
			rpcErr := Unsupported(requested, policy)
			if rpcErr.Code != protocol.ErrUnsupportedProtocolVersion {
				t.Fatalf("unexpected error code: %#v", rpcErr)
			}
			var data struct {
				Requested string   `json:"requested"`
				Supported []string `json:"supported"`
			}
			if err := json.Unmarshal(rpcErr.Data, &data); err != nil || data.Requested != requested {
				t.Fatalf("unexpected unsupported data: %s: %v", rpcErr.Data, err)
			}
			want := 0
			if policy.Enabled(Stateless) {
				want = 1
			}
			if len(data.Supported) != want || (want == 1 && data.Supported[0] != Stateless) {
				t.Fatalf("legacy revision leaked into stateless supported set: %s", rpcErr.Data)
			}
		}
	}
}

func TestCompleteDoesNotMutateDomainResult(t *testing.T) {
	domain := map[string]any{"tools": []any{}}
	result, rpcErr := Complete(domain, "tools/list", "flashgate", "test")
	if rpcErr != nil || len(domain) != 1 {
		t.Fatalf("domain result mutated or failed: %#v %#v", domain, rpcErr)
	}
	wire := result.(map[string]json.RawMessage)
	if string(wire["resultType"]) != `"complete"` || string(wire["ttlMs"]) != `0` || string(wire["cacheScope"]) != `"private"` {
		t.Fatalf("missing stateless fields: %#v", wire)
	}
}
