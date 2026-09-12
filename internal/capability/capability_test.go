package capability

import (
	"reflect"
	"testing"
)

func TestNewSetRejectsUnknownNames(t *testing.T) {
	set, ok := NewSet(FilesystemRead, Name("safe-read"))
	if ok {
		t.Fatalf("expected profile-like unknown name to be rejected: %#v", set)
	}
	if set.Has(FilesystemRead) {
		t.Fatal("rejected input must not return a partially populated set")
	}
}

func TestSetHasAndNames(t *testing.T) {
	set, ok := NewSet(SystemRead, FilesystemWrite, FilesystemRead, FilesystemRead)
	if !ok {
		t.Fatal("expected known names to be accepted")
	}
	if !set.Has(FilesystemRead) || !set.Has(FilesystemWrite) || set.Has(CommandExecute) {
		t.Fatalf("unexpected set membership: %#v", set.Names())
	}
	want := []Name{FilesystemRead, FilesystemWrite, SystemRead}
	if got := set.Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected deterministic names\nwant: %v\n got: %v", want, got)
	}
}

func TestZeroSetIsEmpty(t *testing.T) {
	var set Set
	if set.Has(FilesystemRead) || len(set.Names()) != 0 {
		t.Fatalf("expected zero set to be empty: %#v", set.Names())
	}
}
