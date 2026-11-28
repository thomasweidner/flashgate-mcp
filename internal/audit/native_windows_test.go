package audit

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestWindowsStoreCreatesProtectedNodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit")
	store, err := NewFileStore(path, "fixture", StoreLimits{12 << 10, 4 << 10, 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Append(t.Context(), [][]byte{[]byte("{\"value\":1}\n")}, true); err != nil {
		t.Fatal(err)
	}
	nodes := 0
	err = filepath.WalkDir(path, func(node string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		file, err := os.Open(node)
		if err != nil {
			return err
		}
		defer file.Close()
		if !ownedNode(file, entry.IsDir()) || !hasProtectedDACL(t, file) {
			t.Errorf("%s: expected an explicitly owned node with a protected DACL", entry.Name())
		}
		nodes++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if nodes != 7 {
		t.Fatalf("inspected %d nodes, want root, two coordination files, three partition directories and a segment", nodes)
	}
}

func hasProtectedDACL(t *testing.T, file *os.File) bool {
	t.Helper()
	type descriptorHeader struct {
		revision, reserved uint8
		control            uint16
	}
	var descriptor *descriptorHeader
	const fileObject = 1
	const daclSecurityInformation = 4
	status, _, _ := getSecurityInfo.Call(file.Fd(), fileObject, daclSecurityInformation,
		0, 0, 0, 0, uintptr(unsafe.Pointer(&descriptor)))
	if status != 0 || descriptor == nil {
		t.Fatalf("reading the DACL security descriptor: status %d", status)
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(uintptr(unsafe.Pointer(descriptor)))) }()
	const daclProtected = 0x1000
	return descriptor.control&daclProtected != 0
}

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
