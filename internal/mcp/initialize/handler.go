package initialize

import (
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const method = "initialize"

const (
	// MaxInstructionsBytes is the permanent wire budget for initialization
	// guidance. Keep this deliberately small because every client receives it.
	MaxInstructionsBytes = 512

	readOnlyInstructions = "Work within the active root. Prefer one bounded search_paths call with specific filters; use pageSize and resume nextCursor instead of restarting. Set read_file maxBytes, list before reading when discovery is needed, and avoid redundant get_path_info calls. Use batch, range, field-selection, or resource tools when the active catalog offers them."
	defaultInstructions  = readOnlyInstructions + " Inspect targets before mutation and use dry-run or conditional-write options when the active catalog offers them."
)

// Handler handles MCP initialize requests.
type Handler struct {
	serverName    string
	serverVersion string
	instructions  string
}

// NewHandler creates a new initialize handler.
func NewHandler(serverName string, serverVersion string, instructions string) *Handler {
	return &Handler{
		serverName:    serverName,
		serverVersion: serverVersion,
		instructions:  instructions,
	}
}

// ProfileInstructions returns deterministic guidance for the active catalog.
func ProfileInstructions(readOnly bool) string {
	if readOnly {
		return readOnlyInstructions
	}
	return defaultInstructions
}

// Method returns the JSON-RPC method handled by this handler.
func (h *Handler) Method() string {
	return method
}

// Handle processes an initialize request.
func (h *Handler) Handle(_ handlers.Context, rawParams json.RawMessage) (any, *protocol.Error) {
	var params requestParams
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return nil, &protocol.Error{
			Code:    protocol.ErrInvalidParams,
			Message: "invalid initialize params",
		}
	}

	if params.ProtocolVersion == "" {
		return nil, &protocol.Error{
			Code:    protocol.ErrInvalidParams,
			Message: "missing protocol version",
		}
	}

	return response{
		ProtocolVersion: protocol.ProtocolVersion,
		Capabilities: serverCapabilities{
			Tools: toolsCapability{},
		},
		ServerInfo: implementation{
			Name:    h.serverName,
			Version: h.serverVersion,
		},
		Instructions: h.instructions,
	}, nil
}

type requestParams struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    json.RawMessage `json:"capabilities,omitempty"`
	ClientInfo      implementation  `json:"clientInfo"`
}

type response struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      implementation     `json:"serverInfo"`
	Instructions    string             `json:"instructions"`
}

type serverCapabilities struct {
	Tools toolsCapability `json:"tools"`
}

type toolsCapability struct{}

type implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
