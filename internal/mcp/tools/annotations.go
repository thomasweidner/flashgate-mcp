package tools

import "github.com/thomasweidner/flashgate-mcp/internal/protocol"

var readOnlyAnnotations = protocol.ToolAnnotations{
	ReadOnlyHint:   true,
	IdempotentHint: true,
}

var nonDestructiveWriteAnnotations = protocol.ToolAnnotations{
	IdempotentHint: true,
}

var destructiveWriteAnnotations = protocol.ToolAnnotations{
	DestructiveHint: true,
	IdempotentHint:  true,
}
