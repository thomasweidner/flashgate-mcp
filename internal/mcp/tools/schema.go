package tools

const jsonSchema202012Dialect = "https://json-schema.org/draft/2020-12/schema"

func inputSchema(schema map[string]any) map[string]any {
	schema["$schema"] = jsonSchema202012Dialect
	return schema
}

func outputSchema(schema map[string]any) map[string]any {
	schema["$schema"] = jsonSchema202012Dialect
	return schema
}
