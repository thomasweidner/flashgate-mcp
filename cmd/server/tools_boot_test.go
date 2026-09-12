package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

func TestCreateToolRegistryRegistersExpectedToolsInOrder(t *testing.T) {
	filesystem := noopFileSystem{}

	registry := createToolRegistry(filesystem, 1024, mustCapabilities(t, capability.FilesystemRead, capability.FilesystemWrite))

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

	registry := createToolRegistry(filesystem, 1024, mustCapabilities(t, capability.FilesystemRead, capability.FilesystemWrite))

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

func TestCreateToolRegistryDoesNotResolveRemovedTools(t *testing.T) {
	registry := createToolRegistry(noopFileSystem{}, 1024, mustCapabilities(t, capability.FilesystemRead, capability.FilesystemWrite))
	for _, name := range []string{"list_files", "stat_path", "exists_path", "mkdir", "rename_path"} {
		if _, ok := registry.Get(name); ok {
			t.Fatalf("expected removed tool %q to be unavailable", name)
		}
	}
}

func TestCreateToolRegistryOmitsWriteToolsWhenReadOnly(t *testing.T) {
	filesystem := noopFileSystem{}

	registry := createToolRegistry(filesystem, 1024, capabilitiesFromReadOnly(true))

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

	registry := createToolRegistry(filesystem, 1024, capabilitiesFromReadOnly(true))

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

func TestCapabilitiesFromReadOnly(t *testing.T) {
	t.Parallel()

	if capabilitiesFromReadOnly(true).Has(capability.FilesystemWrite) {
		t.Fatal("expected filesystem writes to be disabled in read-only mode")
	}

	if !capabilitiesFromReadOnly(false).Has(capability.FilesystemWrite) {
		t.Fatal("expected filesystem writes to be enabled outside read-only mode")
	}
	if !capabilitiesFromReadOnly(true).Has(capability.FilesystemRead) {
		t.Fatal("expected filesystem reads to remain enabled in read-only mode")
	}
}

func TestCreateToolRegistryOmitsToolsWithoutCapabilities(t *testing.T) {
	registry := createToolRegistry(noopFileSystem{}, 1024, toolCapabilities{})
	if got := registry.List(); len(got) != 0 {
		t.Fatalf("expected no tools without functional capabilities, got %#v", got)
	}
}

func mustCapabilities(t *testing.T, names ...capability.Name) capability.Set {
	t.Helper()
	set, ok := capability.NewSet(names...)
	if !ok {
		t.Fatalf("invalid test capabilities: %v", names)
	}
	return set
}

type noopFileSystem struct{}

func (noopFileSystem) List(string) ([]fs.Entry, error) {
	return nil, errors.New("not implemented")
}

func (noopFileSystem) Read(string, int64) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (noopFileSystem) Stat(string) (fs.Metadata, error) {
	return fs.Metadata{}, errors.New("not implemented")
}

func (noopFileSystem) Write(string, []byte, bool) error {
	return errors.New("not implemented")
}

func (noopFileSystem) Mkdir(string) (bool, error) {
	return false, errors.New("not implemented")
}

func (noopFileSystem) Delete(string, bool) error {
	return errors.New("not implemented")
}

func (noopFileSystem) Move(string, string, bool) error {
	return errors.New("not implemented")
}

func (noopFileSystem) Copy(string, string, bool) error {
	return errors.New("not implemented")
}
