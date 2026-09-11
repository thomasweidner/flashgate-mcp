package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const (
	hashFilesToolName = "hash_files"
	maxHashFiles      = 100
)

// HashFilesTool computes bounded content fingerprints without returning file content.
type HashFilesTool struct {
	filesystem     fs.FileSystem
	serverMaxBytes int64
}

func NewHashFilesTool(filesystem fs.FileSystem, serverMaxBytes int64) *HashFilesTool {
	return &HashFilesTool{filesystem: filesystem, serverMaxBytes: serverMaxBytes}
}
func (t *HashFilesTool) Name() string  { return hashFilesToolName }
func (t *HashFilesTool) Title() string { return "Hash Files" }
func (t *HashFilesTool) Description() string {
	return "Computes bounded SHA-256 content fingerprints for files below the configured filesystem root."
}
func (t *HashFilesTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"paths": map[string]any{
				"type": "array", "minItems": 1, "maxItems": maxHashFiles,
				"items":       map[string]any{"type": "string", "minLength": 1},
				"description": "Relative file paths below the configured filesystem root.",
			},
			"maxBytes": map[string]any{
				"type": "integer", "minimum": 1,
				"description": "Maximum aggregate bytes to hash. Defaults to and cannot exceed the configured maximum file size.",
			},
		},
		"required": []string{"paths"}, "additionalProperties": false,
	}
}
func (t *HashFilesTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}
func (t *HashFilesTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments hashFilesArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil {
		return nil, rpcErr
	}
	if len(arguments.Paths) == 0 || len(arguments.Paths) > maxHashFiles {
		return nil, invalidParamsError()
	}
	for _, path := range arguments.Paths {
		if !isNonBlank(path) {
			return nil, invalidParamsError()
		}
	}

	limit := t.serverMaxBytes
	if arguments.MaxBytes != nil {
		if *arguments.MaxBytes < 1 {
			return nil, invalidParamsError()
		}
		if *arguments.MaxBytes < limit {
			limit = *arguments.MaxBytes
		}
	}
	if limit < 1 {
		return nil, invalidParamsError()
	}

	result := hashFilesResult{Algorithm: "sha256", Items: make([]hashFileItem, 0, len(arguments.Paths))}
	for _, path := range arguments.Paths {
		remaining := limit - result.BytesHashed
		if remaining < 1 {
			result.Items = append(result.Items, hashFileItem{Path: path, Error: string(categoryLimitExceeded)})
			result.Failed++
			continue
		}
		content, err := t.filesystem.Read(path, remaining)
		if err != nil {
			result.Items = append(result.Items, hashFileItem{Path: path, Error: string(classifyFilesystemError(err))})
			result.Failed++
			continue
		}
		sum := sha256.Sum256(content)
		size := int64(len(content))
		result.Items = append(result.Items, hashFileItem{Path: path, Size: &size, Fingerprint: "sha256:" + hex.EncodeToString(sum[:])})
		result.Completed++
		result.BytesHashed += size
	}
	return result, nil
}

type hashFilesArguments struct {
	Paths    []string `json:"paths"`
	MaxBytes *int64   `json:"maxBytes,omitempty"`
}
type hashFileItem struct {
	Path        string `json:"path"`
	Size        *int64 `json:"size,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Error       string `json:"error,omitempty"`
}
type hashFilesResult struct {
	Algorithm   string         `json:"algorithm"`
	Items       []hashFileItem `json:"items"`
	Completed   int            `json:"completed"`
	Failed      int            `json:"failed"`
	BytesHashed int64          `json:"bytesHashed"`
}
