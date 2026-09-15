package main

import (
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/initialize"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
)

func createRouter(serverName string, serverVersion string, toolRegistry *tools.Registry, capabilities toolCapabilities) *router.Router {
	mcpRouter := router.New()
	mcpRouter.Register(initialize.NewHandler(serverName, serverVersion))
	mcpRouter.Register(tools.NewListHandler(toolRegistry))
	mcpRouter.Register(tools.NewCallHandler(toolRegistry, newToolAuthorizer(capabilities)))

	return mcpRouter
}
