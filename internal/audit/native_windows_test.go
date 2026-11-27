package audit

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestWindowsRejectsExistingBroadAuditACL(t *testing.T) {
	actor, err := currentSID()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "audit")
	sddl, err := syscall.UTF16PtrFromString("O:" + actor + "D:P(A;OICI;FA;;;" + actor + ")(A;OICI;FR;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	var descriptor uintptr
	ok, _, callErr := convertDescriptor.Call(uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&descriptor)), 0)
	if ok == 0 {
		t.Fatal(callErr)
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(descriptor)) }()
	attributes := syscall.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	name, err := syscall.UTF16PtrFromString(`\\?\` + root)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.CreateDirectory(name, &attributes); err != nil {
		t.Fatal(err)
	}
	store, err := NewFileStore(root, "fixture", StoreLimits{12 << 10, 4 << 10, 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Maintain(t.Context()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("existing store granted to unrelated identities: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, lockName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("store data was created before access verification: %v", err)
	}
}
