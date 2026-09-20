package tools

import "github.com/thomasweidner/flashgate-mcp/internal/protocol"

var filesystemToolAnnotationMatrix = map[string]protocol.ToolAnnotations{
	listDirectoryToolName: {
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	},
	readFileToolName: {
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	},
	getPathInfoToolName: {
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	},
	writeFileToolName: {
		ReadOnlyHint:    false,
		DestructiveHint: true,
		IdempotentHint:  false,
		OpenWorldHint:   false,
	},
	createDirectoryToolName: {
		ReadOnlyHint:    false,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	},
	deletePathToolName: {
		ReadOnlyHint:    false,
		DestructiveHint: true,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	},
	copyPathToolName: {
		ReadOnlyHint:    false,
		DestructiveHint: true,
		IdempotentHint:  false,
		OpenWorldHint:   false,
	},
	movePathToolName: {
		ReadOnlyHint:    false,
		DestructiveHint: true,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	},
}

func filesystemToolAnnotations(toolName string) protocol.ToolAnnotations {
	annotations, ok := filesystemToolAnnotationMatrix[toolName]
	if !ok {
		panic("missing MCP annotations for runtime tool: " + toolName)
	}
	return annotations
}
