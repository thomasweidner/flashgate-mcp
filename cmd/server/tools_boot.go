package main

import (
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

func createToolRegistry(filesystem fs.FileSystem, maxFileSize int64, capabilities toolCapabilities) *tools.Registry {
	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(tools.NewListDirectoryTool(filesystem))
	toolRegistry.Register(tools.NewReadFileTool(filesystem, maxFileSize))
	toolRegistry.Register(tools.NewGetPathInfoTool(filesystem))
	toolRegistry.Register(tools.NewGetPathsInfoTool(filesystem))

	if !capabilities.filesystemWrite {
		return toolRegistry
	}

	toolRegistry.Register(tools.NewWriteFileTool(filesystem))
	toolRegistry.Register(tools.NewCreateDirectoryTool(filesystem))
	toolRegistry.Register(tools.NewDeletePathTool(filesystem))
	toolRegistry.Register(tools.NewCopyPathTool(filesystem))
	toolRegistry.Register(tools.NewMovePathTool(filesystem))

	return toolRegistry
}
