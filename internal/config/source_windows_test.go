//go:build windows

package config

import (
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"unsafe"

	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestWindowsConfigPaths(t *testing.T) {
	t.Parallel()
	path, err := defaultPath(SystemService)
	if err != nil || path != "C:\\ProgramData\\FlashGate\\flashgate_config.toml" {
		t.Fatalf("invalid Windows service default: %v", err)
	}
	for _, path := range []string{
		"C:relative.toml", "\\\\server\\share\\config.toml", "\\\\?\\C:\\config.toml",
		"C:\\folder\\..\\config.toml", "C:\\config.toml:secret", "C:\\CON.toml",
		"C:\\folder.\\config.toml", "C:\\",
	} {
		if _, _, ok := splitWindowsPath(path); ok {
			t.Fatalf("unsafe Windows source accepted: %q", path)
		}
	}
	if volume, parts, ok := splitWindowsPath("C:/A Profile/flashgate_config.toml"); !ok || volume != "C:" || len(parts) != 2 {
		t.Fatal("ordinary absolute Windows path rejected")
	}
}

func TestWindowsRejectsJunction(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	testconfig.Write(t, filepath.Join(target, "config.toml"), []byte(documentWithRoot(t, target)))
	junction := filepath.Join(dir, "junction")
	cmd := exec.Command("cmd.exe", "/d", "/c", "mklink", "/J", junction, target)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("junction fixture unavailable: %v %s", err, output)
	}
	_, err := Load(Source{Role: Direct, Path: filepath.Join(junction, "config.toml")})
	assertCategory(t, err, CategoryUntrustedConfig)
}

func TestWindowsDACLRejectsUntrustedMutation(t *testing.T) {
	t.Parallel()
	acl := func(t *testing.T, principal string, mask uint32) []byte {
		t.Helper()
		sid, err := syscall.StringToSid(principal)
		if err != nil {
			t.Fatal(err)
		}
		data := make([]byte, 16+sid.Len())
		data[0] = 2
		binary.LittleEndian.PutUint16(data[2:], uint16(len(data)))
		binary.LittleEndian.PutUint16(data[4:], 1)
		binary.LittleEndian.PutUint16(data[10:], uint16(len(data)-8))
		binary.LittleEndian.PutUint32(data[12:], mask)
		copy(data[16:], unsafe.Slice((*byte)(unsafe.Pointer(sid)), sid.Len()))
		return data
	}
	const everyone = "S-1-1-0"
	for _, mask := range []uint32{0x2, 0x10, 0x40, 0x100, 0x10000, 0x40000, 0x80000, 0x40000000, 0x10000000} {
		if checkWindowsACL(acl(t, everyone, mask), "", false) == nil {
			t.Fatalf("untrusted file write mask accepted: %x", mask)
		}
	}
	if checkWindowsACL(acl(t, everyone, 0x40), "", true) == nil ||
		checkWindowsACL(acl(t, everyone, 0x4), "", true) != nil ||
		checkWindowsACL(acl(t, "S-1-5-18", 0x10000000), "", false) != nil {
		t.Fatal("directory replacement and authorized owner policies diverged")
	}
	if checkWindowsACL([]byte{0}, "", false) == nil {
		t.Fatal("truncated ACL accepted")
	}
}

func TestWindowsRejectsConfigWithUntrustedWriter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	user, err := windowsProcessSID()
	if err != nil {
		t.Fatal(err)
	}
	// Set the initial DACL on this new fixture only. The executing user
	// owns it and an unrelated principal is granted file-data write access.
	sddl, err := syscall.UTF16PtrFromString("O:" + user + "D:P(A;;FA;;;" + user + ")(A;;0x2;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	convert := syscall.NewLazyDLL("advapi32.dll").NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	var descriptor uintptr
	ok, _, callErr := convert.Call(uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&descriptor)), 0)
	if ok == 0 {
		t.Fatal(callErr)
	}
	t.Cleanup(func() {
		if remaining, err := syscall.LocalFree(syscall.Handle(descriptor)); remaining != 0 || err != nil {
			t.Error("releasing the fixture security descriptor failed")
		}
	})
	attributes := syscall.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_WRITE, 0, &attributes, syscall.CREATE_NEW, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	file := os.NewFile(uintptr(handle), "fixture")
	if _, err := file.WriteString(documentWithRoot(t, dir)); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = Load(Source{Role: Direct, Path: path})
	assertCategory(t, err, CategoryUntrustedConfig)
}

func TestWindowsRequiresExactSourceOwner(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	testconfig.Write(t, path, []byte(documentWithRoot(t, dir)))
	if _, err := Load(Source{Role: Direct, Path: path}); err != nil {
		t.Fatalf("user-owned configuration did not load: %v", err)
	}
	user, err := windowsProcessSID()
	if err != nil {
		t.Fatal("reading token user failed")
	}
	handle, err := openWindowsNode(path, user, false)
	if err != nil {
		t.Fatal("opening user-owned fixture failed")
	}
	defer func() { _ = syscall.CloseHandle(handle) }()

	// The same trusted handle is rejected for a different role identity.
	// Both alternatives are trusted principals, but neither stands in for
	// the exact owner of this direct configuration.
	other := "S-1-5-32-544"
	if other == user {
		other = "S-1-5-18"
	}
	privateErr := errors.New(`Win32 fixture details: C:\private\config.toml, S-1-1-0, descriptor 0x1234`)
	for _, tc := range []struct {
		name      string
		user      string
		remaining syscall.Handle
		freeErr   error
		want      error
	}{
		{name: "trusted owner and successful release", user: user},
		{name: "different role identity", user: other, want: os.ErrPermission},
		{name: "trusted owner and release error", user: user, freeErr: privateErr, want: errSecurityDescriptorRelease},
		{name: "trusted owner and nonzero release result", user: user, remaining: 1, want: errSecurityDescriptorRelease},
		{name: "different role identity and release failure", user: other, remaining: 1, freeErr: privateErr, want: os.ErrPermission},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got := windowsTrust(handle, tc.user, false, func(descriptor syscall.Handle) (syscall.Handle, error) {
				calls++
				if descriptor == 0 {
					t.Fatal("missing security descriptor")
				}
				if remaining, err := syscall.LocalFree(descriptor); remaining != 0 || err != nil {
					t.Fatal("native security descriptor release failed")
				}
				return tc.remaining, tc.freeErr
			})
			if calls != 1 {
				t.Fatalf("security descriptor release calls = %d, want 1", calls)
			}
			if got != tc.want {
				t.Fatal("unexpected trust result after security descriptor release")
			}
			if got != nil && sourceError(got).Error() != string(CategoryUntrustedConfig) {
				t.Fatal("source error exposed details outside its safe category")
			}
		})
	}
}
