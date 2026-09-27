package tools

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

func decodeStrictArguments(raw json.RawMessage, target any) *toolExecutionError {
	if !isJSONObject(raw) {
		return invalidArgumentsError()
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return invalidArgumentsError()
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return invalidArgumentsError()
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
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
