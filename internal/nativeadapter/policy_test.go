package nativeadapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const policySchemaVersion = "flashgate-native-os-adapter-policy/v1"

type policy struct {
	SchemaVersion               string                  `json:"schemaVersion"`
	ImplementationPriority      []string                `json:"implementationPriority"`
	RuntimeRoots                []string                `json:"runtimeRoots"`
	ExcludedDevelopmentPackages []string                `json:"excludedDevelopmentPackages"`
	ForbiddenInterpreters       []string                `json:"forbiddenInterpreters"`
	ExternalNativePrograms      []externalNativeProgram `json:"externalNativePrograms"`
}

type externalNativeProgram struct {
	SourceFile   string `json:"sourceFile"`
	ExecutableID string `json:"executableId"`
	Evidence     string `json:"evidence"`
}

func TestProductRuntimeExternalProgramsFollowPolicy(t *testing.T) {
	repositoryRoot := filepath.Clean(filepath.Join("..", ".."))
	configured := readPolicy(t, filepath.Join(repositoryRoot, "docs", "native-os-adapter-policy.json"))

	wantPriority := []string{"go-standard-library", "platform-go-adapter", "direct-os-api-or-stable-interface", "allowlisted-native-program"}
	if !reflect.DeepEqual(configured.ImplementationPriority, wantPriority) {
		t.Fatalf("implementationPriority=%q, want %q", configured.ImplementationPriority, wantPriority)
	}
	if wantRoots := []string{"cmd/server", "internal"}; !reflect.DeepEqual(configured.RuntimeRoots, wantRoots) {
		t.Fatalf("runtimeRoots=%q, want %q", configured.RuntimeRoots, wantRoots)
	}
	if wantExclusions := []string{"internal/benchmark"}; !reflect.DeepEqual(configured.ExcludedDevelopmentPackages, wantExclusions) {
		t.Fatalf("excludedDevelopmentPackages=%q, want %q", configured.ExcludedDevelopmentPackages, wantExclusions)
	}
	if !slices.IsSorted(configured.ForbiddenInterpreters) || len(configured.ForbiddenInterpreters) == 0 {
		t.Fatal("forbiddenInterpreters must be non-empty and sorted")
	}

	allowedSources := make(map[string]struct{}, len(configured.ExternalNativePrograms))
	for _, program := range configured.ExternalNativePrograms {
		if program.SourceFile == "" || program.ExecutableID == "" || program.Evidence == "" {
			t.Fatalf("external native program entries require sourceFile, executableId, and evidence: %#v", program)
		}
		clean := filepath.ToSlash(filepath.Clean(program.SourceFile))
		if clean != program.SourceFile || strings.HasPrefix(clean, "../") || strings.HasSuffix(clean, "_test.go") {
			t.Fatalf("external native program sourceFile is not a clean product source: %q", program.SourceFile)
		}
		if _, duplicate := allowedSources[clean]; duplicate {
			t.Fatalf("duplicate external native program sourceFile %q", clean)
		}
		if slices.Contains(configured.ForbiddenInterpreters, strings.ToLower(program.ExecutableID)) {
			t.Fatalf("external native program executableId %q is a forbidden interpreter", program.ExecutableID)
		}
		allowedSources[clean] = struct{}{}
	}

	foundSources := make(map[string]struct{})
	for _, runtimeRoot := range configured.RuntimeRoots {
		scanRuntimeRoot(t, repositoryRoot, runtimeRoot, configured.ExcludedDevelopmentPackages, allowedSources, foundSources)
	}
	for source := range allowedSources {
		if _, found := foundSources[source]; !found {
			t.Errorf("admitted external native program source %q does not import os/exec in the runtime scope", source)
		}
	}
}

func readPolicy(t *testing.T, path string) policy {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var configured policy
	if err := decoder.Decode(&configured); err != nil {
		t.Fatalf("decode native adapter policy: %v", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("native adapter policy must contain exactly one JSON value: %v", err)
	}
	if configured.SchemaVersion != policySchemaVersion {
		t.Fatalf("schemaVersion=%q, want %q", configured.SchemaVersion, policySchemaVersion)
	}
	return configured
}

func scanRuntimeRoot(t *testing.T, repositoryRoot, runtimeRoot string, exclusions []string, allowed, found map[string]struct{}) {
	t.Helper()
	cleanRoot := filepath.ToSlash(filepath.Clean(runtimeRoot))
	if cleanRoot != runtimeRoot || strings.HasPrefix(cleanRoot, "../") {
		t.Fatalf("runtime root is not a clean repository-relative path: %q", runtimeRoot)
	}
	rootPath := filepath.Join(repositoryRoot, filepath.FromSlash(cleanRoot))
	if err := filepath.WalkDir(rootPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() && relative != cleanRoot && excluded(relative, exclusions) {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		importsExec, err := importsPackage(path, "os/exec")
		if err != nil {
			return err
		}
		if !importsExec {
			return nil
		}
		found[relative] = struct{}{}
		if _, admitted := allowed[relative]; !admitted {
			t.Errorf("product runtime source %q imports os/exec without an externalNativePrograms admission", relative)
		}
		return nil
	}); err != nil {
		t.Fatalf("scan runtime root %q: %v", runtimeRoot, err)
	}
}

func excluded(path string, exclusions []string) bool {
	for _, exclusion := range exclusions {
		clean := filepath.ToSlash(filepath.Clean(exclusion))
		if clean != exclusion || strings.HasPrefix(clean, "../") {
			continue
		}
		if path == clean || strings.HasPrefix(path, clean+"/") {
			return true
		}
	}
	return false
}

func importsPackage(path, importPath string) (bool, error) {
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", path, err)
	}
	quoted := fmt.Sprintf("%q", importPath)
	for _, imported := range parsed.Imports {
		if imported.Path.Value == quoted {
			return true, nil
		}
	}
	return false, nil
}
