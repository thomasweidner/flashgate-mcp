// Package mcptest contains test-only MCP wire-contract helpers.
package mcptest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
)

// DecodedCallToolResult is the validated semantic form of a CallToolResult.
type DecodedCallToolResult struct {
	TextContent          any
	StructuredContent    any
	HasStructuredContent bool
	IsError              bool
}

// DecodeCallToolResult validates the exact CallToolResult subset emitted by
// FlashGate SPR-045. It is not a generic MCP decoder: it requires one text
// block plus structuredContent and rejects optional fields such as _meta.
func DecodeCallToolResult(raw json.RawMessage) (DecodedCallToolResult, error) {
	if !isJSONObject(raw) {
		return DecodedCallToolResult{}, errors.New("call tool result must be an object")
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return DecodedCallToolResult{}, fmt.Errorf("decode call tool result: %w", err)
	}
	for name := range fields {
		switch name {
		case "content", "structuredContent", "isError":
		default:
			return DecodedCallToolResult{}, fmt.Errorf("unexpected call tool result field %q", name)
		}
	}

	contentRaw, ok := fields["content"]
	if !ok {
		return DecodedCallToolResult{}, errors.New("content is required")
	}
	var blocks []json.RawMessage
	if err := json.Unmarshal(contentRaw, &blocks); err != nil {
		return DecodedCallToolResult{}, errors.New("content must be an array")
	}
	if len(blocks) != 1 {
		return DecodedCallToolResult{}, errors.New("content must contain exactly one block")
	}

	var textContent any
	for index, blockRaw := range blocks {
		block, err := decodeTextBlock(blockRaw)
		if err != nil {
			return DecodedCallToolResult{}, fmt.Errorf("content block %d: %w", index, err)
		}
		decodedText, err := decodeJSONValue([]byte(block))
		if err != nil {
			return DecodedCallToolResult{}, fmt.Errorf("content block %d text: %w", index, err)
		}
		if !isObjectValue(decodedText) {
			return DecodedCallToolResult{}, fmt.Errorf("content block %d text must encode an object", index)
		}
		if index == 0 {
			textContent = decodedText
		} else if !reflect.DeepEqual(textContent, decodedText) {
			return DecodedCallToolResult{}, errors.New("text content blocks disagree")
		}
	}

	structuredRaw, ok := fields["structuredContent"]
	if !ok {
		return DecodedCallToolResult{}, errors.New("structuredContent is required")
	}
	structured, err := decodeJSONValue(structuredRaw)
	if err != nil {
		return DecodedCallToolResult{}, fmt.Errorf("structuredContent: %w", err)
	}
	if !isObjectValue(structured) {
		return DecodedCallToolResult{}, errors.New("structuredContent must be an object")
	}
	if !reflect.DeepEqual(textContent, structured) {
		return DecodedCallToolResult{}, errors.New("text and structuredContent differ")
	}
	decoded := DecodedCallToolResult{
		TextContent:          textContent,
		StructuredContent:    structured,
		HasStructuredContent: true,
	}

	if isErrorRaw, ok := fields["isError"]; ok {
		if err := json.Unmarshal(isErrorRaw, &decoded.IsError); err != nil {
			return DecodedCallToolResult{}, errors.New("isError must be a boolean")
		}
	}

	return decoded, nil
}

func decodeTextBlock(raw json.RawMessage) (string, error) {
	if !isJSONObject(raw) {
		return "", errors.New("block must be an object")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return "", err
	}
	for name := range fields {
		if name != "type" && name != "text" {
			return "", fmt.Errorf("unexpected text block field %q", name)
		}
	}
	var blockType string
	if rawType, ok := fields["type"]; !ok || json.Unmarshal(rawType, &blockType) != nil || blockType != "text" {
		return "", errors.New("type must be exactly text")
	}
	var text string
	if rawText, ok := fields["text"]; !ok || json.Unmarshal(rawText, &text) != nil {
		return "", errors.New("text must be a string")
	}
	return text, nil
}

func decodeJSONValue(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, errors.New("trailing JSON value")
	}
	return value, nil
}

func isJSONObject(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func isObjectValue(value any) bool {
	_, ok := value.(map[string]any)
	return ok
}
