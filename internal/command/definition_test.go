package command

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRegistryResolvesTypedDefinitionByServerOwnedIDs(t *testing.T) {
	minimum, maximum := int64(1), int64(8)
	executable := Executable{
		ID:   "git.native",
		Path: absoluteTestPath(t, "git"),
		Identity: &BinaryIdentity{
			SHA256: strings.Repeat("a", 64),
		},
	}
	definition := Definition{
		ID:             "repository.status",
		ExecutableID:   executable.ID,
		FixedArguments: []string{"status", "--porcelain=v1"},
		ArgumentRules: []ArgumentRule{
			{Name: "short", Flag: "--short", Kind: ValueBool},
			{Name: "count", Flag: "--count", Kind: ValueInteger, Minimum: &minimum, Maximum: &maximum},
			{Name: "format", Flag: "--format", Kind: ValueEnum, AllowedValues: []string{"text", "json"}},
			{Name: "label", Flag: "--label", Kind: ValueString, MaxLength: 64},
			{Name: "target", Kind: ValuePath, Required: true, AllowedRoots: []string{"workspace"}, MaxLength: 1024},
		},
		Limits:  Limits{DefaultTimeout: 5 * time.Second, MaximumTimeout: 30 * time.Second, StdoutBytes: 4096, StderrBytes: 2048},
		Network: NetworkDenied,
	}

	registry, err := NewRegistry([]Executable{executable}, []Definition{definition})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	gotDefinition, gotExecutable, ok := registry.Resolve(definition.ID)
	if !ok {
		t.Fatal("Resolve() did not find command")
	}
	if gotExecutable.ID != executable.ID || gotExecutable.Path != executable.Path {
		t.Fatalf("Resolve() executable = %#v", gotExecutable)
	}
	if gotDefinition.ExecutableID != executable.ID || gotDefinition.Network != NetworkDenied {
		t.Fatalf("Resolve() definition = %#v", gotDefinition)
	}

	// Construction and lookup both copy slices and pointers so callers cannot
	// mutate policy after startup validation.
	executable.Identity.SHA256 = strings.Repeat("b", 64)
	definition.FixedArguments[0] = "unsafe"
	definition.ArgumentRules[2].AllowedValues[0] = "unsafe"
	gotDefinition.FixedArguments[0] = "also-unsafe"
	gotDefinition.ArgumentRules[1].Minimum = &maximum
	gotExecutable.Identity.SHA256 = strings.Repeat("c", 64)

	again, againExecutable, ok := registry.Resolve(definition.ID)
	if !ok || again.FixedArguments[0] != "status" || again.ArgumentRules[2].AllowedValues[0] != "text" {
		t.Fatalf("registry policy was mutated: %#v", again)
	}
	if *again.ArgumentRules[1].Minimum != minimum || againExecutable.Identity.SHA256 != strings.Repeat("a", 64) {
		t.Fatal("registry pointer state was mutated")
	}
}

func TestRegistryRejectsInvalidExecutablePolicy(t *testing.T) {
	valid := validExecutable(t)
	tests := map[string]Executable{
		"invalid id":       {ID: "Git", Path: valid.Path},
		"relative path":    {ID: valid.ID, Path: "bin/git"},
		"unclean path":     {ID: valid.ID, Path: valid.Path + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Base(valid.Path)},
		"nul path":         {ID: valid.ID, Path: valid.Path + "\x00suffix"},
		"invalid identity": {ID: valid.ID, Path: valid.Path, Identity: &BinaryIdentity{SHA256: strings.Repeat("A", 64)}},
	}
	for name, executable := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewRegistry([]Executable{executable}, nil); err == nil {
				t.Fatal("NewRegistry() accepted invalid executable")
			}
		})
	}

	if _, err := NewRegistry([]Executable{valid, valid}, nil); err == nil {
		t.Fatal("NewRegistry() accepted duplicate executable IDs")
	}
}

func TestRegistryRejectsInvalidCommandPolicy(t *testing.T) {
	executable := validExecutable(t)
	valid := validDefinition(executable.ID)
	minimum, maximum := int64(10), int64(1)
	tests := map[string]Definition{
		"invalid id":              withDefinition(valid, func(d *Definition) { d.ID = "Status Command" }),
		"unknown executable":      withDefinition(valid, func(d *Definition) { d.ExecutableID = "missing" }),
		"empty fixed arg":         withDefinition(valid, func(d *Definition) { d.FixedArguments = []string{""} }),
		"missing default timeout": withDefinition(valid, func(d *Definition) { d.Limits.DefaultTimeout = 0 }),
		"missing maximum timeout": withDefinition(valid, func(d *Definition) { d.Limits.MaximumTimeout = 0 }),
		"default above maximum": withDefinition(valid, func(d *Definition) {
			d.Limits.DefaultTimeout = 2 * time.Second
			d.Limits.MaximumTimeout = time.Second
		}),
		"missing stdout":   withDefinition(valid, func(d *Definition) { d.Limits.StdoutBytes = 0 }),
		"missing stderr":   withDefinition(valid, func(d *Definition) { d.Limits.StderrBytes = 0 }),
		"implicit network": withDefinition(valid, func(d *Definition) { d.Network = "" }),
		"duplicate name": withDefinition(valid, func(d *Definition) {
			d.ArgumentRules = []ArgumentRule{{Name: "value", Kind: ValueBool}, {Name: "value", Kind: ValueBool}}
		}),
		"duplicate flag": withDefinition(valid, func(d *Definition) {
			d.ArgumentRules = []ArgumentRule{{Name: "one", Flag: "--same", Kind: ValueBool}, {Name: "two", Flag: "--same", Kind: ValueBool}}
		}),
		"flag after positional": withDefinition(valid, func(d *Definition) {
			d.ArgumentRules = []ArgumentRule{{Name: "path", Kind: ValuePath, AllowedRoots: []string{"root"}, MaxLength: 10}, {Name: "flag", Flag: "--flag", Kind: ValueBool}}
		}),
		"open integer range": withDefinition(valid, func(d *Definition) {
			d.ArgumentRules = []ArgumentRule{{Name: "count", Kind: ValueInteger, Minimum: &minimum}}
		}),
		"reversed integer range": withDefinition(valid, func(d *Definition) {
			d.ArgumentRules = []ArgumentRule{{Name: "count", Kind: ValueInteger, Minimum: &minimum, Maximum: &maximum}}
		}),
		"empty enum": withDefinition(valid, func(d *Definition) {
			d.ArgumentRules = []ArgumentRule{{Name: "format", Kind: ValueEnum}}
		}),
		"duplicate enum": withDefinition(valid, func(d *Definition) {
			d.ArgumentRules = []ArgumentRule{{Name: "format", Kind: ValueEnum, AllowedValues: []string{"json", "json"}}}
		}),
		"unbound path": withDefinition(valid, func(d *Definition) {
			d.ArgumentRules = []ArgumentRule{{Name: "path", Kind: ValuePath, MaxLength: 10}}
		}),
		"unbounded string": withDefinition(valid, func(d *Definition) {
			d.ArgumentRules = []ArgumentRule{{Name: "label", Kind: ValueString}}
		}),
		"unknown kind": withDefinition(valid, func(d *Definition) {
			d.ArgumentRules = []ArgumentRule{{Name: "value", Kind: "number"}}
		}),
	}
	for name, definition := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := NewRegistry([]Executable{executable}, []Definition{definition}); err == nil {
				t.Fatal("NewRegistry() accepted invalid command definition")
			}
		})
	}

	if _, err := NewRegistry([]Executable{executable}, []Definition{valid, valid}); err == nil {
		t.Fatal("NewRegistry() accepted duplicate command IDs")
	}
}

func TestRegistryUnknownCommandFailsClosed(t *testing.T) {
	executable := validExecutable(t)
	registry, err := NewRegistry([]Executable{executable}, []Definition{validDefinition(executable.ID)})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if _, _, ok := registry.Resolve("unknown"); ok {
		t.Fatal("Resolve() accepted unknown command ID")
	}
	var nilRegistry *Registry
	if _, _, ok := nilRegistry.Resolve("status"); ok {
		t.Fatal("nil Registry.Resolve() accepted command ID")
	}
}

func validExecutable(t *testing.T) Executable {
	t.Helper()
	return Executable{ID: "git.native", Path: absoluteTestPath(t, "git")}
}

func validDefinition(executableID string) Definition {
	return Definition{
		ID:             "repository.status",
		ExecutableID:   executableID,
		FixedArguments: []string{"status"},
		Limits:         Limits{DefaultTimeout: time.Second, MaximumTimeout: 10 * time.Second, StdoutBytes: 1024, StderrBytes: 1024},
		Network:        NetworkDenied,
	}
}

func withDefinition(definition Definition, mutate func(*Definition)) Definition {
	copy := cloneDefinition(definition)
	mutate(&copy)
	return copy
}

func absoluteTestPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}
