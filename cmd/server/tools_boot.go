package main

import (
	"encoding/json"
	"fmt"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/fsapp"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
)

func createToolRegistry(registry *roots.Registry, maxFileSize int64, profile config.EffectiveProfile) (*tools.Registry, error) {
	application, err := fsapp.NewDirect(registry, profile)
	if err != nil {
		return nil, err
	}
	toolRegistry, err := registerFilesystemTools(maxFileSize, profile, func(registry *tools.Registry, build filesystemTool) error {
		return registry.RegisterBound(build(nil), func(ctx handlers.Context, arguments json.RawMessage) (tools.ToolResult, error) {
			filesystem, err := application.ForRequest(ctx.Context, execution.Protocol(ctx.ProtocolVersion))
			if err != nil {
				return tools.ToolResult{}, err
			}
			return build(filesystem).Execute(ctx.Context, arguments)
		})
	})
	if err != nil {
		return nil, err
	}
	toolRegistry.Seal()
	return toolRegistry, nil
}

type filesystemTool func(fs.FileSystem) tools.Tool

func registerFilesystemTools(maxFileSize int64, profile config.EffectiveProfile, register func(*tools.Registry, filesystemTool) error) (*tools.Registry, error) {
	toolRegistry := tools.NewRegistry()
	operations, err := fs.BindOperations(profile.Catalog())
	if err != nil {
		return nil, fmt.Errorf("declare filesystem operations: %w", err)
	}
	declarations := []struct {
		build     filesystemTool
		operation fs.Operation
	}{
		{func(f fs.FileSystem) tools.Tool { return tools.NewListDirectoryTool(f) }, fs.ReadOperation},
		{func(f fs.FileSystem) tools.Tool { return tools.NewReadFileTool(f, maxFileSize) }, fs.ReadOperation},
		{func(f fs.FileSystem) tools.Tool { return tools.NewGetPathInfoTool(f) }, fs.ReadOperation},
		{func(f fs.FileSystem) tools.Tool { return tools.NewWriteFileTool(f) }, fs.WriteOperation},
		{func(f fs.FileSystem) tools.Tool { return tools.NewCreateDirectoryTool(f) }, fs.WriteOperation},
		{func(f fs.FileSystem) tools.Tool { return tools.NewDeletePathTool(f) }, fs.WriteOperation},
		{func(f fs.FileSystem) tools.Tool { return tools.NewCopyPathTool(f) }, fs.CopyOperation},
		{func(f fs.FileSystem) tools.Tool { return tools.NewMovePathTool(f) }, fs.WriteOperation},
	}
	for _, declaration := range declarations {
		if !profile.Policy().Allows(operations[declaration.operation]) {
			continue
		}
		if err := register(toolRegistry, declaration.build); err != nil {
			return nil, fmt.Errorf("register %s: %w", declaration.build(nil).Name(), err)
		}
	}
	return toolRegistry, nil
}
