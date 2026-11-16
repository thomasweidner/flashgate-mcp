package tools

import "github.com/thomasweidner/flashgate-mcp/internal/fs"

func rootIDSchema() map[string]any {
	return map[string]any{
		"type":    "string",
		"pattern": "^[a-z][a-z0-9_]{0,63}$",
	}
}

func validOperand(operand fs.Operand) bool {
	return operand.RootID != "" && isNonBlank(operand.Path)
}
