package configfixture

import (
	"encoding/binary"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"
)

func TestFixtureHasProtectedDACL(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := Write(path, []byte("fixture")); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(`\\?\` + path)
	if err != nil {
		t.Fatal("fixture name conversion failed")
	}
	get := syscall.NewLazyDLL("advapi32.dll").NewProc("GetFileSecurityW")
	if err := get.Find(); err != nil {
		t.Fatal("reading fixture security is unavailable")
	}
	const daclInformation, daclPresent, daclProtected = 4, 0x0004, 0x1000
	var descriptor [4096]byte
	var size uint32
	ok, _, callErr := get.Call(uintptr(unsafe.Pointer(name)), daclInformation,
		uintptr(unsafe.Pointer(&descriptor[0])), uintptr(len(descriptor)), uintptr(unsafe.Pointer(&size)))
	if ok == 0 || size < 4 {
		t.Fatalf("reading fixture DACL failed (win32=%d)", callErr)
	}
	control := binary.LittleEndian.Uint16(descriptor[2:4])
	if control&(daclPresent|daclProtected) != daclPresent|daclProtected {
		t.Fatal("configuration fixture lacks a protected DACL")
	}
}
