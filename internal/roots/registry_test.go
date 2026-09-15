package roots

import (
	"errors"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

type fakeFileSystem struct {
	fs.FileSystem
	policy security.Policy
}

func (f *fakeFileSystem) PathPolicy() security.Policy { return f.policy }

var testLimits = DefaultLimits(1024, 2048)
var testLinkRules = LinkRules{Symlinks: DenySymlinks, ReparsePoints: DenyReparsePoints}

func TestRegistrySupportsMultipleIndependentRoots(t *testing.T) {
	first := &fakeFileSystem{}
	second := &fakeFileSystem{}
	registry, err := New([]Entry{
		{ID: "source", FileSystem: first, Access: Read, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemRead},
		{ID: "target", FileSystem: second, Access: ReadWrite, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemReadWrite},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := registry.FileSystem("target", Write)
	if err != nil || got != second {
		t.Fatalf("target lookup = (%v, %v), want second filesystem", got, err)
	}
	if ids := registry.IDs(); !reflect.DeepEqual(ids, []string{"source", "target"}) {
		t.Fatalf("IDs = %v", ids)
	}
}

func TestRegistryFailsClosedForInvalidConfigurationAndLookup(t *testing.T) {
	filesystem := &fakeFileSystem{}
	tests := []struct {
		name    string
		entries []Entry
		want    error
	}{
		{name: "empty", want: ErrNoRoots},
		{name: "blank id", entries: []Entry{{ID: " ", FileSystem: filesystem}}, want: ErrInvalidEntry},
		{name: "nil filesystem", entries: []Entry{{ID: "root"}}, want: ErrInvalidEntry},
		{name: "missing access", entries: []Entry{{ID: "root", FileSystem: filesystem}}, want: ErrInvalidEntry},
		{name: "unknown access", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: 4}}, want: ErrInvalidEntry},
		{name: "invalid limits", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read}}, want: ErrInvalidLimits},
		{name: "missing file types", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits}}, want: ErrInvalidFileTypes},
		{name: "unsorted file types", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: FileTypes{Extensions: []string{".txt", ".md"}}, LinkRules: testLinkRules, Capabilities: FilesystemRead}}, want: ErrInvalidFileTypes},
		{name: "nonportable file type", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: FileTypes{Extensions: []string{"*.txt"}}, LinkRules: testLinkRules, Capabilities: FilesystemRead}}, want: ErrInvalidFileTypes},
		{name: "missing link rules", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: AllFileTypes()}}, want: ErrInvalidLinkRules},
		{name: "allows reparse points", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: LinkRules{Symlinks: DenySymlinks, ReparsePoints: 2}, Capabilities: FilesystemRead}}, want: ErrInvalidLinkRules},
		{name: "filesystem policy mismatch", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: LinkRules{Symlinks: FollowInternalSymlinks, ReparsePoints: DenyReparsePoints}, Capabilities: FilesystemRead}}, want: ErrLinkPolicyMismatch},
		{name: "missing capabilities", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules}}, want: ErrInvalidCapabilities},
		{name: "unknown capability", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: 4}}, want: ErrInvalidCapabilities},
		{name: "capability exceeds access", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemWrite}}, want: ErrInvalidCapabilities},
		{name: "duplicate", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemRead}, {ID: "root", FileSystem: filesystem, Access: Write, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemWrite}}, want: ErrDuplicateID},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.entries)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}

	registry, err := Single(filesystem)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.FileSystem("missing", Read); !errors.Is(err, ErrUnknownRoot) {
		t.Fatalf("lookup error = %v", err)
	}
	if got, err := registry.FileSystem(DefaultID, ReadWrite); err != nil || got != filesystem {
		t.Fatalf("default lookup = (%v, %v)", got, err)
	}
}

func TestRegistryEnforcesCapabilityIndependentlyOfAccess(t *testing.T) {
	filesystem := &fakeFileSystem{}
	registry, err := New([]Entry{
		{ID: "read-capability", FileSystem: filesystem, Access: ReadWrite, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemRead},
		{ID: "write-capability", FileSystem: filesystem, Access: ReadWrite, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemWrite},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.RootWithCapability("read-capability", Write, FilesystemWrite); !errors.Is(err, ErrCapabilityDenied) {
		t.Fatalf("write capability error = %v", err)
	}
	if _, err := registry.RootWithCapability("write-capability", Read, FilesystemRead); !errors.Is(err, ErrCapabilityDenied) {
		t.Fatalf("read capability error = %v", err)
	}
	root, err := registry.RootWithCapability("read-capability", Read, FilesystemRead)
	if err != nil || root.Capabilities != FilesystemRead {
		t.Fatalf("authorized root = (%#v, %v)", root, err)
	}
}

func TestRegistryEnforcesIndependentRootAccess(t *testing.T) {
	filesystem := &fakeFileSystem{}
	registry, err := New([]Entry{
		{ID: "read", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemRead},
		{ID: "write", FileSystem: filesystem, Access: Write, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemWrite},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.FileSystem("read", Read); err != nil {
		t.Fatalf("read access: %v", err)
	}
	if _, err := registry.FileSystem("read", Write); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("write against read-only root error = %v", err)
	}
	if _, err := registry.FileSystem("write", Write); err != nil {
		t.Fatalf("write access: %v", err)
	}
	if _, err := registry.FileSystem("write", Read); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("read against write-only root error = %v", err)
	}
}

func TestRegistryEnforcesProcessWorkingDirectoryPermissionPerRoot(t *testing.T) {
	filesystem := &fakeFileSystem{}
	registry, err := New([]Entry{
		{ID: "workspace", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemRead, ProcessWorkingDirectory: true},
		{ID: "documents", FileSystem: filesystem, Access: ReadWrite, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemReadWrite},
	})
	if err != nil {
		t.Fatal(err)
	}
	root, err := registry.WorkingDirectoryRoot("workspace")
	if err != nil || root.FileSystem != filesystem || !root.ProcessWorkingDirectory {
		t.Fatalf("allowed working-directory root = (%#v, %v)", root, err)
	}
	if _, err := registry.WorkingDirectoryRoot("documents"); !errors.Is(err, ErrWorkingDirectoryDenied) {
		t.Fatalf("denied working-directory root error = %v", err)
	}
	if _, err := registry.WorkingDirectoryRoot("missing"); !errors.Is(err, ErrUnknownRoot) {
		t.Fatalf("unknown working-directory root error = %v", err)
	}
}

func TestSingleRootDeniesProcessWorkingDirectoryByDefault(t *testing.T) {
	registry, err := Single(&fakeFileSystem{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.WorkingDirectoryRoot(DefaultID); !errors.Is(err, ErrWorkingDirectoryDenied) {
		t.Fatalf("default working-directory root error = %v", err)
	}
}

func TestRegistryCopiesFileTypePolicy(t *testing.T) {
	extensions := []string{".md", ".txt"}
	registry, err := New([]Entry{{
		ID: "docs", FileSystem: &fakeFileSystem{}, Access: Read, Limits: testLimits,
		FileTypes: FileTypes{Extensions: extensions}, LinkRules: testLinkRules, Capabilities: FilesystemRead,
	}})
	if err != nil {
		t.Fatal(err)
	}
	extensions[0] = ".exe"
	root, err := registry.Root("docs", Read)
	if err != nil {
		t.Fatal(err)
	}
	root.FileTypes.Extensions[0] = ".bin"
	again, err := registry.Root("docs", Read)
	if err != nil {
		t.Fatal(err)
	}
	if !again.FileTypes.Allows("README.MD") || again.FileTypes.Allows("payload.exe") {
		t.Fatalf("registry file-type policy was mutated: %#v", again.FileTypes)
	}
}

func TestRegistryPreservesMatchingPerRootLinkRules(t *testing.T) {
	deny := &fakeFileSystem{}
	follow := &fakeFileSystem{policy: security.Policy{FollowSymlinks: true}}
	registry, err := New([]Entry{
		{ID: "deny", FileSystem: deny, Access: Read, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: testLinkRules, Capabilities: FilesystemRead},
		{ID: "follow", FileSystem: follow, Access: Read, Limits: testLimits, FileTypes: AllFileTypes(), LinkRules: LinkRules{Symlinks: FollowInternalSymlinks, ReparsePoints: DenyReparsePoints}, Capabilities: FilesystemRead},
	})
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]SymlinkRule{"deny": DenySymlinks, "follow": FollowInternalSymlinks} {
		root, err := registry.Root(id, Read)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if root.LinkRules.Symlinks != want || root.LinkRules.ReparsePoints != DenyReparsePoints {
			t.Fatalf("%s link rules = %#v", id, root.LinkRules)
		}
	}
}
