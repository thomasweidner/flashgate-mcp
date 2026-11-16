package main

import (
	"fmt"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
)

func createToolRegistry(filesystem fs.FileSystem, maxFileSize int64, profile config.EffectiveProfile) (*tools.Registry, error) {
	toolRegistry := tools.NewRegistry()
	read := []capability.ID{capability.FilesRead}
	write := []capability.ID{capability.FilesWrite}
	readWrite := []capability.ID{capability.FilesRead, capability.FilesWrite}
	declarations := []struct {
		tool   tools.Tool
		rights []capability.ID
	}{
		{tools.NewListDirectoryTool(filesystem), read},
		{tools.NewReadFileTool(filesystem, maxFileSize), read},
		{tools.NewGetPathInfoTool(filesystem), read},
		{tools.NewWriteFileTool(filesystem), write},
		{tools.NewCreateDirectoryTool(filesystem), write},
		{tools.NewDeletePathTool(filesystem), write},
		{tools.NewCopyPathTool(filesystem), readWrite},
		{tools.NewMovePathTool(filesystem), write},
	}
	for _, declaration := range declarations {
		tool := declaration.tool
		operation, err := profile.Catalog().NewOperation(capability.RiskRead, declaration.rights...)
		if err != nil {
			return nil, fmt.Errorf("declare %s: %w", tool.Name(), err)
		}
		if !profile.Policy().Allows(operation) {
			continue
		}
		if err := toolRegistry.Register(tool); err != nil {
			return nil, fmt.Errorf("register %s: %w", tool.Name(), err)
		}
	}
	return toolRegistry, nil
}
