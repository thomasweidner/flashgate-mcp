package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestCreateToolRegistryRegistersExpectedToolsInOrder(t *testing.T) {
	filesystem := noopFileSystem{}

	registry := createToolRegistry(filesystem, 1024, toolCapabilities{filesystemRead: true, filesystemWrite: true})

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

	registry := createToolRegistry(filesystem, 1024, toolCapabilities{filesystemRead: true, filesystemWrite: true})

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
	registry := createToolRegistry(noopFileSystem{}, 1024, toolCapabilities{filesystemRead: true, filesystemWrite: true})
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

	if capabilitiesFromReadOnly(true).filesystemWrite {
		t.Fatal("expected filesystem writes to be disabled in read-only mode")
	}

	if !capabilitiesFromReadOnly(false).filesystemWrite {
		t.Fatal("expected filesystem writes to be enabled outside read-only mode")
	}
}

func TestCapabilitiesFromRootsUsesEffectiveAggregateCapabilities(t *testing.T) {
	t.Parallel()

	filesystem := noopFileSystem{}
	limits := roots.DefaultLimits(1024, 2048)
	linkRules := roots.LinkRules{Symlinks: roots.DenySymlinks, ReparsePoints: roots.DenyReparsePoints}
	registry, err := roots.New([]roots.Entry{
		{ID: "read", FileSystem: filesystem, Access: roots.Read, Limits: limits, FileTypes: roots.AllFileTypes(), LinkRules: linkRules, Capabilities: roots.FilesystemRead},
		{ID: "write", FileSystem: filesystem, Access: roots.Write, Limits: limits, FileTypes: roots.AllFileTypes(), LinkRules: linkRules, Capabilities: roots.FilesystemWrite},
	})
	if err != nil {
		t.Fatalf("create roots registry: %v", err)
	}

	capabilities := capabilitiesFromRoots(registry)
	if !capabilities.filesystemRead || !capabilities.filesystemWrite {
		t.Fatalf("aggregate capabilities = %#v", capabilities)
	}
	if capabilities := capabilitiesFromRoots(nil); capabilities.filesystemRead || capabilities.filesystemWrite {
		t.Fatalf("nil registry capabilities = %#v", capabilities)
	}
}

func TestCreateToolRegistrySupportsWriteOnlyEffectiveCatalog(t *testing.T) {
	t.Parallel()

	registry := createToolRegistry(noopFileSystem{}, 1024, toolCapabilities{filesystemWrite: true})
	gotNames := make([]string, 0, len(registry.List()))
	for _, tool := range registry.List() {
		gotNames = append(gotNames, tool.Name())
	}
	wantNames := []string{"write_file", "create_directory", "delete_path", "copy_path", "move_path"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("unexpected write-only catalog\nwant: %v\n got: %v", wantNames, gotNames)
	}
}

type noopFileSystem struct{}

func (noopFileSystem) PathPolicy() security.Policy { return security.DefaultPolicy() }

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
