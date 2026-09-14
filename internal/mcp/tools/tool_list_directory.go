package tools

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

const listDirectoryToolName = "list_directory"

// ListDirectoryTool exposes directory listing as an MCP tool.
type ListDirectoryTool struct {
	filesystem fs.FileSystem
	secret     [32]byte
}

const (
	defaultListDirectoryPageSize = 100
	maxListDirectoryPageSize     = 1000
	listDirectoryCursorLifetime  = 5 * time.Minute
)

// NewListDirectoryTool creates a new list_directory tool.
func NewListDirectoryTool(filesystem fs.FileSystem) *ListDirectoryTool {
	tool := &ListDirectoryTool{filesystem: filesystem}
	if _, err := rand.Read(tool.secret[:]); err != nil {
		panic("cannot initialize list_directory cursor authority: " + err.Error())
	}
	return tool
}

func (t *ListDirectoryTool) Name() string  { return listDirectoryToolName }
func (t *ListDirectoryTool) Title() string { return "List Directory" }
func (t *ListDirectoryTool) Description() string {
	return "Lists one bounded page of files and directories below the configured filesystem root."
}
func (t *ListDirectoryTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Relative directory path below the configured filesystem root. Defaults to '.' when omitted.",
			},
			"pageSize": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     maxListDirectoryPageSize,
				"description": "Maximum entries in this page. Defaults to 100; continuation requests may only reduce it.",
			},
			"cursor": map[string]any{
				"type":        "string",
				"minLength":   1,
				"description": "Opaque continuation cursor returned by a previous list_directory page.",
			},
		},
		"additionalProperties": false,
	}
}
func (t *ListDirectoryTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}

func (t *ListDirectoryTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments listDirectoryArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}
	if arguments.PageSize != nil && (*arguments.PageSize < 1 || *arguments.PageSize > maxListDirectoryPageSize) {
		return nil, invalidParamsError()
	}

	path, offset, pageSize := ".", 0, defaultListDirectoryPageSize
	maxPageSize := pageSize
	expiresAt := time.Now().Add(listDirectoryCursorLifetime).Unix()
	var expectedFingerprint string
	if arguments.Cursor != nil {
		if !isNonBlank(*arguments.Cursor) || arguments.Path != nil {
			return nil, invalidParamsError()
		}
		state, cursorErr := t.decodeCursor(*arguments.Cursor)
		if cursorErr != nil {
			return nil, cursorError(cursorErr)
		}
		path, offset, pageSize, maxPageSize, expectedFingerprint, expiresAt = state.Path, state.Offset, state.PageSize, state.PageSize, state.Fingerprint, state.ExpiresAt
		if arguments.PageSize != nil {
			if *arguments.PageSize > pageSize {
				return nil, invalidParamsError()
			}
			pageSize = *arguments.PageSize
		}
	} else {
		if arguments.Path != nil {
			if !isNonBlank(*arguments.Path) {
				return nil, invalidParamsError()
			}
			path = *arguments.Path
		}
		if arguments.PageSize != nil {
			pageSize = *arguments.PageSize
			maxPageSize = pageSize
		}
	}

	entries, fingerprint, nextOffset, err := t.listPage(path, offset, pageSize)
	if err != nil {
		return nil, mapFilesystemError(err)
	}
	if expectedFingerprint != "" && subtle.ConstantTimeCompare([]byte(expectedFingerprint), []byte(fingerprint)) != 1 {
		return nil, cursorError(errCursorInvalidated)
	}

	result := listDirectoryResult{Entries: entries}
	if nextOffset != 0 {
		cursor, err := t.encodeCursor(listDirectoryCursor{Path: path, Offset: nextOffset, PageSize: maxPageSize, Fingerprint: fingerprint, ExpiresAt: expiresAt})
		if err != nil {
			return nil, &protocol.Error{Code: protocol.ErrInternalError, Message: "filesystem error: io error"}
		}
		result.NextCursor = cursor
	}
	return result, nil
}

type directoryPager interface {
	ListPage(path string, offset int, limit int) ([]fs.Entry, string, int, error)
}

func (t *ListDirectoryTool) listPage(path string, offset, pageSize int) ([]fs.Entry, string, int, error) {
	if pager, ok := t.filesystem.(directoryPager); ok {
		return pager.ListPage(path, offset, pageSize)
	}
	entries, err := t.filesystem.List(path)
	if err != nil {
		return nil, "", 0, err
	}
	payload, err := json.Marshal(entries)
	if err != nil {
		return nil, "", 0, err
	}
	fingerprint := sha256.Sum256(payload)
	end := offset + pageSize
	if offset > len(entries) {
		offset = len(entries)
	}
	if end > len(entries) {
		end = len(entries)
	}
	next := 0
	if end < len(entries) {
		next = end
	}
	return entries[offset:end], base64.RawURLEncoding.EncodeToString(fingerprint[:]), next, nil
}

type listDirectoryArguments struct {
	Path     *string `json:"path,omitempty"`
	PageSize *int    `json:"pageSize,omitempty"`
	Cursor   *string `json:"cursor,omitempty"`
}

type listDirectoryResult struct {
	Entries    []fs.Entry `json:"entries"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

type listDirectoryCursor struct {
	Path        string `json:"path"`
	Offset      int    `json:"offset"`
	PageSize    int    `json:"pageSize"`
	Fingerprint string `json:"fingerprint"`
	ExpiresAt   int64  `json:"expiresAt"`
}

var (
	errCursorInvalid     = errors.New("invalid_cursor")
	errCursorExpired     = errors.New("cursor_expired")
	errCursorInvalidated = errors.New("cursor_invalidated")
)

func (t *ListDirectoryTool) encodeCursor(state listDirectoryCursor) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, t.secret[:])
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (t *ListDirectoryTool) decodeCursor(value string) (listDirectoryCursor, error) {
	var state listDirectoryCursor
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return state, errCursorInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return state, errCursorInvalid
	}
	tag, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return state, errCursorInvalid
	}
	mac := hmac.New(sha256.New, t.secret[:])
	mac.Write(payload)
	if !hmac.Equal(tag, mac.Sum(nil)) {
		return state, errCursorInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil || state.Path == "" || state.Offset < 1 || state.PageSize < 1 || state.PageSize > maxListDirectoryPageSize || state.Fingerprint == "" || state.ExpiresAt < 1 {
		return listDirectoryCursor{}, errCursorInvalid
	}
	if time.Now().Unix() >= state.ExpiresAt {
		return listDirectoryCursor{}, errCursorExpired
	}
	return state, nil
}

func cursorError(err error) *protocol.Error {
	return &protocol.Error{Code: protocol.ErrInvalidParams, Message: err.Error()}
}

type filesystemErrorCategory string

const (
	categoryNotFound             filesystemErrorCategory = "not_found"
	categoryAlreadyExists        filesystemErrorCategory = "already_exists"
	categoryAccessDenied         filesystemErrorCategory = "access_denied"
	categoryInvalidPath          filesystemErrorCategory = "invalid_path"
	categoryUnsupportedPathType  filesystemErrorCategory = "unsupported_path_type"
	categoryUnsupportedOperation filesystemErrorCategory = "unsupported_operation"
	categoryLimitExceeded        filesystemErrorCategory = "limit_exceeded"
	categoryIOError              filesystemErrorCategory = "io_error"
)

func classifyFilesystemError(err error) filesystemErrorCategory {
	switch {
	case errors.Is(err, security.ErrAbsolutePath),
		errors.Is(err, security.ErrPathTraversal),
		errors.Is(err, security.ErrOutsideRoot),
		errors.Is(err, security.ErrHiddenPathDenied),
		errors.Is(err, security.ErrUNCPathDenied),
		errors.Is(err, security.ErrSymlinkDenied),
		errors.Is(err, security.ErrReparsePointDenied),
		errors.Is(err, fs.ErrSamePath),
		errors.Is(err, fs.ErrMoveIntoSelf),
		errors.Is(err, fs.ErrMovePathChanged):
		return categoryInvalidPath
	case errors.Is(err, fs.ErrNotFound), errors.Is(err, os.ErrNotExist):
		return categoryNotFound
	case errors.Is(err, fs.ErrFileExists), errors.Is(err, os.ErrExist):
		return categoryAlreadyExists
	case errors.Is(err, os.ErrPermission):
		return categoryAccessDenied
	case errors.Is(err, fs.ErrPathIsDirectory),
		errors.Is(err, fs.ErrPathIsNotDirectory),
		errors.Is(err, fs.ErrCopyDirectoryUnsupported),
		errors.Is(err, fs.ErrMoveTypeMismatch),
		errors.Is(err, fs.ErrDirectoryNotEmpty):
		return categoryUnsupportedPathType
	case errors.Is(err, fs.ErrCrossVolumeMoveUnsupported):
		return categoryUnsupportedOperation
	case errors.Is(err, fs.ErrFileTooLarge), errors.Is(err, fs.ErrLimitExceeded):
		return categoryLimitExceeded
	default:
		return categoryIOError
	}
}

func mapFilesystemError(err error) *protocol.Error {
	category := classifyFilesystemError(err)
	code := protocol.ErrInvalidParams
	if category == categoryIOError {
		code = protocol.ErrInternalError
	}

	return &protocol.Error{Code: code, Message: "filesystem error: " + strings.ReplaceAll(string(category), "_", " ")}
}
