package command

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

type testPathResolver struct {
	base string
	err  error
}

func (r testPathResolver) ResolveCommandPath(_ string, path string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return filepath.Join(r.base, path), nil
}

func TestBuildInvocationCreatesArgvWithoutShell(t *testing.T) {
	minimum, maximum := int64(1), int64(9)
	executable := validExecutable(t)
	definition := validDefinition(executable.ID)
	definition.FixedArguments = []string{"status"}
	definition.ArgumentRules = []ArgumentRule{
		{Name: "short", Flag: "--short", Kind: ValueBool},
		{Name: "count", Flag: "--count", Kind: ValueInteger, Minimum: &minimum, Maximum: &maximum},
		{Name: "format", Flag: "--format", Kind: ValueEnum, AllowedValues: []string{"json", "text"}},
		{Name: "label", Flag: "--label", Kind: ValueString, MaxLength: 50},
		{Name: "target", Kind: ValuePath, Required: true, AllowedRoots: []string{"workspace"}, MaxLength: 100},
	}
	registry, err := NewRegistry([]Executable{executable}, []Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	invocation, err := registry.BuildInvocation(definition.ID, map[string]any{
		"short": true, "count": int64(3), "format": "json",
		"label":  "literal;$(not-a-shell)",
		"target": PathArgument{RootID: "workspace", Path: "dir/file.txt"},
	}, testPathResolver{base: root})
	if err != nil {
		t.Fatalf("BuildInvocation() error = %v", err)
	}
	want := []string{"status", "--short", "--count", "3", "--format", "json", "--label", "literal;$(not-a-shell)", filepath.Join(root, "dir/file.txt")}
	if invocation.ExecutablePath != executable.Path || !reflect.DeepEqual(invocation.Arguments, want) {
		t.Fatalf("BuildInvocation() = %#v, want path %q argv %#v", invocation, executable.Path, want)
	}
}

func TestBuildInvocationRejectsMalformedArgumentObjects(t *testing.T) {
	minimum, maximum := int64(1), int64(3)
	executable := validExecutable(t)
	definition := validDefinition(executable.ID)
	definition.ArgumentRules = []ArgumentRule{
		{Name: "enabled", Flag: "--enabled", Kind: ValueBool},
		{Name: "count", Flag: "--count", Kind: ValueInteger, Minimum: &minimum, Maximum: &maximum},
		{Name: "label", Flag: "--label", Kind: ValueString, MaxLength: 20},
		{Name: "target", Kind: ValuePath, Required: true, AllowedRoots: []string{"workspace"}, MaxLength: 50},
	}
	registry, err := NewRegistry([]Executable{executable}, []Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	validPath := PathArgument{RootID: "workspace", Path: "file.txt"}
	tests := map[string]map[string]any{
		"unknown":       {"target": validPath, "extra": true},
		"missing":       {"enabled": true},
		"null":          {"target": nil},
		"wrong bool":    {"target": validPath, "enabled": "true"},
		"wrong integer": {"target": validPath, "count": float64(2)},
		"out of range":  {"target": validPath, "count": int64(4)},
		"response file": {"target": validPath, "label": "@options"},
		"option value":  {"target": validPath, "label": "--config=bad"},
		"absolute path": {"target": PathArgument{RootID: "workspace", Path: filepath.Join(string(filepath.Separator), "tmp")}},
		"traversal":     {"target": PathArgument{RootID: "workspace", Path: "../secret"}},
		"wrong root":    {"target": PathArgument{RootID: "other", Path: "file"}},
	}
	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := registry.BuildInvocation(definition.ID, values, testPathResolver{base: t.TempDir()}); err == nil {
				t.Fatal("BuildInvocation() accepted malformed arguments")
			}
		})
	}
	if _, err := registry.BuildInvocation(definition.ID, map[string]any{"target": validPath}, testPathResolver{err: errors.New("denied")}); err == nil {
		t.Fatal("BuildInvocation() accepted a failed path binding")
	}
}

func TestRegistryRejectsInjectionSelectors(t *testing.T) {
	executable := validExecutable(t)
	for _, fixed := range []string{"@args", "--config", "--plugin-dir"} {
		definition := validDefinition(executable.ID)
		definition.FixedArguments = []string{fixed}
		if _, err := NewRegistry([]Executable{executable}, []Definition{definition}); err == nil {
			t.Fatalf("NewRegistry() accepted fixed injection selector %q", fixed)
		}
	}
	for _, flag := range []string{"--config", "--hook-path", "--loader"} {
		definition := validDefinition(executable.ID)
		definition.ArgumentRules = []ArgumentRule{{Name: "value", Flag: flag, Kind: ValueString, MaxLength: 10}}
		if _, err := NewRegistry([]Executable{executable}, []Definition{definition}); err == nil {
			t.Fatalf("NewRegistry() accepted injection flag %q", flag)
		}
	}
}
