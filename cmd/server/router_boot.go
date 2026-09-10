package main

import (
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/initialize"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
)

func createRouter(serverName string, serverVersion string, toolRegistry *tools.Registry, capabilities toolCapabilities) *router.Router {
	instructions := initialize.DefaultInstructions
	if !capabilities.filesystemWrite {
		instructions = initialize.ReadOnlyInstructions
	}

	mcpRouter := router.New()
	mcpRouter.Register(initialize.NewHandler(serverName, serverVersion, instructions))
	mcpRouter.Register(tools.NewListHandler(toolRegistry))
	mcpRouter.Register(tools.NewCallHandler(toolRegistry))

	return mcpRouter
}
