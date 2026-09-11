package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

func TestCreateToolRegistryRegistersExpectedToolsInOrder(t *testing.T) {
	filesystem := noopFileSystem{}

	registry := createToolRegistry(filesystem, 1024, toolCapabilities{filesystemWrite: true})

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
		"append_file",
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

	registry := createToolRegistry(filesystem, 1024, toolCapabilities{filesystemWrite: true})

	expectedNames := []string{
		"list_directory",
		"read_file",
		"get_path_info",
		"write_file",
		"append_file",
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
	registry := createToolRegistry(noopFileSystem{}, 1024, toolCapabilities{filesystemWrite: true})
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
		"append_file",
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

	if capabilitiesFromReadOnly(true).filesystemWrite {
		t.Fatal("expected filesystem writes to be disabled in read-only mode")
	}

	if !capabilitiesFromReadOnly(false).filesystemWrite {
		t.Fatal("expected filesystem writes to be enabled outside read-only mode")
	}
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

func (noopFileSystem) Append(string, []byte) error {
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
