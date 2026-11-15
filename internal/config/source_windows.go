//go:build windows

package config

import (
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
	readAttributes       = 0x80
	fileReparsePoint     = 0x400
)

var (
	// syscall loads this system library from System32.
	getFinalPath = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")
	getDriveType = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")
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

func readTrustedConfig(path string, _ Role) (trustedDocument, error) {
	var document trustedDocument
	err := withTrustedConfig(path, func(file *os.File) error {
		before, err := file.Stat()
		if err != nil {
			return sourceError(err)
		}
		if before.Size() > MaxDocumentBytes {
			return oversizedDocument()
		}
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
		if before.Size() != int64(len(data)) || before.Size() != info.Size() ||
			!before.ModTime().Equal(info.ModTime()) {
			return invalidSourcePath()
		}
		document = trustedDocument{data: data, info: info}
		return nil
	})
	if err != nil {
		return trustedDocument{}, err
	}
	return document, nil
}

func withTrustedConfig(path string, inspect func(*os.File) error) error {
	return withTrustedWindowsParents(path, func(current string) error {
		handle, err := openWindowsNode(current, false)
		if err != nil {
			return sourceError(err)
		}
		file := os.NewFile(uintptr(handle), defaultFileName)
		defer file.Close()
		if err := inspect(file); err != nil {
			return err
		}
		if err := verifyWindowsNode(handle, current, false); err != nil {
			return sourceError(err)
		}
		return nil
	})
}

// WithTrustedUserConfigParent validates and holds the same Windows directory
// chain as the direct configuration loader while use creates a fixture.
func WithTrustedUserConfigParent(path string, use func(string) error) error {
	return withTrustedWindowsParents(path, use)
}

func withTrustedWindowsParents(path string, use func(string) error) error {
	volume, components, ok := splitWindowsPath(path)
	if !ok {
		return invalidSourcePath()
	}
	root := volume + "\\"
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
		handle, err := openWindowsNode(current, true)
		if err != nil {
			return sourceError(err)
		}
		directories = append(directories, handle)
		current = filepath.Join(current, component)
	}
	return use(current)
}

func checkRootAliases(source string, guard *security.PathGuard) error {
	// Network mappings can alias local files without comparable IDs.
	if !windowsLocalVolume(source) || !windowsLocalVolume(guard.EffectiveRoot()) {
		return security.ErrProtectedPath
	}
	return nil
}

func windowsLocalVolume(path string) bool {
	volume := filepath.VolumeName(path)
	if len(volume) != 2 {
		return false
	}
	switch windowsDriveKind(volume + "\\") {
	case 2, 3, 5, 6: // Removable, fixed, optical, and RAM drives.
		return true
	default:
		return false
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

func openWindowsNode(path string, directory bool) (handle syscall.Handle, err error) {
	name, err := syscall.UTF16PtrFromString("\\\\?\\" + path)
	if err != nil {
		return 0, err
	}
	access := uint32(readAttributes)
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
	if err := verifyWindowsNode(handle, path, directory); err != nil {
		_ = syscall.CloseHandle(handle)
		return 0, err
	}
	return handle, nil
}

func verifyWindowsNode(handle syscall.Handle, path string, directory bool) error {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil {
		return err
	}
	kind, err := syscall.GetFileType(handle)
	if err != nil || kind != syscall.FILE_TYPE_DISK || info.FileAttributes&fileReparsePoint != 0 ||
		directory != (info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0) {
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
	return nil
}
