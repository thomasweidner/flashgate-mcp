package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
)

// Tool describes a MCP tool exposed by the server.
type Tool struct {
	Name         string         `json:"name"`
	Title        string         `json:"title,omitempty"`
	Description  string         `json:"description"`
	InputSchema  any            `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
}

// TextContent is a text content block returned by an MCP tool call.
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewTextContent creates a text content block with the required MCP type.
func NewTextContent(text string) TextContent {
	return TextContent{Type: "text", Text: text}
}

// CallToolResult is the MCP result envelope for tools/call.
type CallToolResult struct {
	Content           []TextContent   `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError,omitempty"`
}

// ToolError is the stable machine-readable payload returned for a failed tool
// invocation. Message is deliberately safe for clients and must not contain
// raw operating-system errors or resolved host paths.
type ToolError struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}

// NewCallToolResult creates a successful result from one serialized JSON object.
func NewCallToolResult(structuredContent json.RawMessage) (CallToolResult, error) {
	var compact bytes.Buffer
	if err := json.Compact(&compact, structuredContent); err != nil {
		return CallToolResult{}, errors.New("structured content must be a JSON object")
	}

	content := compact.Bytes()
	if len(content) < 2 || content[0] != '{' || content[len(content)-1] != '}' {
		return CallToolResult{}, errors.New("structured content must be a JSON object")
	}

	return CallToolResult{
		Content:           []TextContent{NewTextContent(string(content))},
		StructuredContent: content,
	}, nil
}

// NewCallToolErrorResult creates an MCP tool-execution error. Tool failures are
// successful JSON-RPC exchanges and are distinguished with isError instead of
// the JSON-RPC error member.
func NewCallToolErrorResult(category, message string) (CallToolResult, error) {
	encoded, err := json.Marshal(ToolError{Category: category, Message: message})
	if err != nil {
		return CallToolResult{}, err
	}
	result, err := NewCallToolResult(encoded)
	if err != nil {
		return CallToolResult{}, err
	}
	result.IsError = true
	return result, nil
}
