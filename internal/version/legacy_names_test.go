package version

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
)

func TestLegacyProductNamesRemainConfinedToHistoricalRecords(t *testing.T) {
	repositoryRoot := versionRepositoryRoot(t)
	legacyNames := [][]byte{
		[]byte("file" + "server-mcp"),
		[]byte("file" + "server mcp"),
	}
	expected := map[string][]int{
		"CHANGELOG.md":                                                     {0, 1},
		"docs/adr/001-use-go.md":                                           {3, 0},
		"docs/adr/002-no-external-mcp-library.md":                          {1, 0},
		"docs/adr/003-stdio-transport.md":                                  {2, 0},
		"docs/adr/004-dependency-injection.md":                             {1, 0},
		"docs/adr/005-filesystem-abstraction.md":                           {1, 0},
		"docs/adr/006-flashgate-project-identity-and-open-source-scope.md": {2, 0},
		"docs/technical-rename-to-flashgate-2026-07-11.md":                 {6, 1},
	}

	actual, err := findLegacyProductNames(repositoryRoot, legacyNames)
	if err != nil {
		t.Fatalf("scan repository for legacy product names: %v", err)
	}

	if diff := legacyNameCountDiff(expected, actual); diff != "" {
		t.Fatalf("legacy product-name inventory changed; only the pinned historical records are allowed:\n%s", diff)
	}
}

func TestFindLegacyProductNamesReportsUnexpectedFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "active.txt"), []byte("start "+"file"+"server-mcp"+" end"), 0o600); err != nil {
		t.Fatal(err)
	}

	actual, err := findLegacyProductNames(root, [][]byte{[]byte("file" + "server-mcp")})
	if err != nil {
		t.Fatal(err)
	}
	if diff := legacyNameCountDiff(map[string][]int{}, actual); !strings.Contains(diff, "active.txt") {
		t.Fatalf("expected unexpected path in inventory diff, got %q", diff)
	}
}

func versionRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func findLegacyProductNames(root string, names [][]byte) (map[string][]int, error) {
	actual := make(map[string][]int)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "build":
				if path != root {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		contents = bytes.ToLower(contents)
		counts := make([]int, len(names))
		found := false
		for index, name := range names {
			counts[index] = bytes.Count(contents, bytes.ToLower(name))
			found = found || counts[index] > 0
		}
		if found {
			relativePath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			actual[filepath.ToSlash(relativePath)] = counts
		}
		return nil
	})
	return actual, err
}

func legacyNameCountDiff(expected, actual map[string][]int) string {
	paths := make(map[string]struct{}, len(expected)+len(actual))
	for path := range expected {
		paths[path] = struct{}{}
	}
	for path := range actual {
		paths[path] = struct{}{}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)

	var differences []string
	for _, path := range ordered {
		want, expectedPath := expected[path]
		got, actualPath := actual[path]
		if !expectedPath {
			differences = append(differences, fmt.Sprintf("unexpected %s: %v", path, got))
		} else if !actualPath {
			differences = append(differences, fmt.Sprintf("missing %s: want %v", path, want))
		} else if !slices.Equal(want, got) {
			differences = append(differences, fmt.Sprintf("changed %s: want %v, got %v", path, want, got))
		}
	}
	return strings.Join(differences, "\n")
}
