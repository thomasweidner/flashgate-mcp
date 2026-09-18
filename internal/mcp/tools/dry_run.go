package tools

func dryRunInputSchema() map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": "When true, validates path policy and returns a request preview without changing the filesystem. Defaults to false.",
	}
}
