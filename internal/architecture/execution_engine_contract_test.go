package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Process creation belongs to the Managed Process Engine. The benchmark
// harness is the only production-code exception because it starts the built
// FlashGate binary as the system under measurement rather than executing a
// user-requested command.
var approvedProcessStartOwners = []string{
	"internal/benchmark",
	"internal/managedprocess",
}

func TestProductionProcessStartsUseApprovedExecutionOwners(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	fileSet := token.NewFileSet()

	err := filepath.WalkDir(repositoryRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return err
		}
		relativePath = filepath.ToSlash(relativePath)

		if entry.IsDir() {
			if relativePath == ".git" || relativePath == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		parsed, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		processPackages := make(map[string]string)
		for _, imported := range parsed.Imports {
			importPath := strings.Trim(imported.Path.Value, `"`)
			if importPath != "os/exec" && importPath != "os" && importPath != "syscall" {
				continue
			}
			importName := filepath.Base(importPath)
			if imported.Name != nil {
				importName = imported.Name.Name
			}
			processPackages[importName] = importPath
			if importPath == "os/exec" && !ownedByApprovedProcessStarter(relativePath) {
				t.Errorf("%s imports os/exec outside an approved process-start owner", relativePath)
			}
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || (selector.Sel.Name != "StartProcess" && selector.Sel.Name != "ForkExec") {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if !ok || (processPackages[identifier.Name] != "os" && processPackages[identifier.Name] != "syscall") {
				return true
			}
			if !ownedByApprovedProcessStarter(relativePath) {
				position := fileSet.Position(selector.Pos())
				t.Errorf("%s:%d calls %s.%s outside an approved process-start owner", relativePath, position.Line, identifier.Name, selector.Sel.Name)
			}
			return true
		})

		return nil
	})
	if err != nil {
		t.Fatalf("inspect production Go sources: %v", err)
	}
}

func ownedByApprovedProcessStarter(path string) bool {
	return slices.ContainsFunc(approvedProcessStartOwners, func(owner string) bool {
		return path == owner || strings.HasPrefix(path, owner+"/")
	})
}
