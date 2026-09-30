// Package revision owns MCP wire revision policy and stateless request metadata.
package revision

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const Stateless = "2026-07-28"

const (
	protocolVersionKey    = "io.modelcontextprotocol/protocolVersion"
	clientCapabilitiesKey = "io.modelcontextprotocol/clientCapabilities"
	clientInfoKey         = "io.modelcontextprotocol/clientInfo"
)

// Policy is an internal, immutable set of enabled exact MCP revisions.
// The production constructor deliberately enables only the released revision.
type Policy struct{ enabled map[string]bool }

func ProductionPolicy() Policy { return NewPolicy(protocol.ProtocolVersion) }

// NewPolicy permits explicit injection by production-code integration tests.
func NewPolicy(versions ...string) Policy {
	p := Policy{enabled: make(map[string]bool, len(versions))}
	for _, version := range versions {
		if version == protocol.ProtocolVersion || version == Stateless {
			p.enabled[version] = true
		}
	}
	return p
}

func (p Policy) Enabled(version string) bool { return p.enabled[version] }

func (p Policy) Supported() []string {
	versions := make([]string, 0, len(p.enabled))
	for version := range p.enabled {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	return versions
}

// StatelessSupported lists only revisions selectable through per-request
// metadata. The initialize-based legacy revision is not a stateless option.
func (p Policy) StatelessSupported() []string {
	if p.Enabled(Stateless) {
		return []string{Stateless}
	}
	return []string{}
}

// Request contains only adapter-owned data. Client claims never authorize tools.
type Request struct {
	Version           string
	Params            json.RawMessage
	StatelessEnvelope bool
	// Extensions is the exact intersection of client claims and enabled server
	// extensions. It is empty until a separate extension decision activates one.
	Extensions []string
}

// Inspect classifies each request independently and removes reserved metadata
// before invoking a domain-facing method handler.
func Inspect(params json.RawMessage) (Request, *protocol.Error) {
	legacy := Request{Version: protocol.ProtocolVersion, Params: params}
	trimmed := bytes.TrimSpace(params)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || trimmed[0] != '{' {
		return legacy, nil
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(trimmed, &fields) != nil {
		return legacy, nil
	}
	metaRaw, exists := fields["_meta"]
	if !exists {
		return legacy, nil
	}
	if len(bytes.TrimSpace(metaRaw)) == 0 || bytes.TrimSpace(metaRaw)[0] != '{' {
		return Request{}, invalidMetadata()
	}
	var meta map[string]json.RawMessage
	if json.Unmarshal(metaRaw, &meta) != nil {
		return Request{}, invalidMetadata()
	}
	_, hasVersion := meta[protocolVersionKey]
	_, hasCapabilities := meta[clientCapabilitiesKey]
	_, hasInfo := meta[clientInfoKey]
	if !hasVersion && !hasCapabilities && !hasInfo {
		return legacy, nil
	}
	var version string
	if !hasVersion || !stringValue(meta[protocolVersionKey], &version) || !hasCapabilities || !object(meta[clientCapabilitiesKey]) {
		return Request{}, invalidMetadata()
	}
	if hasInfo {
		if !object(meta[clientInfoKey]) {
			return Request{}, invalidMetadata()
		}
		var info map[string]json.RawMessage
		if json.Unmarshal(meta[clientInfoKey], &info) != nil || !nonemptyString(info["name"]) || !nonemptyString(info["version"]) {
			return Request{}, invalidMetadata()
		}
	}
	var capabilities map[string]json.RawMessage
	if json.Unmarshal(meta[clientCapabilitiesKey], &capabilities) != nil {
		return Request{}, invalidMetadata()
	}
	if rawExtensions, exists := capabilities["extensions"]; exists {
		if !object(rawExtensions) {
			return Request{}, invalidMetadata()
		}
		var claimed map[string]json.RawMessage
		if json.Unmarshal(rawExtensions, &claimed) != nil {
			return Request{}, invalidMetadata()
		}
		for _, value := range claimed {
			if !object(value) {
				return Request{}, invalidMetadata()
			}
		}
	}
	delete(fields, "_meta")
	clean, err := json.Marshal(fields)
	if err != nil {
		return Request{}, invalidMetadata()
	}
	return Request{Version: version, Params: clean, StatelessEnvelope: true}, nil
}

func object(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func stringValue(raw json.RawMessage, value *string) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '"' && json.Unmarshal(trimmed, value) == nil
}

func nonemptyString(raw json.RawMessage) bool {
	var value string
	return json.Unmarshal(raw, &value) == nil && value != ""
}

func invalidMetadata() *protocol.Error {
	return &protocol.Error{Code: protocol.ErrInvalidParams, Message: "invalid params"}
}

// Unsupported returns only safe revision identities in public error data.
func Unsupported(requested string, policy Policy) *protocol.Error {
	data, _ := json.Marshal(struct {
		Requested string   `json:"requested"`
		Supported []string `json:"supported"`
	}{requested, policy.StatelessSupported()})
	return &protocol.Error{Code: protocol.ErrUnsupportedProtocolVersion, Message: "unsupported protocol version", Data: data}
}

// Complete encodes a synchronous result for the stateless revision. Domain
// values remain unchanged; only the wire result receives revision fields.
func Complete(value any, method, serverName, serverVersion string) (any, *protocol.Error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, &protocol.Error{Code: protocol.ErrInternalError, Message: "internal error"}
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(encoded, &result) != nil || result == nil {
		return nil, &protocol.Error{Code: protocol.ErrInternalError, Message: "internal error"}
	}
	result["resultType"] = json.RawMessage(`"complete"`)
	serverInfo, _ := json.Marshal(map[string]string{"name": serverName, "version": serverVersion})
	meta, _ := json.Marshal(map[string]json.RawMessage{"io.modelcontextprotocol/serverInfo": serverInfo})
	result["_meta"] = meta
	if Cacheable(method) {
		result["ttlMs"] = json.RawMessage(`0`)
		result["cacheScope"] = json.RawMessage(`"private"`)
	}
	return result, nil
}

// Cacheable centralizes revision-specific list/read hint placement. Routing
// still controls which methods exist; this function does not register any.
func Cacheable(method string) bool {
	switch method {
	case "server/discover", "tools/list", "prompts/list", "resources/list", "resources/read":
		return true
	default:
		return false
	}
}
