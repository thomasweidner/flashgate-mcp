package main

import (
	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
)

type toolCapabilities = capability.Set

func capabilitiesFromReadOnly(readOnly bool) toolCapabilities {
	names := []capability.Name{capability.FilesystemRead}
	if !readOnly {
		names = append(names, capability.FilesystemWrite)
	}
	capabilities, ok := capability.NewSet(names...)
	if !ok {
		panic("server capability constants are invalid")
	}
	return capabilities
}

func createToolRegistry(filesystem fs.FileSystem, maxFileSize int64, capabilities toolCapabilities) *tools.Registry {
	toolRegistry := tools.NewRegistry()
	if capabilities.Has(capability.FilesystemRead) {
		toolRegistry.Register(tools.NewListDirectoryTool(filesystem))
		toolRegistry.Register(tools.NewReadFileTool(filesystem, maxFileSize))
		toolRegistry.Register(tools.NewGetPathInfoTool(filesystem))
	}

	if !capabilities.Has(capability.FilesystemWrite) {
		return toolRegistry
	}

	toolRegistry.Register(tools.NewWriteFileTool(filesystem))
	toolRegistry.Register(tools.NewCreateDirectoryTool(filesystem))
	toolRegistry.Register(tools.NewDeletePathTool(filesystem))
	toolRegistry.Register(tools.NewCopyPathTool(filesystem))
	toolRegistry.Register(tools.NewMovePathTool(filesystem))

	return toolRegistry
}
