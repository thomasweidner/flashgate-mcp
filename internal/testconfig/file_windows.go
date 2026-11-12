package testconfig

import (
	"errors"
	"os"
	"syscall"
	"testing"
	"unsafe"
)

var convertSecurityDescriptor = syscall.NewLazyDLL("advapi32.dll").
	NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")

// Write creates a new configuration fixture owned by the token user. The
// token's default owner can instead be a group, even under the user's profile.
func Write(t testing.TB, path string, data []byte) {
	t.Helper()
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		failFixture(t, "open_token", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		failFixture(t, "token_user", err)
	}
	sid, err := user.User.Sid.String()
	if err != nil {
		failFixture(t, "user_sid", err)
	}
	sddl, err := syscall.UTF16PtrFromString("O:" + sid + "D:P(A;;FA;;;" + sid + ")")
	if err != nil {
		failFixture(t, "descriptor_input", err)
	}
	if err := convertSecurityDescriptor.Find(); err != nil {
		failFixture(t, "descriptor_api", err)
	}
	var descriptor uintptr
	ok, _, callErr := convertSecurityDescriptor.Call(
		uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&descriptor)), 0)
	if ok == 0 {
		failFixture(t, "descriptor_create", callErr)
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(descriptor)) }()

	name, err := syscall.UTF16PtrFromString(`\\?\` + path)
	if err != nil {
		failFixture(t, "fixture_name", err)
	}
	attributes := syscall.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	handle, err := syscall.CreateFile(name, syscall.GENERIC_WRITE, 0, &attributes,
		syscall.CREATE_NEW, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		failFixture(t, "fixture_create", err)
	}
	file := os.NewFile(uintptr(handle), "configuration fixture")
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		failFixture(t, "fixture_write", writeErr)
	}
	if closeErr != nil {
		failFixture(t, "fixture_close", closeErr)
	}
}

func failFixture(t testing.TB, phase string, err error) {
	t.Helper()
	var code syscall.Errno
	if errors.As(err, &code) {
		t.Fatalf("Windows configuration fixture failed (%s, win32=%d)", phase, uint32(code))
	}
	t.Fatalf("Windows configuration fixture failed (%s)", phase)
}
