// Package discovery builds stateless MCP discovery from enabled adapter state.
package discovery

import "github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"

// DiscoverResult is the 2026-07-28 discovery DTO. Extensions, when enabled by
// a later decision, belong under capabilities rather than at result top level.
type DiscoverResult struct {
	SupportedVersions []string     `json:"supportedVersions"`
	Capabilities      Capabilities `json:"capabilities"`
}

// Capabilities contains only server features actually exposed on this path.
type Capabilities struct {
	Tools      struct{}       `json:"tools"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// Result describes revisions selectable through the stateless entry path.
// The handshake-based 2025-11-25 path is never a modern follow-up selection.
func Result(policy revision.Policy) DiscoverResult {
	versions := []string{}
	if policy.Enabled(revision.Stateless) {
		versions = append(versions, revision.Stateless)
	}
	return DiscoverResult{SupportedVersions: versions, Capabilities: Capabilities{}}
}
