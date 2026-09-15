package main

import (
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
)

type toolCapabilities struct {
	filesystemRead  bool
	filesystemWrite bool
}

func capabilitiesFromReadOnly(readOnly bool) toolCapabilities {
	return toolCapabilities{
		filesystemRead:  true,
		filesystemWrite: !readOnly,
	}
}

func capabilitiesFromRoots(registry *roots.Registry) toolCapabilities {
	return toolCapabilities{
		filesystemRead:  registry != nil && registry.HasCapability(roots.FilesystemRead),
		filesystemWrite: registry != nil && registry.HasCapability(roots.FilesystemWrite),
	}
}

func createToolRegistry(filesystem fs.FileSystem, maxFileSize int64, capabilities toolCapabilities) *tools.Registry {
	toolRegistry := tools.NewRegistry()
	if capabilities.filesystemRead {
		toolRegistry.Register(tools.NewListDirectoryTool(filesystem))
		toolRegistry.Register(tools.NewReadFileTool(filesystem, maxFileSize))
		toolRegistry.Register(tools.NewGetPathInfoTool(filesystem))
	}

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
