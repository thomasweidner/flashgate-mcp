package main

import (
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/initialize"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
)

func createRouter(serverName string, serverVersion string, toolRegistry *tools.Registry) *router.Router {
	mcpRouter := router.New()
	_, writeEnabled := toolRegistry.Get("write_file")
	mcpRouter.Register(initialize.NewHandler(serverName, serverVersion, initialize.ProfileInstructions(!writeEnabled)))
	mcpRouter.Register(tools.NewListHandler(toolRegistry))
	mcpRouter.Register(tools.NewCallHandler(toolRegistry))

	return mcpRouter
}
