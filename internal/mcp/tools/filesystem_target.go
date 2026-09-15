package tools

import (
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
)

type filesystemResolver interface {
	FileSystem(string, roots.Access) (fs.FileSystem, error)
	Root(string, roots.Access) (roots.Root, error)
}

type singleFilesystemResolver struct{ filesystem fs.FileSystem }

func (r singleFilesystemResolver) FileSystem(id string, required roots.Access) (fs.FileSystem, error) {
	root, err := r.Root(id, required)
	return root.FileSystem, err
}

func (r singleFilesystemResolver) Root(id string, required roots.Access) (roots.Root, error) {
	if id != roots.DefaultID {
		return roots.Root{}, roots.ErrUnknownRoot
	}
	if required == 0 || required&^roots.ReadWrite != 0 {
		return roots.Root{}, roots.ErrAccessDenied
	}
	return roots.Root{FileSystem: r.filesystem, Limits: roots.DefaultLimits(1<<62, 1<<62)}, nil
}

func resolveRoot(resolver filesystemResolver, id string, required roots.Access) (roots.Root, string, bool) {
	id = effectiveRootID(id)
	if !isNonBlank(id) {
		return roots.Root{}, "", false
	}
	root, err := resolver.Root(id, required)
	if err != nil {
		return roots.Root{}, "", false
	}
	return root, id, true
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

func resolveFilesystem(resolver filesystemResolver, id string, required roots.Access) (fs.FileSystem, string, bool) {
	id = effectiveRootID(id)
	if !isNonBlank(id) {
		return nil, "", false
	}
	filesystem, err := resolver.FileSystem(id, required)
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
