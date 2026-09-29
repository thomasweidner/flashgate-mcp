package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/thomasweidner/flashgate-mcp/internal/diagnostics"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/transport"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

// Server is the MCP server runtime.
type Server struct {
	transport   *transport.Transport
	runtime     *Runtime
	diagnostics *diagnostics.Logger
}

// Runtime is the transport-neutral MCP request runtime. It validates and
// dispatches one complete JSON-RPC message without owning framing or host
// lifecycle.
type Runtime struct {
	router           *router.Router
	maxArgumentBytes int64
	diagnostics      *diagnostics.Logger
}

// Options contains optional server runtime limits and diagnostics.
type Options struct {
	MaxMessageBytes  int64
	MaxArgumentBytes int64
	MaxResponseBytes int64
	Diagnostics      *diagnostics.Logger
}

// New creates a new MCP server.
func New(in io.Reader, out io.Writer, router *router.Router) *Server {
	return NewWithOptions(in, out, router, Options{})
}

// NewWithOptions creates a new MCP server with explicit runtime options.
func NewWithOptions(in io.Reader, out io.Writer, router *router.Router, options Options) *Server {
	return &Server{
		transport:   transport.NewWithLimits(in, out, options.MaxMessageBytes, options.MaxResponseBytes),
		runtime:     NewRuntime(router, options),
		diagnostics: options.Diagnostics,
	}
}

// NewRuntime creates a transport-neutral MCP request runtime.
func NewRuntime(router *router.Router, options Options) *Runtime {
	return &Runtime{
		router:           router,
		maxArgumentBytes: options.MaxArgumentBytes,
		diagnostics:      options.Diagnostics,
	}
}

// Run starts the MCP request loop.
func (s *Server) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		message, err := s.transport.ReadMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			if errors.Is(err, transport.ErrMessageTooLarge) {
				s.logDebug("jsonrpc message limit exceeded")
				if writeErr := s.writeError(nullID(), protocol.ErrInvalidRequest, "invalid request"); writeErr != nil {
					return writeErr
				}
				continue
			}

			return err
		}

		response := s.runtime.Handle(ctx, message)
		if response == nil {
			continue
		}
		if err := s.writeResponse(response.ID, *response); err != nil {
			return err
		}
	}
}

// Handle validates and dispatches one complete JSON-RPC message. Notifications
// return nil because they have no protocol response.
func (r *Runtime) Handle(ctx context.Context, message []byte) *protocol.Response {
	request, validationErr := validateRequestMessageWithLimits(message, r.maxArgumentBytes)
	if validationErr != nil {
		r.logDebug("jsonrpc validation error code=%d", validationErr.code)
		return &protocol.Response{
			JSONRPC: protocol.JSONRPCVersion,
			ID:      normalizedID(validationErr.id),
			Error: &protocol.Error{
				Code:    validationErr.code,
				Message: validationErr.message,
			},
		}
	}
	if request.notification {
		r.logDebug("jsonrpc notification ignored method=%s", request.method)
		return nil
	}
	response := r.handleRequest(ctx, request)
	return &response
}

func (r *Runtime) handleRequest(ctx context.Context, request validatedRequest) (response protocol.Response) {
	defer func() {
		if recovered := recover(); recovered != nil {
			r.logDebug("handler panic recovered method=%s", request.method)
			response = protocol.Response{
				JSONRPC: protocol.JSONRPCVersion,
				ID:      request.id,
				Error: &protocol.Error{
					Code:    protocol.ErrInternalError,
					Message: "internal error",
				},
			}
		}
	}()

	result, rpcErr := r.router.Dispatch(
		request.method,
		handlers.Context{Context: ctx},
		request.params,
	)

	if rpcErr != nil {
		return protocol.Response{
			JSONRPC: protocol.JSONRPCVersion,
			ID:      request.id,
			Error: &protocol.Error{
				Code:    rpcErr.Code,
				Message: rpcErr.Message,
			},
		}
	}

	return protocol.Response{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      request.id,
		Result:  result,
	}
}

func (s *Server) writeResponse(id json.RawMessage, response protocol.Response) error {
	if err := s.transport.WriteMessage(response); err != nil {
		if errors.Is(err, transport.ErrResponseTooLarge) {
			s.logDebug("jsonrpc response limit exceeded")
			return s.writeError(id, protocol.ErrInternalError, "internal error")
		}

		return err
	}

	return nil
}

func (s *Server) writeError(id json.RawMessage, code int, message string) error {
	if len(id) == 0 {
		id = nullID()
	}

	response := protocol.Response{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      id,
		Error: &protocol.Error{
			Code:    code,
			Message: message,
		},
	}

	return s.transport.WriteMessageUnbounded(response)
}

func (s *Server) logDebug(format string, args ...any) {
	if s.diagnostics != nil {
		s.diagnostics.Debugf(format, args...)
	}
}

func (r *Runtime) logDebug(format string, args ...any) {
	if r.diagnostics != nil {
		r.diagnostics.Debugf(format, args...)
	}
}

func normalizedID(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return nullID()
	}
	return id
}
