package main

import (
	"encoding/json"
	"fmt"
	"runtime"

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
	bind := func(ctx handlers.Context) (execution.CatalogContext, error) {
		return application.CatalogContext(ctx.Context, execution.Protocol(ctx.ProtocolVersion))
	}
	var definitions []*tools.CatalogTool
	for _, route := range filesystemTools(maxFileSize) {
		metadata := route.build(nil)
		operationKeys := application.OperationKeys(route.route)
		if len(operationKeys) == 0 {
			return nil, fmt.Errorf("unbound filesystem route %s: %w", route.route, execution.ErrMismatch)
		}
		var outcomes []tools.CatalogOutcome
		for _, operation := range operationKeys {
			for _, result := range route.results {
				outcomes = append(outcomes, tools.CatalogOutcome{Operation: operation, Result: result})
			}
		}
		definition := &tools.CatalogTool{
			Handle: func(ctx handlers.Context, invocation execution.CatalogInvocation, arguments json.RawMessage) (tools.ToolResult, error) {
				filesystem, err := application.ForTool(ctx.Context, invocation)
				if err != nil {
					return tools.ToolResult{}, err
				}
				return route.build(filesystem).Execute(ctx.Context, arguments)
			},
		}
		for _, revision := range []execution.Protocol{execution.MCP20251125, execution.MCP20260728} {
			definition.Representations = append(definition.Representations, tools.CatalogRepresentation{
				Support:  tools.CatalogSupport{Revision: revision, Backend: execution.CurrentProcess, Origin: execution.InheritedSTDIO, Platform: runtime.GOOS},
				Metadata: metadata, Variants: operationKeys, Outcomes: outcomes,
			})
		}
		definitions = append(definitions, definition)
	}
	return tools.NewCatalog(bind, validateProfileView, definitions...)
}

type filesystemTool func(fs.FileSystem) tools.Tool

type filesystemRegistration struct {
	build   filesystemTool
	route   execution.OperationID
	results []string
}

func filesystemTools(maxFileSize int64) []filesystemRegistration {
	return []filesystemRegistration{
		{func(f fs.FileSystem) tools.Tool { return tools.NewListDirectoryTool(f) }, fsapp.ListOperation, []string{"complete"}},
		{func(f fs.FileSystem) tools.Tool { return tools.NewReadFileTool(f, maxFileSize) }, fsapp.ReadOperation, []string{"complete"}},
		{func(f fs.FileSystem) tools.Tool { return tools.NewGetPathInfoTool(f) }, fsapp.StatOperation, []string{"existing", "missing"}},
		{func(f fs.FileSystem) tools.Tool { return tools.NewWriteFileTool(f) }, fsapp.WriteOperation, []string{"complete"}},
		{func(f fs.FileSystem) tools.Tool { return tools.NewCreateDirectoryTool(f) }, fsapp.MkdirOperation, []string{"complete"}},
		{func(f fs.FileSystem) tools.Tool { return tools.NewDeletePathTool(f) }, fsapp.DeleteOperation, []string{"complete"}},
		{func(f fs.FileSystem) tools.Tool { return tools.NewCopyPathTool(f) }, fsapp.CopyOperation, []string{"complete"}},
		{func(f fs.FileSystem) tools.Tool { return tools.NewMovePathTool(f) }, fsapp.MoveOperation, []string{"complete"}},
	}
}
