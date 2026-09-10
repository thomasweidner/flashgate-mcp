package operation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// These packages own business semantics and must not become dependencies of
// the generic operation lifecycle package.
var forbiddenDomainImports = []string{
	"github.com/thomasweidner/flashgate-mcp/internal/fs",
	"github.com/thomasweidner/flashgate-mcp/internal/search",
	"github.com/thomasweidner/flashgate-mcp/internal/process",
	"github.com/thomasweidner/flashgate-mcp/internal/execution",
	"github.com/thomasweidner/flashgate-mcp/internal/systeminfo",
}

func TestOperationPackageDoesNotOwnDomainLogic(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read operation package: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		file, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}

		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", entry.Name(), err)
			}
			if forbiddenOperationDomainImport(path) {
				t.Errorf("%s imports business-domain package %q; operation owns lifecycle only", entry.Name(), path)
			}
		}

		assertNoDomainPackagesDeclared(t, entry.Name(), file)
	}
}

func forbiddenOperationDomainImport(path string) bool {
	for _, forbidden := range forbiddenDomainImports {
		if path == forbidden || strings.HasPrefix(path, forbidden+"/") {
			return true
		}
	}
	return false
}

func assertNoDomainPackagesDeclared(t *testing.T, filename string, file *ast.File) {
	t.Helper()
	if file.Name.Name != "operation" {
		t.Errorf("%s declares package %q, want operation", filename, file.Name.Name)
	}
}

func TestForbiddenOperationDomainImport(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"context": false,
		"github.com/thomasweidner/flashgate-mcp/internal/protocol":   false,
		"github.com/thomasweidner/flashgate-mcp/internal/fs":         true,
		"github.com/thomasweidner/flashgate-mcp/internal/fs/detail":  true,
		"github.com/thomasweidner/flashgate-mcp/internal/search":     true,
		"github.com/thomasweidner/flashgate-mcp/internal/process":    true,
		"github.com/thomasweidner/flashgate-mcp/internal/execution":  true,
		"github.com/thomasweidner/flashgate-mcp/internal/systeminfo": true,
	}
	for path, want := range tests {
		path, want := path, want
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			if got := forbiddenOperationDomainImport(path); got != want {
				t.Fatalf("forbiddenOperationDomainImport(%q) = %v, want %v", path, got, want)
			}
		})
	}
}
