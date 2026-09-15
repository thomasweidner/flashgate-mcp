package search

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestProductionSearchRemainsPureGo is the permanent BL-082 dependency gate.
// Search must remain available without an external executable, cgo, or a
// third-party Go module. A future accelerator belongs behind a separate,
// optional adapter and may not replace this package's portable baseline.
func TestProductionSearchRemainsPureGo(t *testing.T) {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve pure-Go gate location")
	}
	packageDir := filepath.Dir(currentFile)
	files, err := filepath.Glob(filepath.Join(packageDir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}

	for _, filename := range files {
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", filepath.Base(filename), err)
		}
		for _, imported := range parsed.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("decode import in %s: %v", filepath.Base(filename), err)
			}
			if path == "C" || path == "os/exec" {
				t.Errorf("%s imports prohibited runtime dependency %q", filepath.Base(filename), path)
			}
			if strings.Contains(path, ".") && !strings.HasPrefix(path, "github.com/thomasweidner/flashgate-mcp/internal/") {
				t.Errorf("%s imports third-party module %q", filepath.Base(filename), path)
			}
		}
	}
}
