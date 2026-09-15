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
	"path"
	"sort"
	"strings"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const getDirectoryTreeToolName = "get_directory_tree"

const (
	defaultTreeDepth       = 3
	maxTreeDepth           = 32
	defaultTreeEntries     = 1000
	maxTreeEntries         = 10000
	defaultTreePageSize    = 100
	maxTreePageSize        = 1000
	defaultTreeBytes       = 64 * 1024
	minTreeBytes           = 512
	maxTreeBytes           = 1024 * 1024
	directoryTreeCursorTTL = 5 * time.Minute
)

// GetDirectoryTreeTool returns a bounded, recursively enumerated directory tree.
type GetDirectoryTreeTool struct {
	filesystem fs.FileSystem
	secret     [32]byte
}

func NewGetDirectoryTreeTool(filesystem fs.FileSystem) *GetDirectoryTreeTool {
	tool := &GetDirectoryTreeTool{filesystem: filesystem}
	if _, err := rand.Read(tool.secret[:]); err != nil {
		panic("cannot initialize get_directory_tree cursor authority: " + err.Error())
	}
	return tool
}

func (t *GetDirectoryTreeTool) Name() string  { return getDirectoryTreeToolName }
func (t *GetDirectoryTreeTool) Title() string { return "Get Directory Tree" }
func (t *GetDirectoryTreeTool) Description() string {
	return "Returns one bounded page of a directory tree below the configured filesystem root."
}
func (t *GetDirectoryTreeTool) InputSchema() any {
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"path":       map[string]any{"type": "string", "minLength": 1, "description": "Relative directory path. Defaults to '.' when omitted."},
			"maxDepth":   map[string]any{"type": "integer", "minimum": 0, "maximum": maxTreeDepth, "description": "Maximum descendant depth. Defaults to 3."},
			"maxEntries": map[string]any{"type": "integer", "minimum": 1, "maximum": maxTreeEntries, "description": "Maximum entries in the complete tree sequence. Defaults to 1000."},
			"maxBytes":   map[string]any{"type": "integer", "minimum": minTreeBytes, "maximum": maxTreeBytes, "description": "Maximum encoded domain-result bytes per page. Defaults to 65536."},
			"pageSize":   map[string]any{"type": "integer", "minimum": 1, "maximum": maxTreePageSize, "description": "Maximum entries in this page. Defaults to 100."},
			"fields": map[string]any{"type": "array", "minItems": 1, "uniqueItems": true, "items": map[string]any{
				"type": "string", "enum": []string{"name", "isDir", "size"},
			}, "description": "Optional entry fields; path and depth are always returned."},
			"cursor": map[string]any{"type": "string", "minLength": 1, "description": "Opaque continuation cursor returned by a previous get_directory_tree page."},
		},
	}
}
func (t *GetDirectoryTreeTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}

type directoryTreeArguments struct {
	Path       *string  `json:"path,omitempty"`
	MaxDepth   *int     `json:"maxDepth,omitempty"`
	MaxEntries *int     `json:"maxEntries,omitempty"`
	MaxBytes   *int     `json:"maxBytes,omitempty"`
	PageSize   *int     `json:"pageSize,omitempty"`
	Fields     []string `json:"fields,omitempty"`
	Cursor     *string  `json:"cursor,omitempty"`
}

type directoryTreeEntry struct {
	Path  string `json:"path"`
	Depth int    `json:"depth"`
	Name  string `json:"name,omitempty"`
	IsDir *bool  `json:"isDir,omitempty"`
	Size  *int64 `json:"size,omitempty"`
}

type directoryTreeResult struct {
	Entries    []directoryTreeEntry `json:"entries"`
	NextCursor string               `json:"nextCursor,omitempty"`
	Truncated  bool                 `json:"truncated"`
}

type directoryTreeCursor struct {
	Path        string   `json:"path"`
	Offset      int      `json:"offset"`
	MaxDepth    int      `json:"maxDepth"`
	MaxEntries  int      `json:"maxEntries"`
	PageSize    int      `json:"pageSize"`
	MaxBytes    int      `json:"maxBytes"`
	Fields      []string `json:"fields"`
	Fingerprint string   `json:"fingerprint"`
	ExpiresAt   int64    `json:"expiresAt"`
}

func (t *GetDirectoryTreeTool) Execute(_ context.Context, raw json.RawMessage) (any, *protocol.Error) {
	var args directoryTreeArguments
	if rpcErr := decodeStrictArguments(raw, &args); rpcErr != nil {
		return nil, rpcErr
	}
	if !validTreeArguments(args) {
		return nil, invalidParamsError()
	}
	state := directoryTreeCursor{Path: ".", MaxDepth: defaultTreeDepth, MaxEntries: defaultTreeEntries, PageSize: defaultTreePageSize, MaxBytes: defaultTreeBytes, Fields: []string{"name", "isDir", "size"}, ExpiresAt: time.Now().Add(directoryTreeCursorTTL).Unix()}
	if args.Cursor != nil {
		if !isNonBlank(*args.Cursor) || args.Path != nil || args.MaxDepth != nil || args.MaxEntries != nil || args.Fields != nil {
			return nil, invalidParamsError()
		}
		var err error
		state, err = t.decodeTreeCursor(*args.Cursor)
		if err != nil {
			return nil, cursorError(err)
		}
		if args.PageSize != nil {
			if *args.PageSize > state.PageSize {
				return nil, invalidParamsError()
			}
			state.PageSize = *args.PageSize
		}
		if args.MaxBytes != nil {
			if *args.MaxBytes > state.MaxBytes {
				return nil, invalidParamsError()
			}
			state.MaxBytes = *args.MaxBytes
		}
	} else {
		if args.Path != nil {
			state.Path = *args.Path
		}
		if args.MaxDepth != nil {
			state.MaxDepth = *args.MaxDepth
		}
		if args.MaxEntries != nil {
			state.MaxEntries = *args.MaxEntries
		}
		if args.PageSize != nil {
			state.PageSize = *args.PageSize
		}
		if args.MaxBytes != nil {
			state.MaxBytes = *args.MaxBytes
		}
		if args.Fields != nil {
			state.Fields = append([]string(nil), args.Fields...)
			sort.Strings(state.Fields)
		}
	}

	entries, fingerprint, truncated, err := t.walk(state)
	if err != nil {
		return nil, mapFilesystemError(err)
	}
	if state.Fingerprint != "" && subtle.ConstantTimeCompare([]byte(state.Fingerprint), []byte(fingerprint)) != 1 {
		return nil, cursorError(errCursorInvalidated)
	}
	state.Fingerprint = fingerprint
	if state.Offset > len(entries) {
		return nil, cursorError(errCursorInvalidated)
	}
	end := state.Offset + state.PageSize
	if end > len(entries) {
		end = len(entries)
	}
	page := append([]directoryTreeEntry(nil), entries[state.Offset:end]...)
	result, ok := t.fitTreePage(page, state, end < len(entries), truncated)
	if !ok {
		return nil, &protocol.Error{Code: protocol.ErrInvalidParams, Message: "maxBytes_too_small"}
	}
	return result, nil
}

func validTreeArguments(a directoryTreeArguments) bool {
	if a.Path != nil && !isNonBlank(*a.Path) {
		return false
	}
	if a.MaxDepth != nil && (*a.MaxDepth < 0 || *a.MaxDepth > maxTreeDepth) {
		return false
	}
	if a.MaxEntries != nil && (*a.MaxEntries < 1 || *a.MaxEntries > maxTreeEntries) {
		return false
	}
	if a.PageSize != nil && (*a.PageSize < 1 || *a.PageSize > maxTreePageSize) {
		return false
	}
	if a.MaxBytes != nil && (*a.MaxBytes < minTreeBytes || *a.MaxBytes > maxTreeBytes) {
		return false
	}
	seen := map[string]bool{}
	for _, field := range a.Fields {
		if seen[field] || (field != "name" && field != "isDir" && field != "size") {
			return false
		}
		seen[field] = true
	}
	return a.Fields == nil || len(a.Fields) > 0
}

func (t *GetDirectoryTreeTool) walk(state directoryTreeCursor) ([]directoryTreeEntry, string, bool, error) {
	type pendingDirectory struct {
		path  string
		depth int
	}
	if state.MaxDepth == 0 {
		hash := sha256.Sum256([]byte("[]"))
		return []directoryTreeEntry{}, base64.RawURLEncoding.EncodeToString(hash[:]), false, nil
	}
	pending := []pendingDirectory{{state.Path, 0}}
	entries := make([]directoryTreeEntry, 0, state.MaxEntries+1)
	for len(pending) > 0 && len(entries) <= state.MaxEntries {
		current := pending[0]
		pending = pending[1:]
		children, err := t.filesystem.List(current.path)
		if err != nil {
			return nil, "", false, err
		}
		sort.Slice(children, func(i, j int) bool { return children[i].Name < children[j].Name })
		for _, child := range children {
			if len(entries) > state.MaxEntries {
				break
			}
			childPath := child.Name
			if current.path != "." {
				childPath = path.Join(current.path, child.Name)
			}
			depth := current.depth + 1
			entry := directoryTreeEntry{Path: childPath, Depth: depth}
			for _, field := range state.Fields {
				switch field {
				case "name":
					entry.Name = child.Name
				case "isDir":
					value := child.IsDir
					entry.IsDir = &value
				case "size":
					value := child.Size
					entry.Size = &value
				}
			}
			entries = append(entries, entry)
			if child.IsDir && depth < state.MaxDepth {
				pending = append(pending, pendingDirectory{childPath, depth})
			}
		}
	}
	truncated := len(entries) > state.MaxEntries
	if truncated {
		entries = entries[:state.MaxEntries]
	}
	payload, err := json.Marshal(entries)
	if err != nil {
		return nil, "", false, err
	}
	hash := sha256.Sum256(payload)
	return entries, base64.RawURLEncoding.EncodeToString(hash[:]), truncated, nil
}

func (t *GetDirectoryTreeTool) fitTreePage(page []directoryTreeEntry, state directoryTreeCursor, more, truncated bool) (directoryTreeResult, bool) {
	if len(page) == 0 {
		result := directoryTreeResult{Entries: []directoryTreeEntry{}, Truncated: truncated}
		payload, err := json.Marshal(result)
		return result, err == nil && len(payload) <= state.MaxBytes
	}
	for len(page) > 0 {
		result := directoryTreeResult{Entries: page, Truncated: truncated}
		if more {
			next := state
			next.Offset += len(page)
			cursor, err := t.encodeTreeCursor(next)
			if err != nil {
				return directoryTreeResult{}, false
			}
			result.NextCursor = cursor
		}
		payload, err := json.Marshal(result)
		if err == nil && len(payload) <= state.MaxBytes {
			return result, true
		}
		page = page[:len(page)-1]
		more = true
	}
	return directoryTreeResult{}, false
}

func (t *GetDirectoryTreeTool) encodeTreeCursor(state directoryTreeCursor) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, t.secret[:])
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (t *GetDirectoryTreeTool) decodeTreeCursor(value string) (directoryTreeCursor, error) {
	var state directoryTreeCursor
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
	if err := decoder.Decode(&state); err != nil || state.Path == "" || state.Offset < 1 || state.ExpiresAt < 1 {
		return directoryTreeCursor{}, errCursorInvalid
	}
	if time.Now().Unix() >= state.ExpiresAt {
		return directoryTreeCursor{}, errCursorExpired
	}
	return state, nil
}
