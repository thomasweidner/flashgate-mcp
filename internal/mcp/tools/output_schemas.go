package tools

func filesystemOutputSchema(toolName string) map[string]any {
	switch toolName {
	case listDirectoryToolName:
		return objectOutputSchema(map[string]any{
			"entries": map[string]any{
				"type": "array",
				"items": objectOutputSchema(map[string]any{
					"name":  map[string]any{"type": "string"},
					"isDir": map[string]any{"type": "boolean"},
					"size":  map[string]any{"type": "integer"},
				}, "name", "isDir", "size"),
			},
			"nextCursor": map[string]any{"type": "string", "minLength": 1},
		}, "entries")
	case getDirectoryTreeToolName:
		return objectOutputSchema(map[string]any{
			"entries": map[string]any{
				"type": "array",
				"items": objectOutputSchema(map[string]any{
					"path":  map[string]any{"type": "string"},
					"depth": map[string]any{"type": "integer", "minimum": 1},
					"name":  map[string]any{"type": "string"},
					"isDir": map[string]any{"type": "boolean"},
					"size":  map[string]any{"type": "integer"},
				}, "path", "depth"),
			},
			"nextCursor": map[string]any{"type": "string", "minLength": 1},
			"truncated":  map[string]any{"type": "boolean"},
		}, "entries", "truncated")
	case readFileToolName:
		return objectOutputSchema(map[string]any{
			"content": map[string]any{"type": "string"},
			"size":    map[string]any{"type": "integer"},
		}, "content", "size")
	case getPathInfoToolName:
		return map[string]any{
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
		return objectOutputSchema(map[string]any{
			"path":    map[string]any{"type": "string"},
			"size":    map[string]any{"type": "integer"},
			"written": map[string]any{"type": "boolean"},
		}, "path", "size", "written")
	case createDirectoryToolName:
		return objectOutputSchema(map[string]any{
			"path":    map[string]any{"type": "string"},
			"created": map[string]any{"type": "boolean"},
		}, "path", "created")
	case deletePathToolName:
		return objectOutputSchema(map[string]any{
			"path":    map[string]any{"type": "string"},
			"deleted": map[string]any{"type": "boolean"},
		}, "path", "deleted")
	case copyPathToolName:
		return objectOutputSchema(map[string]any{
			"source": map[string]any{"type": "string"},
			"target": map[string]any{"type": "string"},
			"copied": map[string]any{"type": "boolean"},
		}, "source", "target", "copied")
	case movePathToolName:
		return objectOutputSchema(map[string]any{
			"source": map[string]any{"type": "string"},
			"target": map[string]any{"type": "string"},
			"moved":  map[string]any{"type": "boolean"},
		}, "source", "target", "moved")
	default:
		return nil
	}
}

func objectOutputSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}
