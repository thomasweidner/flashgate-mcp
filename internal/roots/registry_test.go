package roots

import (
	"errors"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

type fakeFileSystem struct{ fs.FileSystem }

var testLimits = DefaultLimits(1024, 2048)

func TestRegistrySupportsMultipleIndependentRoots(t *testing.T) {
	first := &fakeFileSystem{}
	second := &fakeFileSystem{}
	registry, err := New([]Entry{
		{ID: "source", FileSystem: first, Access: Read, Limits: testLimits, FileTypes: AllFileTypes()},
		{ID: "target", FileSystem: second, Access: ReadWrite, Limits: testLimits, FileTypes: AllFileTypes()},
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
		{name: "unsorted file types", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: FileTypes{Extensions: []string{".txt", ".md"}}}}, want: ErrInvalidFileTypes},
		{name: "nonportable file type", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: FileTypes{Extensions: []string{"*.txt"}}}}, want: ErrInvalidFileTypes},
		{name: "duplicate", entries: []Entry{{ID: "root", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: AllFileTypes()}, {ID: "root", FileSystem: filesystem, Access: Write, Limits: testLimits, FileTypes: AllFileTypes()}}, want: ErrDuplicateID},
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

func TestRegistryEnforcesIndependentRootAccess(t *testing.T) {
	filesystem := &fakeFileSystem{}
	registry, err := New([]Entry{
		{ID: "read", FileSystem: filesystem, Access: Read, Limits: testLimits, FileTypes: AllFileTypes()},
		{ID: "write", FileSystem: filesystem, Access: Write, Limits: testLimits, FileTypes: AllFileTypes()},
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

func TestRegistryCopiesFileTypePolicy(t *testing.T) {
	extensions := []string{".md", ".txt"}
	registry, err := New([]Entry{{
		ID: "docs", FileSystem: &fakeFileSystem{}, Access: Read, Limits: testLimits,
		FileTypes: FileTypes{Extensions: extensions},
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
