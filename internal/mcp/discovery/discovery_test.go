package discovery

import (
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func TestDiscoverResultUsesOnlyStatelessEntryVersions(t *testing.T) {
	policy := revision.NewPolicy(protocol.ProtocolVersion, revision.Stateless)
	result := Result(policy)
	wire, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(wire, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields["supportedVersions"] == nil || fields["capabilities"] == nil || fields["supportedProtocolVersions"] != nil || fields["extensions"] != nil {
		t.Fatalf("unexpected discovery fields: %s", wire)
	}
	var versions []string
	if err := json.Unmarshal(fields["supportedVersions"], &versions); err != nil || len(versions) != 1 || versions[0] != revision.Stateless {
		t.Fatalf("unexpected stateless versions: %s: %v", wire, err)
	}
	var capabilities map[string]json.RawMessage
	if err := json.Unmarshal(fields["capabilities"], &capabilities); err != nil || len(capabilities) != 1 || string(capabilities["tools"]) != `{}` || capabilities["extensions"] != nil {
		t.Fatalf("unexpected capabilities: %s: %v", wire, err)
	}
}

func TestDiscoverResultCompleteWireMetadata(t *testing.T) {
	result, rpcErr := revision.Complete(Result(revision.NewPolicy(protocol.ProtocolVersion, revision.Stateless)), "server/discover", "flashgate", "test-version")
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	wire, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(wire, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 6 || string(fields["resultType"]) != `"complete"` || string(fields["ttlMs"]) != `0` || string(fields["cacheScope"]) != `"private"` {
		t.Fatalf("unexpected completed discovery: %s", wire)
	}
	var meta map[string]json.RawMessage
	if err := json.Unmarshal(fields["_meta"], &meta); err != nil || len(meta) != 1 {
		t.Fatalf("unexpected result metadata: %s: %v", wire, err)
	}
	var info map[string]string
	if err := json.Unmarshal(meta["io.modelcontextprotocol/serverInfo"], &info); err != nil || len(info) != 2 || info["name"] != "flashgate" || info["version"] != "test-version" {
		t.Fatalf("unexpected server identity: %s: %v", wire, err)
	}
}
