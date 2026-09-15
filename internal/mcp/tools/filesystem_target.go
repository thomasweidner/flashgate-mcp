package tools

import (
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
)

type filesystemResolver interface {
	FileSystem(string) (fs.FileSystem, error)
}

type singleFilesystemResolver struct{ filesystem fs.FileSystem }

func (r singleFilesystemResolver) FileSystem(id string) (fs.FileSystem, error) {
	if id != roots.DefaultID {
		return nil, roots.ErrUnknownRoot
	}
	return r.filesystem, nil
}

func rootIDSchema() map[string]any {
	return map[string]any{
		"type":        "string",
		"minLength":   1,
		"description": "Opaque configured root ID. Defaults to 'default' during single-root migration.",
	}
}

func effectiveRootID(id string) string {
	if id == "" {
		return roots.DefaultID
	}
	return id
}

func resolveFilesystem(resolver filesystemResolver, id string) (fs.FileSystem, string, bool) {
	id = effectiveRootID(id)
	if !isNonBlank(id) {
		return nil, "", false
	}
	filesystem, err := resolver.FileSystem(id)
	if err != nil {
		return nil, "", false
	}
	return filesystem, id, true
}

// BindRootRegistry attaches the application named-root resolver to a filesystem tool.
func BindRootRegistry(tool Tool, resolver *roots.Registry) Tool {
	switch typed := tool.(type) {
	case *ListDirectoryTool:
		typed.resolver = resolver
	case *ReadFileTool:
		typed.resolver = resolver
	case *GetPathInfoTool:
		typed.resolver = resolver
	case *WriteFileTool:
		typed.resolver = resolver
	case *CreateDirectoryTool:
		typed.resolver = resolver
	case *DeletePathTool:
		typed.resolver = resolver
	case *CopyPathTool:
		typed.resolver = resolver
	case *MovePathTool:
		typed.resolver = resolver
	}
	return tool
}
