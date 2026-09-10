package tools

func filesystemOutputSchema(toolName string) map[string]any {
	var schema map[string]any
	switch toolName {
	case listDirectoryToolName:
		schema = objectOutputSchema(map[string]any{
			"entries": map[string]any{
				"type": "array",
				"items": objectOutputSchema(map[string]any{
					"name":  map[string]any{"type": "string"},
					"isDir": map[string]any{"type": "boolean"},
					"size":  map[string]any{"type": "integer"},
				}, "name", "isDir", "size"),
			},
		}, "entries")
	case readFileToolName:
		schema = objectOutputSchema(map[string]any{
			"content": map[string]any{"type": "string"},
			"size":    map[string]any{"type": "integer"},
		}, "content", "size")
	case getPathInfoToolName:
		schema = map[string]any{
			"type": "object",
			"oneOf": []any{
				objectOutputSchema(map[string]any{
					"path":   map[string]any{"type": "string"},
					"exists": map[string]any{"const": false},
				}, "path", "exists"),
				objectOutputSchema(map[string]any{
					"path":   map[string]any{"type": "string"},
					"exists": map[string]any{"const": true},
					"name":   map[string]any{"type": "string"},
					"isDir":  map[string]any{"type": "boolean"},
					"size":   map[string]any{"type": "integer"},
				}, "path", "exists", "name", "isDir", "size"),
			},
		}
	case writeFileToolName:
		schema = objectOutputSchema(map[string]any{
			"path":    map[string]any{"type": "string"},
			"size":    map[string]any{"type": "integer"},
			"written": map[string]any{"type": "boolean"},
		}, "path", "size", "written")
	case createDirectoryToolName:
		schema = objectOutputSchema(map[string]any{
			"path":    map[string]any{"type": "string"},
			"created": map[string]any{"type": "boolean"},
		}, "path", "created")
	case deletePathToolName:
		schema = objectOutputSchema(map[string]any{
			"path":    map[string]any{"type": "string"},
			"deleted": map[string]any{"type": "boolean"},
		}, "path", "deleted")
	case copyPathToolName:
		schema = objectOutputSchema(map[string]any{
			"source": map[string]any{"type": "string"},
			"target": map[string]any{"type": "string"},
			"copied": map[string]any{"type": "boolean"},
		}, "source", "target", "copied")
	case movePathToolName:
		schema = objectOutputSchema(map[string]any{
			"source": map[string]any{"type": "string"},
			"target": map[string]any{"type": "string"},
			"moved":  map[string]any{"type": "boolean"},
		}, "source", "target", "moved")
	default:
		return nil
	}
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	return schema
}

func objectOutputSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}
