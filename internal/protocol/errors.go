package protocol

const (
	// ErrParseError indicates invalid JSON.
	ErrParseError = -32700

	// ErrInvalidRequest indicates an invalid JSON-RPC request.
	ErrInvalidRequest = -32600

	// ErrMethodNotFound indicates an unknown method.
	ErrMethodNotFound = -32601

	// ErrInvalidParams indicates invalid method parameters.
	ErrInvalidParams = -32602

	// ErrInternalError indicates an internal server error.
	ErrInternalError = -32603

	// ErrUnsupportedProtocolVersion is the stateless MCP revision mismatch.
	ErrUnsupportedProtocolVersion = -32022
)
