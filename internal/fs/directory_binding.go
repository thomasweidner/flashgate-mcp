package fs

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// DirectoryBinding retains the authorized backend, parent and directory
// identities for revalidation. It does not expose a native path or handle.
type DirectoryBinding struct {
	backend *LocalFileSystem
	path    effectPath
}

// BindDirectory checks a directory selection through the existing root and
// native effect boundary. Selecting a directory is a metadata-only check.
// The caller must authorize any subsequent access or effect separately.
func (f *LocalFileSystem) BindDirectory(path string) (DirectoryBinding, error) {
	f.effects.Lock()
	defer f.effects.Unlock()

	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
		return DirectoryBinding{}, security.ErrUNCPathDenied
	}

	// Reject traversal before PathGuard cleans the path. Only "." explicitly
	// selects the root. Reject ambiguous Windows names in every component.
	for component := range strings.FieldsFuncSeq(path, func(r rune) bool {
		return r == '/' || r == rune(filepath.Separator)
	}) {
		if component == ".." {
			return DirectoryBinding{}, security.ErrPathTraversal
		}
		if component != "." && ambiguousNativeFileName(component) {
			return DirectoryBinding{}, ErrFileTypeDenied
		}
	}
	if path != "." && filepath.Clean(path) == "." && path != "" {
		return DirectoryBinding{}, security.ErrPathTraversal
	}

	resolved, err := f.resolveExisting(path)
	if err != nil {
		return DirectoryBinding{}, err
	}
	operand, err := f.inspectEffect(resolved.String(), false)
	if err != nil {
		return DirectoryBinding{}, err
	}
	if operand.info == nil || !operand.info.IsDir() {
		return DirectoryBinding{}, security.ErrRootNotDirectory
	}
	if err := f.recheckEffect(operand, false); err != nil {
		return DirectoryBinding{}, err
	}
	return DirectoryBinding{backend: f, path: operand}, nil
}

// Revalidate repeats the current path and protection checks against the
// captured identities. The result is scoped to this check at this instant.
func (b DirectoryBinding) Revalidate() error {
	if b.backend == nil {
		return os.ErrPermission
	}
	b.backend.effects.Lock()
	defer b.backend.effects.Unlock()
	return b.backend.recheckEffect(b.path, false)
}
