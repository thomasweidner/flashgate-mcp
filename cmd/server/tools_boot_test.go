package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

func TestCreateToolRegistryRegistersExpectedToolsInOrder(t *testing.T) {
	filesystem := noopFileSystem{}

	registry := requireToolRegistry(t, filesystem, 1024, false)

	registeredTools := registry.List()
	gotNames := make([]string, 0, len(registeredTools))

	for _, tool := range registeredTools {
		gotNames = append(gotNames, tool.Name())
	}

	wantNames := []string{
		"list_directory",
		"read_file",
		"get_path_info",
		"write_file",
		"create_directory",
		"delete_path",
		"copy_path",
		"move_path",
	}

	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("unexpected tool registration order\nwant: %v\n got: %v", wantNames, gotNames)
	}
}

func TestCreateToolRegistryRegistersResolvableTools(t *testing.T) {
	filesystem := noopFileSystem{}

	registry := requireToolRegistry(t, filesystem, 1024, false)

	expectedNames := []string{
		"list_directory",
		"read_file",
		"get_path_info",
		"write_file",
		"create_directory",
		"delete_path",
		"copy_path",
		"move_path",
	}

	for _, name := range expectedNames {
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("expected tool %q to be registered", name)
		}
	}
}

var mcpRemovedToolNames = []string{"list_files", "stat_path", "exists_path", "mkdir", "rename_path"}

func TestCreateToolRegistryDoesNotResolveRemovedTools(t *testing.T) {
	registry := requireToolRegistry(t, noopFileSystem{}, 1024, false)
	for _, name := range mcpRemovedToolNames {
		if _, ok := registry.Get(name); ok {
			t.Fatalf("expected removed tool %q to be unavailable", name)
		}
	}
}

func TestCreateToolRegistryOmitsWriteToolsWhenReadOnly(t *testing.T) {
	filesystem := noopFileSystem{}

	registry := requireToolRegistry(t, filesystem, 1024, true)

	registeredTools := registry.List()
	gotNames := make([]string, 0, len(registeredTools))

	for _, tool := range registeredTools {
		gotNames = append(gotNames, tool.Name())
	}

	wantNames := []string{
		"list_directory",
		"read_file",
		"get_path_info",
	}

	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("unexpected read-only tool registration order\nwant: %v\n got: %v", wantNames, gotNames)
	}
}

func TestCreateToolRegistryDoesNotResolveWriteToolsWhenReadOnly(t *testing.T) {
	filesystem := noopFileSystem{}

	registry := requireToolRegistry(t, filesystem, 1024, true)

	writeToolNames := []string{
		"write_file",
		"create_directory",
		"delete_path",
		"copy_path",
		"move_path",
	}

	for _, name := range writeToolNames {
		if _, ok := registry.Get(name); ok {
			t.Fatalf("expected write tool %q to be disabled in read-only mode", name)
		}
	}
}

type noopFileSystem struct{}

func (noopFileSystem) LimitsForRoot(config.RootID) (resource.Limits, error) {
	return fs.DefaultLimits().ResourceLimits(), nil
}

func requireToolRegistry(t testing.TB, filesystem fs.FileSystem, maxFileSize int64, readOnly bool) *tools.Registry {
	t.Helper()
	registry := tools.NewRegistry()
	registrations := filesystemTools(maxFileSize)
	if readOnly {
		registrations = registrations[:3]
	}
	for _, registration := range registrations {
		if err := registry.Register(registration.build(filesystem)); err != nil {
			t.Fatal(err)
		}
	}
	return registry
}

// Construct a separate fixture registry to inject result observations or
// intentionally incompatible definitions without changing the live registry.
func registryWithFixtureTool(original *tools.Registry, replacement tools.Tool) (*tools.Registry, error) {
	fixture := tools.NewRegistry()
	for _, tool := range original.List() {
		if tool.Name() == replacement.Name() {
			tool = replacement
		}
		if err := fixture.Register(tool); err != nil {
			return nil, err
		}
	}
	return fixture, nil
}

func effectiveFixtureCatalog(t testing.TB, readOnly bool, maxFileSize int64, version string) (*tools.Registry, *tools.CatalogView) {
	t.Helper()
	registry, view, _ := mcpEffectiveRegistry(t, readOnly, maxFileSize, fs.DefaultLimits(), version)
	return registry, view
}

func (noopFileSystem) List(fs.Operand) ([]fs.Entry, error) {
	return nil, errors.New("not implemented")
}

func (noopFileSystem) Read(fs.Operand, int64) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (noopFileSystem) Stat(fs.Operand) (fs.Metadata, error) {
	return fs.Metadata{}, errors.New("not implemented")
}

func (noopFileSystem) Write(fs.Operand, []byte, bool) error {
	return errors.New("not implemented")
}

func (noopFileSystem) Mkdir(fs.Operand) (bool, error) {
	return false, errors.New("not implemented")
}

func (noopFileSystem) Delete(fs.Operand, bool) error {
	return errors.New("not implemented")
}

func (noopFileSystem) Move(fs.Operand, fs.Operand, bool) error {
	return errors.New("not implemented")
}

func (noopFileSystem) Copy(fs.Operand, fs.Operand, bool) error {
	return errors.New("not implemented")
}
