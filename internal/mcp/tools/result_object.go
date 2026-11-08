package tools

import (
	"context"
	"encoding/json"
	"strconv"
	"unicode/utf8"

	"github.com/thomasweidner/flashgate-mcp/internal/wiretrace"
)

// resultObject is a standalone filesystem object. The encoder admits only the
// concrete shapes whose wire sizes it can bound before calling encoding/json.
type resultObject interface{ resultObject() }

func (getPathInfoMissingResult) resultObject()  {}
func (getPathInfoExistingResult) resultObject() {}
func (writeFileResult) resultObject()           {}
func (createDirectoryResult) resultObject()     {}
func (deletePathResult) resultObject()          {}
func (copyPathResult) resultObject()            {}
func (movePathResult) resultObject()            {}
func (listDirectoryResult) resultObject()       {}

func encodeResultObject(ctx context.Context, object resultObject, kind PayloadKind, limit int) (json.RawMessage, error) {
	_, collection := object.(listDirectoryResult)
	if collection != (kind == StructuredPayload) {
		return nil, errInvalidToolResult
	}
	fits, err := resultObjectFits(object, limit)
	if err != nil {
		return nil, err
	}
	if !fits {
		return nil, errToolResultLimit
	}
	encoded, err := wiretrace.Marshal(ctx, object)
	if err != nil {
		return nil, errInvalidToolResult
	}
	if len(encoded) > limit {
		return nil, errToolResultLimit
	}
	return encoded, nil
}

func resultObjectFits(object resultObject, limit int) (bool, error) {
	switch value := object.(type) {
	case getPathInfoMissingResult:
		return resultStringsFit(limit-len(`{"path":,"exists":}`)-boolSize(value.Exists), value.Path), nil
	case getPathInfoExistingResult:
		return resultStringsFit(limit-len(`{"path":,"exists":,"name":,"isDir":,"size":}`)-
			boolSize(value.Exists)-boolSize(value.IsDir)-decimalSize(value.Size), value.Path, value.Name), nil
	case writeFileResult:
		return resultStringsFit(limit-len(`{"path":,"size":,"written":}`)-
			decimalSize(value.Size)-boolSize(value.Written), value.Path), nil
	case createDirectoryResult:
		return resultStringsFit(limit-len(`{"path":,"created":}`)-boolSize(value.Created), value.Path), nil
	case deletePathResult:
		return resultStringsFit(limit-len(`{"path":,"deleted":}`)-boolSize(value.Deleted), value.Path), nil
	case copyPathResult:
		return resultStringsFit(limit-len(`{"source":,"target":,"copied":}`)-boolSize(value.Copied), value.Source, value.Target), nil
	case movePathResult:
		return resultStringsFit(limit-len(`{"source":,"target":,"moved":}`)-boolSize(value.Moved), value.Source, value.Target), nil
	case listDirectoryResult:
		return directoryResultFits(value, limit), nil
	default:
		return false, errInvalidToolResult
	}
}

func directoryResultFits(result listDirectoryResult, limit int) bool {
	if result.Entries == nil {
		return limit >= len(`{"entries":null}`)
	}
	remaining := limit - len(`{"entries":[]}`)
	for index, entry := range result.Entries {
		if index > 0 {
			remaining-- // comma between entries
		}
		remaining -= len(`{"name":,"isDir":,"size":}`) + boolSize(entry.IsDir) + decimalSize(entry.Size)
		if !consumeResultString(&remaining, entry.Name) {
			return false
		}
	}
	return remaining >= 0
}

func resultStringsFit(remaining int, values ...string) bool {
	for _, value := range values {
		if !consumeResultString(&remaining, value) {
			return false
		}
	}
	return remaining >= 0
}

// consumeResultString charges the UTF-8 byte count plus encoding/json's
// default escapes. It stops once the remaining budget is exhausted.
func consumeResultString(remaining *int, value string) bool {
	if *remaining < 2 || len(value) > *remaining-2 {
		return false
	}
	*remaining -= len(value) + 2
	for len(value) > 0 {
		char, width := utf8.DecodeRuneInString(value)
		switch {
		case char == utf8.RuneError && width == 1:
			*remaining -= 5
		case char == '\\' || char == '"' || char == '\b' || char == '\f' ||
			char == '\n' || char == '\r' || char == '\t':
			*remaining--
		case char < ' ' || char == '<' || char == '>' || char == '&':
			*remaining -= 5
		case char == '\u2028' || char == '\u2029':
			*remaining -= 3
		}
		if *remaining < 0 {
			return false
		}
		value = value[width:]
	}
	return true
}

func boolSize(value bool) int { return len(strconv.FormatBool(value)) }

func decimalSize(value int64) int {
	var scratch [20]byte
	return len(strconv.AppendInt(scratch[:0], value, 10))
}
