package configfixture

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var convertSecurityDescriptor = syscall.NewLazyDLL("advapi32.dll").
	NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")

// Write creates a new configuration fixture owned by the token user. The
// token's default owner can instead be a group, even under the user's profile.
func Write(path string, data []byte) error {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return fixtureError("open_token", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return fixtureError("token_user", err)
	}
	sid, err := user.User.Sid.String()
	if err != nil {
		return fixtureError("user_sid", err)
	}
	sddl, err := syscall.UTF16PtrFromString("O:" + sid + "D:P(A;;FA;;;" + sid + ")")
	if err != nil {
		return fixtureError("descriptor_input", err)
	}
	if err := convertSecurityDescriptor.Find(); err != nil {
		return fixtureError("descriptor_api", err)
	}
	var descriptor uintptr
	ok, _, callErr := convertSecurityDescriptor.Call(
		uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&descriptor)), 0)
	if ok == 0 {
		return fixtureError("descriptor_create", callErr)
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(descriptor)) }()

	name, err := syscall.UTF16PtrFromString(`\\?\` + path)
	if err != nil {
		return fixtureError("fixture_name", err)
	}
	attributes := syscall.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	handle, err := syscall.CreateFile(name, syscall.GENERIC_WRITE, 0, &attributes,
		syscall.CREATE_NEW, syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return fixtureError("fixture_create", err)
	}
	file := os.NewFile(uintptr(handle), "configuration fixture")
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return fixtureError("fixture_write", writeErr)
	}
	if closeErr != nil {
		return fixtureError("fixture_close", closeErr)
	}
	return nil
}

func fixtureError(phase string, err error) error {
	var code syscall.Errno
	if errors.As(err, &code) {
		return fmt.Errorf("Windows configuration fixture failed (%s, win32=%d)", phase, uint32(code))
	}
	return fmt.Errorf("Windows configuration fixture failed (%s)", phase)
}
