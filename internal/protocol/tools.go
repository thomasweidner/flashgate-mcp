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

// ToolError is the stable machine-readable body of an unsuccessful tool call.
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

// NewCallToolErrorResult creates an MCP tool result with isError=true. Tool
// failures use this envelope instead of the JSON-RPC error channel.
func NewCallToolErrorResult(category, message string) (CallToolResult, error) {
	if category == "" || message == "" {
		return CallToolResult{}, errors.New("tool error category and message are required")
	}

	structuredContent, err := json.Marshal(ToolError{Category: category, Message: message})
	if err != nil {
		return CallToolResult{}, err
	}
	result, err := NewCallToolResult(structuredContent)
	if err != nil {
		return CallToolResult{}, err
	}
	result.IsError = true
	return result, nil
}
