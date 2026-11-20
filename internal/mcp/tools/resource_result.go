package tools

import (
	"context"
	"math"
	"unicode/utf8"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

func validatePairLimits(filesystem fs.FileSystem, source, target fs.Operand) error {
	if _, err := filesystem.LimitsForRoot(source.RootID); err != nil {
		return mapFilesystemError(err)
	}
	if _, err := filesystem.LimitsForRoot(target.RootID); err != nil {
		return mapFilesystemError(err)
	}
	if source.RootID != target.RootID {
		return mapFilesystemError(fs.ErrCrossRootOperation)
	}
	return nil
}

// Effect admission is lazy: static domain denials never serialize a success
// receipt. The domain invokes it after validation and before its first effect.
func mutationResult(ctx context.Context, filesystem fs.FileSystem, id config.RootID, result ToolResult, contract ResultContract) (*ToolResult, func() error) {
	return &result, func() error {
		if result.prepared != nil {
			return nil
		}
		prepared, err := prepareResult(ctx, filesystem, id, result, contract)
		if err != nil {
			return fs.ErrLimitExceeded
		}
		result = prepared
		return nil
	}
}

func finishMutationResult(result *ToolResult) (ToolResult, error) {
	if result.prepared == nil {
		return ToolResult{}, errInvalidToolResult
	}
	return *result, nil
}

// Count the known mkdir receipt using the existing string-escape rules. The
// object is encoded only once after its actual outcome is known. The second
// escape count accounts for the small-metadata compatibility text copy.
func mkdirReceiptSize(path string, created bool) int64 {
	remaining := maxMetadataBytes
	if !consumeResultString(&remaining, path) {
		return math.MaxInt64
	}
	objectSize := len(`{"path":,"created":}`) + maxMetadataBytes - remaining + boolSize(created)
	escapes := 2 // the encoded path string's opening and closing quotes
	for len(path) > 0 {
		char, width := utf8.DecodeRuneInString(path)
		switch {
		case char == '"' || char == '\\':
			escapes += 2
		case char < ' ' || char == '<' || char == '>' || char == '&' || char == '\u2028' || char == '\u2029':
			escapes++
		case char == utf8.RuneError && width == 1 && len(invalidUTF8JSON) == 6:
			escapes++
		}
		path = path[width:]
	}
	textSize := len(`"See structuredContent for the result."`)
	if objectSize <= metadataParityBytes {
		textSize = objectSize + 2 + 4 + escapes
	}
	return int64(len(`{"content":[{"type":"text","text":}],"structuredContent":}`) + objectSize + textSize)
}

// prepareResult admits the actual serialized envelope. Mutating consumers call
// it before the effect and reuse the admitted envelope without encoding again.
func prepareResult(ctx context.Context, filesystem fs.FileSystem, id config.RootID, result ToolResult, contract ResultContract) (ToolResult, error) {
	limits, err := filesystem.LimitsForRoot(id)
	if err != nil {
		return ToolResult{}, mapFilesystemError(err)
	}
	wrapped, err := contract.wrap(ctx, result)
	if err != nil {
		if err == errToolResultLimit {
			return ToolResult{}, mapFilesystemError(fs.ErrLimitExceeded)
		}
		return ToolResult{}, err
	}
	limit := resource.ResultLimit(ctx, limits.Result)
	if !wrappedResultFits(wrapped, limit) {
		return ToolResult{}, mapFilesystemError(fs.ErrLimitExceeded)
	}
	result.prepared = &wrapped
	return result, nil
}

// Count encoding/json's exact envelope and string escapes, without a second
// serialization or a payload-sized counting buffer. Raw objects were already
// admitted by the existing encoder and contain compact, HTML-escaped JSON.
func wrappedResultFits(result protocol.CallToolResult, limit int64) bool {
	if limit < 0 {
		return false
	}
	remaining := int(min(limit, int64(math.MaxInt))) - len(`{"content":[]}`)
	for i, block := range result.Content {
		if i != 0 {
			remaining--
		}
		switch b := block.(type) {
		case protocol.TextContent:
			remaining -= len(`{"type":"text","text":}`)
			if b.Type != "text" || !consumeResultString(&remaining, b.Text) {
				return false
			}
		case protocol.MediaContent:
			remaining -= len(`{"type":,"mimeType":,"data":""}`)
			if !consumeResultString(&remaining, b.Type) || !consumeResultString(&remaining, b.MIMEType) {
				return false
			}
			remaining -= (len(b.Data) + 2) / 3 * 4
		default:
			return false
		}
	}
	if len(result.StructuredContent) != 0 {
		remaining -= len(`,"structuredContent":`) + len(result.StructuredContent)
	}
	if result.IsError {
		remaining -= len(`,"isError":true`)
	}
	return remaining >= 0
}
