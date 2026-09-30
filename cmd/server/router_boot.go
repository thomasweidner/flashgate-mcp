package main

import (
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/initialize"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
)

func createRouter(serverName string, serverVersion string, toolRegistry *tools.Registry) *router.Router {
	mcpRouter := router.NewWithPolicy(serverName, serverVersion, revision.ProductionPolicy())
	mcpRouter.Register(initialize.NewHandler(serverName, serverVersion))
	mcpRouter.Register(tools.NewListHandler(toolRegistry))
	mcpRouter.Register(tools.NewCallHandler(toolRegistry))

	return mcpRouter
}
