package tools

import "github.com/thomasweidner/flashgate-mcp/internal/protocol"

var filesystemToolAnnotations = map[string]protocol.ToolAnnotations{
	listDirectoryToolName:   {ReadOnlyHint: true, IdempotentHint: true},
	readFileToolName:        {ReadOnlyHint: true, IdempotentHint: true},
	getPathInfoToolName:     {ReadOnlyHint: true, IdempotentHint: true},
	writeFileToolName:       {DestructiveHint: true, IdempotentHint: true},
	createDirectoryToolName: {IdempotentHint: true},
	deletePathToolName:      {DestructiveHint: true, IdempotentHint: true},
	copyPathToolName:        {DestructiveHint: true, IdempotentHint: true},
	movePathToolName:        {DestructiveHint: true, IdempotentHint: true},
}

func annotationsFor(toolName string) protocol.ToolAnnotations {
	annotations, ok := filesystemToolAnnotations[toolName]
	if !ok {
		panic("missing annotations for MCP tool " + toolName)
	}
	return annotations
}
