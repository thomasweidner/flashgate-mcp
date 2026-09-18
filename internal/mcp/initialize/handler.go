package initialize

import (
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const method = "initialize"

const (
	// ReadOnlyInstructions describe efficient use of the restricted filesystem profile.
	ReadOnlyInstructions = "Use list_directory, read_file, and get_path_info. Reuse returned metadata; avoid redundant get_path_info calls. When supported, request only needed fields or ranges and continue paginated, search, or process output with the returned cursor."
	// DefaultInstructions add write guidance for the current default filesystem profile.
	DefaultInstructions = ReadOnlyInstructions + " Prefer batch operations for independent work. Use dry-run before destructive or multi-step changes when supported."
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
