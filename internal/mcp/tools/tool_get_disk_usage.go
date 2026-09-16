package tools

import (
	"context"
	"encoding/json"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const getDiskUsageToolName = "get_disk_usage"

// GetDiskUsageTool exposes privacy-safe capacity information for a path's
// root-confined filesystem.
type GetDiskUsageTool struct{ provider fs.DiskUsageProvider }

func NewGetDiskUsageTool(provider fs.DiskUsageProvider) *GetDiskUsageTool {
	return &GetDiskUsageTool{provider: provider}
}

func (t *GetDiskUsageTool) Name() string  { return getDiskUsageToolName }
func (t *GetDiskUsageTool) Title() string { return "Get Disk Usage" }
func (t *GetDiskUsageTool) Description() string {
	return "Returns privacy-safe capacity information for an existing path below the configured filesystem root."
}
func (t *GetDiskUsageTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string", "minLength": 1, "description": "Relative existing path below the configured filesystem root."},
		},
		"required": []string{"path"}, "additionalProperties": false,
	}
}
func (t *GetDiskUsageTool) Definition() protocol.Tool {
	return protocol.Tool{Name: t.Name(), Title: t.Title(), Description: t.Description(), InputSchema: t.InputSchema(), OutputSchema: filesystemOutputSchema(t.Name())}
}
func (t *GetDiskUsageTool) Execute(_ context.Context, rawArguments json.RawMessage) (any, *protocol.Error) {
	var arguments getDiskUsageArguments
	if rpcErr := decodeStrictArguments(rawArguments, &arguments); rpcErr != nil || !isNonBlank(arguments.Path) {
		return nil, invalidParamsError()
	}

	usage, err := t.provider.DiskUsage(arguments.Path)
	if err != nil {
		return nil, mapFilesystemError(err)
	}
	return getDiskUsageResult{
		Path: arguments.Path, TotalBytes: usage.TotalBytes, UsedBytes: usage.UsedBytes, AvailableBytes: usage.AvailableBytes,
	}, nil
}

type getDiskUsageArguments struct {
	Path string `json:"path"`
}

type getDiskUsageResult struct {
	Path           string `json:"path"`
	TotalBytes     uint64 `json:"totalBytes"`
	UsedBytes      uint64 `json:"usedBytes"`
	AvailableBytes uint64 `json:"availableBytes"`
}
