// Package mcptest contains test-only MCP wire-contract helpers.
package mcptest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

// DecodedCallToolResult is the validated semantic form of a CallToolResult.
type DecodedCallToolResult struct {
	TextContent          any
	StructuredContent    any
	HasStructuredContent bool
	IsError              bool
}

// DecodeCallToolResult validates the exact CallToolResult subset emitted by
// FlashGate. Success requires text/structured parity; expected errors require
// isError:true and no structuredContent. Optional fields such as _meta are rejected.
func DecodeCallToolResult(raw json.RawMessage) (DecodedCallToolResult, error) {
	return decodeCallToolResult(raw, nil)
}

// DecodeSplitCallToolResult checks the single plain-text block and independent
// structured object emitted by the current split filesystem result contracts.
func DecodeSplitCallToolResult(raw json.RawMessage) (DecodedCallToolResult, error) {
	var plainText string
	result, err := decodeCallToolResult(raw, func(blocks []json.RawMessage) error {
		if len(blocks) != 1 {
			return errors.New("content must contain exactly one text block")
		}
		var err error
		plainText, err = decodeTextBlock(blocks[0])
		return err
	})
	if err == nil && !result.IsError {
		result.TextContent = plainText
	}
	return result, err
}

// DecodeDeclaredCallToolResult compares every payload block against the exact
// ordered array expected by a conformance fixture.
func DecodeDeclaredCallToolResult(raw, expectedBlocks json.RawMessage) (DecodedCallToolResult, error) {
	want, err := decodeJSONValue(expectedBlocks)
	if err != nil {
		return DecodedCallToolResult{}, err
	}
	return decodeCallToolResult(raw, func(blocks []json.RawMessage) error {
		got := make([]any, 0, len(blocks))
		for _, block := range blocks {
			value, err := decodeJSONValue(block)
			if err != nil {
				return err
			}
			got = append(got, value)
		}
		if !reflect.DeepEqual(got, want) {
			return errors.New("content blocks differ from the declared fixture")
		}
		return nil
	})
}

func decodeCallToolResult(raw json.RawMessage, checkParts func([]json.RawMessage) error) (DecodedCallToolResult, error) {
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
	if len(blocks) == 0 {
		return DecodedCallToolResult{}, errors.New("content must contain a block")
	}

	isError := false
	if raw, present := fields["isError"]; present {
		if string(raw) != "true" && string(raw) != "false" {
			return DecodedCallToolResult{}, errors.New("isError must be a boolean")
		}
		isError = string(raw) == "true"
	}
	var textContent any
	if isError || checkParts == nil {
		if len(blocks) != 1 {
			return DecodedCallToolResult{}, errors.New("content must contain exactly one block")
		}
		block, err := decodeTextBlock(blocks[0])
		if err != nil {
			return DecodedCallToolResult{}, err
		}
		textContent, err = decodeJSONValue([]byte(block))
		if err != nil || !isObjectValue(textContent) {
			return DecodedCallToolResult{}, errors.New("content block text must encode an object")
		}
	} else if err := checkParts(blocks); err != nil {
		return DecodedCallToolResult{}, err
	}
	if isError {
		if _, present := fields["structuredContent"]; present {
			return DecodedCallToolResult{}, errors.New("tool errors must omit structuredContent")
		}
		payload := textContent.(map[string]any)
		category, categoryOK := payload["category"].(string)
		message, messageOK := payload["message"].(string)
		if len(payload) != 2 || !categoryOK || !messageOK || strings.TrimSpace(category) == "" || strings.TrimSpace(message) == "" {
			return DecodedCallToolResult{}, errors.New("tool error requires category and message only")
		}
		return DecodedCallToolResult{TextContent: textContent, IsError: true}, nil
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
	if checkParts == nil && !reflect.DeepEqual(textContent, structured) {
		return DecodedCallToolResult{}, errors.New("text and structuredContent differ")
	}
	return DecodedCallToolResult{
		TextContent:          textContent,
		StructuredContent:    structured,
		HasStructuredContent: true,
	}, nil
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
	if rawText := fields["text"]; len(rawText) == 0 || rawText[0] != '"' || json.Unmarshal(rawText, &text) != nil {
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
