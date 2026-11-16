package tools

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func decodeStrictArguments(raw json.RawMessage, fields map[string]any) *toolExecutionError {
	if !isJSONObject(raw) {
		return invalidArgumentsError()
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	if _, err := decoder.Token(); err != nil {
		return invalidArgumentsError()
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return invalidArgumentsError()
		}
		name, ok := key.(string)
		target := fields[name]
		if !ok || target == nil {
			return invalidArgumentsError()
		}
		delete(fields, name)
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil || isJSONNull(value) || json.Unmarshal(value, target) != nil {
			return invalidArgumentsError()
		}
	}
	if _, err := decoder.Token(); err != nil {
		return invalidArgumentsError()
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return invalidArgumentsError()
	}

	return nil
}

func isNonBlank(value string) bool {
	return strings.TrimSpace(value) != ""
}

func invalidParamsError() *protocol.Error {
	return &protocol.Error{
		Code:    protocol.ErrInvalidParams,
		Message: "invalid params",
	}
}

func isMissingNullOrObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)

	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || isJSONObject(raw)
}

func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)

	return len(trimmed) > 0 && trimmed[0] == '{'
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func stringField(fields map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := fields[key]
	if !ok {
		return "", false
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}

	if strings.TrimSpace(value) == "" {
		return "", false
	}

	return value, true
}
