package main

import (
	"fmt"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
)

func createToolRegistry(filesystem fs.FileSystem, maxFileSize int64, profile config.EffectiveProfile) (*tools.Registry, error) {
	toolRegistry := tools.NewRegistry()
	operations, err := fs.BindOperations(profile.Catalog())
	if err != nil {
		return nil, fmt.Errorf("declare filesystem operations: %w", err)
	}
	declarations := []struct {
		tool      tools.Tool
		operation fs.Operation
	}{
		{tools.NewListDirectoryTool(filesystem), fs.ReadOperation},
		{tools.NewReadFileTool(filesystem, maxFileSize), fs.ReadOperation},
		{tools.NewGetPathInfoTool(filesystem), fs.ReadOperation},
		{tools.NewWriteFileTool(filesystem), fs.WriteOperation},
		{tools.NewCreateDirectoryTool(filesystem), fs.WriteOperation},
		{tools.NewDeletePathTool(filesystem), fs.WriteOperation},
		{tools.NewCopyPathTool(filesystem), fs.CopyOperation},
		{tools.NewMovePathTool(filesystem), fs.WriteOperation},
	}
	for _, declaration := range declarations {
		tool := declaration.tool
		if !profile.Policy().Allows(operations[declaration.operation]) {
			continue
		}
		if err := toolRegistry.Register(tool); err != nil {
			return nil, fmt.Errorf("register %s: %w", tool.Name(), err)
		}
	}
	return toolRegistry, nil
}
