package server

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type validatedRequest struct {
	id           json.RawMessage
	method       string
	params       json.RawMessage
	notification bool
}

type requestValidationError struct {
	id      json.RawMessage
	code    int
	message string
}

func validateRequestMessageWithLimits(message []byte, maxArgumentBytes int64) (validatedRequest, *requestValidationError) {
	if !json.Valid(message) {
		return validatedRequest{}, newRequestValidationError(nullID(), protocol.ErrParseError, "parse error")
	}

	trimmed := bytes.TrimSpace(message)
	if len(trimmed) == 0 {
		return validatedRequest{}, newRequestValidationError(nullID(), protocol.ErrParseError, "parse error")
	}

	if trimmed[0] != '{' {
		return validatedRequest{}, newRequestValidationError(nullID(), protocol.ErrInvalidRequest, "invalid request")
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return validatedRequest{}, newRequestValidationError(nullID(), protocol.ErrInvalidRequest, "invalid request")
	}

	version, ok := stringField(fields, "jsonrpc")
	if !ok || version != protocol.JSONRPCVersion {
		return validatedRequest{}, newRequestValidationError(requestIDOrNull(fields), protocol.ErrInvalidRequest, "invalid request")
	}

	id, hasID, ok := requestID(fields)
	if !ok {
		return validatedRequest{}, newRequestValidationError(nullID(), protocol.ErrInvalidRequest, "invalid request")
	}

	method, ok := stringField(fields, "method")
	if !ok || strings.TrimSpace(method) == "" {
		return validatedRequest{}, newRequestValidationError(idForInvalidRequest(id, hasID), protocol.ErrInvalidRequest, "invalid request")
	}

	if !hasID {
		return validatedRequest{
			method:       method,
			params:       fields["params"],
			notification: true,
		}, nil
	}

	params, paramsErr := validateMethodParams(method, fields["params"], maxArgumentBytes)
	if paramsErr != nil {
		paramsErr.id = id
		return validatedRequest{}, paramsErr
	}

	return validatedRequest{
		id:     id,
		method: method,
		params: params,
	}, nil
}

func validateMethodParams(method string, params json.RawMessage, maxArgumentBytes int64) (json.RawMessage, *requestValidationError) {
	switch method {
	case "initialize":
		if !isJSONObject(params) {
			return nil, newRequestValidationError(nil, protocol.ErrInvalidParams, "invalid params")
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(params, &fields); err != nil {
			return nil, newRequestValidationError(nil, protocol.ErrInvalidParams, "invalid params")
		}

		version, ok := stringField(fields, "protocolVersion")
		if !ok || version == "" {
			return nil, newRequestValidationError(nil, protocol.ErrInvalidParams, "invalid params")
		}

		return params, nil
	case "tools/list":
		if len(bytes.TrimSpace(params)) == 0 || isJSONNull(params) || isJSONObject(params) {
			return params, nil
		}

		return nil, newRequestValidationError(nil, protocol.ErrInvalidParams, "invalid params")
	case "tools/call":
		return validateToolsCallParams(params, maxArgumentBytes)
	case "ping":
		if !validPingParams(params) {
			return nil, newRequestValidationError(nil, protocol.ErrInvalidParams, "invalid params")
		}
		return params, nil
	default:
		return params, nil
	}
}

func validPingParams(params json.RawMessage) bool {
	if len(params) == 0 {
		return true
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(params, &fields) != nil || fields == nil {
		return false
	}
	rawMeta, exists := fields["_meta"]
	if !exists {
		return true
	}
	var meta map[string]json.RawMessage
	if json.Unmarshal(rawMeta, &meta) != nil || meta == nil {
		return false
	}
	token, exists := meta["progressToken"]
	if !exists {
		return true
	}
	switch token[0] {
	case '"':
		return true
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return isIntegralJSONNumber(string(token))
	default:
		return false
	}
}

// JSON Schema accepts any number with an integral value. Compare the decimal
// scale without converting to float or allocating a power of ten.
func isIntegralJSONNumber(number string) bool {
	mantissa, exponent := number, "0"
	if split := strings.IndexAny(number, "eE"); split >= 0 {
		mantissa, exponent = number[:split], number[split+1:]
	}
	if strings.Trim(mantissa, "-0.") == "" {
		return true
	}
	whole, fraction, _ := strings.Cut(mantissa, ".")
	trailingZeros := len(fraction) - len(strings.TrimRight(fraction, "0"))
	if trailingZeros == len(fraction) {
		trailingZeros += len(whole) - len(strings.TrimRight(whole, "0"))
	}
	exp, err := strconv.Atoi(exponent)
	if err != nil {
		// The JSON syntax is already valid. An exponent outside the native
		// integer range exceeds every possible mantissa length.
		return exponent[0] != '-'
	}
	return exp >= len(fraction)-trailingZeros
}

func validateToolsCallParams(params json.RawMessage, maxArgumentBytes int64) (json.RawMessage, *requestValidationError) {
	if !isJSONObject(params) {
		return nil, newRequestValidationError(nil, protocol.ErrInvalidParams, "invalid params")
	}

	if exceedsLimit(params, maxArgumentBytes) {
		return nil, newRequestValidationError(nil, protocol.ErrInvalidParams, "invalid params")
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(params, &fields); err != nil {
		return nil, newRequestValidationError(nil, protocol.ErrInvalidParams, "invalid params")
	}

	name, ok := stringField(fields, "name")
	if !ok || strings.TrimSpace(name) == "" {
		return nil, newRequestValidationError(nil, protocol.ErrInvalidParams, "invalid params")
	}

	arguments, hasArguments := fields["arguments"]
	if !hasArguments || isJSONNull(arguments) {
		fields["arguments"] = json.RawMessage(`{}`)
		normalized, err := json.Marshal(fields)
		if err != nil {
			return nil, newRequestValidationError(nil, protocol.ErrInternalError, "internal error")
		}

		return normalized, nil
	}

	if !isJSONObject(arguments) {
		return nil, newRequestValidationError(nil, protocol.ErrInvalidParams, "invalid params")
	}

	if exceedsLimit(arguments, maxArgumentBytes) {
		return nil, newRequestValidationError(nil, protocol.ErrInvalidParams, "invalid params")
	}

	return params, nil
}

func exceedsLimit(raw json.RawMessage, limit int64) bool {
	return limit > 0 && int64(len(bytes.TrimSpace(raw))) > limit
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

	return value, true
}

func requestID(fields map[string]json.RawMessage) (json.RawMessage, bool, bool) {
	raw, ok := fields["id"]
	if !ok {
		return nil, false, true
	}

	if !isValidID(raw) {
		return nil, true, false
	}

	return append(json.RawMessage(nil), raw...), true, true
}

func requestIDOrNull(fields map[string]json.RawMessage) json.RawMessage {
	id, hasID, ok := requestID(fields)
	if !hasID || !ok {
		return nullID()
	}

	return id
}

func idForInvalidRequest(id json.RawMessage, hasID bool) json.RawMessage {
	if !hasID {
		return nullID()
	}

	return id
}

func isValidID(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return true
	}

	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return false
	}

	switch value.(type) {
	case string, float64:
		return true
	default:
		return false
	}
}

func isJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)

	return len(trimmed) > 0 && trimmed[0] == '{'
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func newRequestValidationError(id json.RawMessage, code int, message string) *requestValidationError {
	if len(id) == 0 {
		id = nullID()
	}

	return &requestValidationError{
		id:      id,
		code:    code,
		message: message,
	}
}

func nullID() json.RawMessage {
	return json.RawMessage(`null`)
}
