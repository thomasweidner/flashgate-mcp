//go:build windows

package config

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

const (
	windowsServiceConfig = "C:\\ProgramData\\FlashGate\\" + defaultFileName
	readControl          = 0x00020000
	readAttributes       = 0x80
	fileReparsePoint     = 0x400
)

var (
	errSecurityDescriptorRelease = errors.New("could not release configuration security descriptor")

	// syscall registers both libraries as known system DLLs, so its loader
	// uses LOAD_LIBRARY_SEARCH_SYSTEM32 for these exact names.
	getSecurityInfo = syscall.NewLazyDLL("advapi32.dll").NewProc("GetSecurityInfo")
	getFinalPath    = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")
	getDriveType    = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")
)

func defaultPath(role Role) (string, error) {
	if role == SystemService {
		return windowsServiceConfig, nil
	}
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer token.Close()
	home, err := token.GetUserProfileDirectory()
	if err != nil {
		return "", err
	}
	if _, _, ok := splitWindowsPath(home); !ok {
		return "", os.ErrPermission
	}
	return filepath.Join(home, ".flashgate", defaultFileName), nil
}

func readTrustedConfig(path string, role Role) (trustedDocument, error) {
	var document trustedDocument
	err := withTrustedConfig(path, role, func(file *os.File) error {
		data, err := io.ReadAll(io.LimitReader(file, MaxDocumentBytes+1))
		if err != nil {
			return sourceError(err)
		}
		if len(data) > MaxDocumentBytes {
			return oversizedDocument()
		}
		info, err := file.Stat()
		if err != nil {
			return sourceError(err)
		}
		document = trustedDocument{data: data, info: info}
		return nil
	})
	if err != nil {
		return trustedDocument{}, err
	}
	return document, nil
}

func withTrustedConfig(path string, role Role, inspect func(*os.File) error) error {
	return withTrustedWindowsParents(path, role, func(current, userSID string) error {
		handle, err := openWindowsNode(current, userSID, false)
		if err != nil {
			return sourceError(err)
		}
		file := os.NewFile(uintptr(handle), defaultFileName)
		defer file.Close()
		if err := inspect(file); err != nil {
			return err
		}
		if err := verifyWindowsNode(handle, current, userSID, false); err != nil {
			return sourceError(err)
		}
		return nil
	})
}

// WithTrustedUserConfigParent validates and holds the same Windows directory
// chain as the direct configuration loader while use creates a fixture.
func WithTrustedUserConfigParent(path string, use func(string) error) error {
	return withTrustedWindowsParents(path, Direct, func(current, _ string) error {
		return use(current)
	})
}

func withTrustedWindowsParents(path string, role Role, use func(string, string) error) error {
	volume, components, ok := splitWindowsPath(path)
	if !ok {
		return invalidSourcePath()
	}
	root := volume + "\\"
	const fixedDrive = 3
	if windowsDriveKind(root) != fixedDrive {
		return invalidSourcePath()
	}

	userSID := ""
	var err error
	if role != SystemService {
		userSID, err = windowsProcessSID()
		if err != nil {
			return sourceError(err)
		}
	}
	// Keep verified directory handles open without delete sharing. The
	// validated names stay pinned while opening each successive component.
	directories := make([]syscall.Handle, 0, len(components))
	defer func() {
		for i := len(directories) - 1; i >= 0; i-- {
			_ = syscall.CloseHandle(directories[i])
		}
	}()
	current := root
	for _, component := range components {
		handle, err := openWindowsNode(current, userSID, true)
		if err != nil {
			return sourceError(err)
		}
		directories = append(directories, handle)
		current = filepath.Join(current, component)
	}
	return use(current, userSID)
}

func checkRootAliases(_ string, guard *security.PathGuard) error {
	volume := filepath.VolumeName(guard.EffectiveRoot())
	if len(volume) != 2 {
		return security.ErrProtectedPath
	}
	switch windowsDriveKind(volume + "\\") {
	case 2, 3, 5, 6: // Removable, fixed, optical, and RAM drives.
		return nil
	default:
		// Network mappings can alias local files without comparable IDs.
		return security.ErrProtectedPath
	}
}

func windowsDriveKind(root string) uintptr {
	if err := getDriveType.Find(); err != nil {
		return 0
	}
	rootPtr, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0
	}
	kind, _, _ := getDriveType.Call(uintptr(unsafe.Pointer(rootPtr)))
	return kind
}

func splitWindowsPath(path string) (string, []string, bool) {
	if len(path) < 4 || len(path) > 4096 || strings.IndexByte(path, 0) >= 0 {
		return "", nil, false
	}
	path = strings.ReplaceAll(path, "/", "\\")
	volume := filepath.VolumeName(path)
	if len(volume) != 2 || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", nil, false
	}
	parts := strings.Split(path[3:], "\\")
	for _, part := range parts {
		if !filepath.IsLocal(part) || strings.ContainsAny(part, ":<>\"|?*\r\n\t") ||
			strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || reservedWindowsName(part) {
			return "", nil, false
		}
	}
	return volume, parts, true
}

func reservedWindowsName(component string) bool {
	base, _, _ := strings.Cut(component, ".")
	base = strings.ToUpper(strings.TrimRight(base, " "))
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	for _, prefix := range []string{"COM", "LPT"} {
		if suffix, ok := strings.CutPrefix(base, prefix); ok {
			switch suffix {
			case "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
				return true
			}
		}
	}
	return false
}

func windowsProcessSID() (string, error) {
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return "", err
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", err
	}
	return user.User.Sid.String()
}

func openWindowsNode(path, userSID string, directory bool) (handle syscall.Handle, err error) {
	name, err := syscall.UTF16PtrFromString("\\\\?\\" + path)
	if err != nil {
		return 0, err
	}
	access := uint32(readControl | readAttributes)
	if !directory {
		access |= syscall.GENERIC_READ
	}
	handle, err = syscall.CreateFile(name, access, syscall.FILE_SHARE_READ, nil, syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_OPEN_REPARSE_POINT|syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) || errors.Is(err, syscall.ERROR_PATH_NOT_FOUND) {
		return 0, errMissingFile
	}
	if err != nil {
		return 0, err
	}
	if err := verifyWindowsNode(handle, path, userSID, directory); err != nil {
		_ = syscall.CloseHandle(handle)
		return 0, err
	}
	return handle, nil
}

func verifyWindowsNode(handle syscall.Handle, path, userSID string, directory bool) error {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil {
		return err
	}
	kind, err := syscall.GetFileType(handle)
	if err != nil || kind != syscall.FILE_TYPE_DISK || info.FileAttributes&fileReparsePoint != 0 ||
		directory != (info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0) ||
		!directory && info.NumberOfLinks != 1 {
		return os.ErrPermission
	}
	// Detect substituted drives and alternate resolved names as well as
	// reparse metadata. Compare normalized DOS paths, never user aliases.
	if err := getFinalPath.Find(); err != nil {
		return err
	}
	var finalName [4100]uint16
	length, _, _ := getFinalPath.Call(uintptr(handle), uintptr(unsafe.Pointer(&finalName[0])), uintptr(len(finalName)), 0)
	if length == 0 || length >= uintptr(len(finalName)) ||
		!strings.EqualFold(syscall.UTF16ToString(finalName[:length]), "\\\\?\\"+path) {
		return os.ErrPermission
	}
	return windowsTrust(handle, userSID, directory, syscall.LocalFree)
}

func windowsTrust(handle syscall.Handle, userSID string, directory bool, freeDescriptor func(syscall.Handle) (syscall.Handle, error)) (trustErr error) {
	if err := getSecurityInfo.Find(); err != nil {
		return err
	}
	const fileObject, ownerAndDACL = 1, 5
	var owner *syscall.SID
	var dacl *byte
	var descriptor syscall.Handle
	result, _, _ := getSecurityInfo.Call(uintptr(handle), fileObject, ownerAndDACL,
		uintptr(unsafe.Pointer(&owner)), 0, uintptr(unsafe.Pointer(&dacl)), 0, uintptr(unsafe.Pointer(&descriptor)))
	if result != 0 {
		return syscall.Errno(result)
	}
	defer func() {
		remaining, err := freeDescriptor(descriptor)
		if (remaining != 0 || err != nil) && trustErr == nil {
			trustErr = errSecurityDescriptorRelease
		}
	}()
	if owner == nil || dacl == nil {
		return os.ErrPermission
	}
	ownerID, err := owner.String()
	if err != nil || !trustedWindowsSID(ownerID, userSID) ||
		!directory && userSID != "" && ownerID != userSID {
		return os.ErrPermission
	}
	header := unsafe.Slice(dacl, 8)
	size := int(binary.LittleEndian.Uint16(header[2:4]))
	if size < len(header) {
		return os.ErrPermission
	}
	return checkWindowsACL(unsafe.Slice(dacl, size), userSID, directory)
}

func trustedWindowsSID(sid, userSID string) bool {
	switch sid {
	case "S-1-5-18", "S-1-5-32-544", "S-1-5-80-956008885-3418522649-1831038044-1853292631-2271478464":
		// SYSTEM, local Administrators and the Windows servicing identity.
		return true
	}
	return userSID != "" && sid == userSID
}

func checkWindowsACL(acl []byte, userSID string, directory bool) error {
	const (
		allowACE    = 0
		denyACE     = 1
		inheritOnly = 0x08
		// Delete/replace and ACL/ownership changes. Read-sharing also
		// blocks an open for directory data writes during the walk.
		mutation  = 0x000d0040 | 0x52000000
		writeFile = 0x0116
	)
	if len(acl) < 8 || int(binary.LittleEndian.Uint16(acl[2:4])) != len(acl) {
		return os.ErrPermission
	}
	mask := uint32(mutation)
	if !directory {
		mask |= writeFile
	}
	count := int(binary.LittleEndian.Uint16(acl[4:6]))
	remaining := acl[8:]
	for i := 0; i < count; i++ {
		if len(remaining) < 8 {
			return os.ErrPermission
		}
		size := int(binary.LittleEndian.Uint16(remaining[2:4]))
		if size < 8 || size > len(remaining) || size%4 != 0 {
			return os.ErrPermission
		}
		ace := remaining[:size]
		remaining = remaining[size:]
		if ace[1]&inheritOnly != 0 || ace[0] == denyACE {
			continue
		}
		if ace[0] != allowACE {
			return os.ErrPermission
		}
		if binary.LittleEndian.Uint32(ace[4:8])&mask == 0 {
			continue
		}
		sid := ace[8:]
		if len(sid) < 8 || sid[0] != 1 || sid[1] > 15 || 8+4*int(sid[1]) != len(sid) {
			return os.ErrPermission
		}
		sidID, err := (*syscall.SID)(unsafe.Pointer(&sid[0])).String()
		if err != nil || !trustedWindowsSID(sidID, userSID) && sidID != "S-1-3-4" {
			return os.ErrPermission
		}
	}
	return nil
}
