package main

import (
	"fmt"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
)

type toolCapabilities struct {
	filesystemWrite bool
}

func capabilitiesFromReadOnly(readOnly bool) toolCapabilities {
	return toolCapabilities{
		filesystemWrite: !readOnly,
	}
}

func createToolRegistry(filesystem fs.FileSystem, maxFileSize int64, capabilities toolCapabilities) (*tools.Registry, error) {
	toolRegistry := tools.NewRegistry()
	enabled := []tools.Tool{
		tools.NewListDirectoryTool(filesystem),
		tools.NewReadFileTool(filesystem, maxFileSize),
		tools.NewGetPathInfoTool(filesystem),
	}
	if capabilities.filesystemWrite {
		enabled = append(enabled, tools.NewWriteFileTool(filesystem), tools.NewCreateDirectoryTool(filesystem),
			tools.NewDeletePathTool(filesystem), tools.NewCopyPathTool(filesystem), tools.NewMovePathTool(filesystem))
	}
	for _, tool := range enabled {
		if err := toolRegistry.Register(tool); err != nil {
			return nil, fmt.Errorf("register %s: %w", tool.Name(), err)
		}
	}
	return toolRegistry, nil
}
