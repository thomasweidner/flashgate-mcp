package roots

import (
	"errors"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

type fakeFileSystem struct{ fs.FileSystem }

func TestRegistrySupportsMultipleIndependentRoots(t *testing.T) {
	first := &fakeFileSystem{}
	second := &fakeFileSystem{}
	registry, err := New([]Entry{{ID: "source", FileSystem: first}, {ID: "target", FileSystem: second}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := registry.FileSystem("target")
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
		{name: "duplicate", entries: []Entry{{ID: "root", FileSystem: filesystem}, {ID: "root", FileSystem: filesystem}}, want: ErrDuplicateID},
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
	if _, err := registry.FileSystem("missing"); !errors.Is(err, ErrUnknownRoot) {
		t.Fatalf("lookup error = %v", err)
	}
	if got, err := registry.FileSystem(DefaultID); err != nil || got != filesystem {
		t.Fatalf("default lookup = (%v, %v)", got, err)
	}
}
