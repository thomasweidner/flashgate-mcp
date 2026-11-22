//go:build windows

package security

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	fileAttributeHidden       = uint32(0x2)
	fileAttributeReparsePoint = uint32(0x400)
)

func isUNCPath(path string) bool {
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) {
		return true
	}

	volume := filepath.VolumeName(path)
	return strings.HasPrefix(volume, `\\`) || strings.HasPrefix(volume, `//`)
}

func validatePathMetadata(path string, policy Policy) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	attributes, err := fileAttributes(info)
	if err != nil {
		return err
	}

	if !policy.AllowHiddenFiles && attributes&fileAttributeHidden != 0 {
		return ErrHiddenPathDenied
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0
	isReparse := attributes&fileAttributeReparsePoint != 0

	if isReparse && !isSymlink {
		return ErrReparsePointDenied
	}
	if isSymlink && !policy.FollowSymlinks {
		return ErrSymlinkDenied
	}
	if isReparse {
		if _, err := nativeLinkTarget(path); err != nil {
			return err
		}
	}

	if isSymlink {
		_, err := relativeLinkTarget(path)
		return err
	}

	return nil
}

func fileAttributes(info os.FileInfo) (uint32, error) {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || data == nil {
		return 0, ErrReparsePointDenied
	}

	return data.FileAttributes, nil
}

func validateNativeRootMetadata(info os.FileInfo, policy Policy) error {
	attributes, err := fileAttributes(info)
	if err != nil {
		return err
	}
	if !policy.AllowHiddenFiles && attributes&fileAttributeHidden != 0 {
		return ErrHiddenPathDenied
	}
	return nil
}

func samePolicyMetadata(left, right os.FileInfo) bool {
	l, lerr := fileAttributes(left)
	r, rerr := fileAttributes(right)
	return lerr == nil && rerr == nil && (l^r)&fileAttributeHidden == 0
}

var attributeTagInfo = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFileInformationByHandleEx")
var exclusionFinalPath = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")
var exclusionVolumeInfo = syscall.NewLazyDLL("kernel32.dll").NewProc("GetVolumeInformationByHandleW")

// WindowsExclusionVolume binds a drive root to a local NTFS volume GUID.
// Substituted drives and redirected roots cannot masquerade as a volume root.
func WindowsExclusionVolume(driveRoot string) (string, error) {
	dir, err := openExclusionDirectory(driveRoot)
	if err != nil {
		return "", err
	}
	defer func() { _ = syscall.CloseHandle(dir) }()
	var fsName [32]uint16
	ok, _, _ := exclusionVolumeInfo.Call(uintptr(dir), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&fsName[0])), uintptr(len(fsName)))
	if ok == 0 || syscall.UTF16ToString(fsName[:]) != "NTFS" {
		return "", os.ErrPermission
	}
	var name [64]uint16
	// VOLUME_NAME_GUID (1), with FILE_NAME_NORMALIZED (0).
	n, _, _ := exclusionFinalPath.Call(uintptr(dir), uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)), 1)
	if n == 0 || n >= uintptr(len(name)) {
		return "", os.ErrPermission
	}
	path := syscall.UTF16ToString(name[:n])
	if !strings.HasPrefix(path, `\\?\Volume{`) || !strings.HasSuffix(path, `}\`) || len(path) != len(`\\?\Volume{00000000-0000-0000-0000-000000000000}\`) {
		return "", os.ErrPermission
	}
	return strings.ToUpper(path), nil // the volume GUID is ASCII
}

// WindowsExclusionCase classifies lookup in the given existing directory.
func WindowsExclusionCase(path string) (bool, error) {
	dir, err := openExclusionDirectory(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = syscall.CloseHandle(dir) }()
	var flags uint32
	// FILE_CASE_SENSITIVE_INFO uses FileCaseSensitiveInfo (23).
	ok, _, _ := attributeTagInfo.Call(uintptr(dir), 23, uintptr(unsafe.Pointer(&flags)), unsafe.Sizeof(flags))
	if ok == 0 || flags&^uint32(1) != 0 {
		return false, os.ErrPermission
	}
	return flags == 0, nil
}

func openExclusionDirectory(path string) (syscall.Handle, error) {
	if !filepath.IsAbs(path) || len(filepath.VolumeName(path)) != 2 {
		return syscall.InvalidHandle, os.ErrPermission
	}
	name, err := syscall.UTF16PtrFromString(`\\?\` + path)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	const fileReadAttributes = 0x80
	return syscall.CreateFile(name, fileReadAttributes, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|
		syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
}

// WindowsExclusionName looks up the actual entry spelling without opening
// file contents or following the final reparse point.
func WindowsExclusionName(path string) (string, error) {
	nativeName := `\\?\` + path
	if isUNCPath(path) {
		nativeName = `\\?\UNC\` + strings.TrimLeft(path, `\`)
	}
	name, err := syscall.UTF16PtrFromString(nativeName)
	if err != nil {
		return "", err
	}
	var data syscall.Win32finddata
	handle, err := syscall.FindFirstFile(name, &data)
	if err != nil {
		return "", err
	}
	if err := syscall.FindClose(handle); err != nil {
		return "", err
	}
	return syscall.UTF16ToString(data.FileName[:]), nil
}

func nativeLinkTarget(path string) (string, error) {
	namePath := path
	if filepath.IsAbs(path) && len(filepath.VolumeName(path)) == 2 {
		namePath = `\\?\` + path
	}
	name, err := syscall.UTF16PtrFromString(namePath)
	if err != nil {
		return "", ErrReparsePointDenied
	}
	h, err := syscall.CreateFile(name, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OPEN_REPARSE_POINT|syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", ErrReparsePointDenied
	}
	defer func() { _ = syscall.CloseHandle(h) }()
	var tag struct{ Attributes, Tag uint32 }
	ok, _, _ := attributeTagInfo.Call(uintptr(h), 9, uintptr(unsafe.Pointer(&tag)), unsafe.Sizeof(tag))
	if ok == 0 || tag.Attributes&fileAttributeReparsePoint == 0 || tag.Tag != 0xa000000c {
		return "", ErrReparsePointDenied
	}
	var buffer [16 * 1024]byte
	var returned uint32
	if syscall.DeviceIoControl(h, 0x000900a8, nil, 0, &buffer[0], uint32(len(buffer)), &returned, nil) != nil || returned > uint32(len(buffer)) {
		return "", ErrReparsePointDenied
	}
	return relativeWindowsReparseTarget(buffer[:returned])
}

// Classify the actual substitute name and relative flag, not the optional
// human-readable print name. Unknown flags and malformed UTF-16 fail closed.
func relativeWindowsReparseTarget(data []byte) (string, error) {
	if len(data) < 20 || binary.LittleEndian.Uint32(data[:4]) != 0xa000000c {
		return "", ErrReparsePointDenied
	}
	flags := binary.LittleEndian.Uint32(data[16:20])
	if flags == 0 {
		return "", ErrSymlinkDenied
	}
	if flags != 1 {
		return "", ErrReparsePointDenied
	}
	end := 8 + int(binary.LittleEndian.Uint16(data[4:6]))
	offset := int(binary.LittleEndian.Uint16(data[8:10]))
	length := int(binary.LittleEndian.Uint16(data[10:12]))
	if end > len(data) || end < 20 || offset%2 != 0 || length == 0 || length%2 != 0 || 20+offset+length > end {
		return "", ErrReparsePointDenied
	}
	units := make([]uint16, length/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[20+offset+2*i:])
		if units[i] == 0 {
			return "", ErrReparsePointDenied
		}
	}
	for i := 0; i < len(units); i++ {
		u := units[i]
		if u >= 0xd800 && u <= 0xdbff {
			if i+1 >= len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
				return "", ErrReparsePointDenied
			}
			i++
		} else if u >= 0xdc00 && u <= 0xdfff {
			return "", ErrReparsePointDenied
		}
	}
	target := string(utf16.Decode(units))
	if filepath.IsAbs(target) || filepath.VolumeName(target) != "" || strings.HasPrefix(target, `\`) || strings.HasPrefix(target, "/") || isUNCPath(target) {
		return "", ErrSymlinkDenied
	}
	return target, nil
}

func evalEffectivePath(path string) (string, error) {
	return evalEffectivePathDepth(path, 0)
}

func evalEffectivePathDepth(path string, depth int) (string, error) {
	if depth > 255 {
		return "", errors.New("too many links")
	}

	absolutePath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}

	volume := filepath.VolumeName(absolutePath)
	remainder := strings.TrimPrefix(absolutePath, volume)
	remainder = strings.TrimLeft(remainder, `\/`)

	current := volume
	if current == "" {
		current = string(filepath.Separator)
	} else if strings.HasPrefix(absolutePath[len(volume):], `\`) || strings.HasPrefix(absolutePath[len(volume):], `/`) {
		current += string(filepath.Separator)
	}

	if remainder == "" {
		return filepath.Clean(current), nil
	}

	parts := strings.Split(remainder, string(filepath.Separator))
	for index := 0; index < len(parts); index++ {
		if depth > 255 {
			return "", errors.New("too many links")
		}
		part := parts[index]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			current = filepath.Dir(current)
			continue
		}

		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}

		if info.Mode()&os.ModeSymlink == 0 {
			// FindFirstFile returns the actual directory entry spelling for
			// an 8.3 alias without opening file contents or following a leaf
			// reparse point. Retain that spelling for BL-106 classification.
			name, err := WindowsExclusionName(current)
			if err != nil {
				return "", err
			}
			current = filepath.Join(filepath.Dir(current), name)
			if index+1 < len(parts) && !info.IsDir() {
				return "", ErrRootNotDirectory
			}
			continue
		}

		target, err := nativeLinkTarget(current)
		if err != nil {
			return "", err
		}
		// Resolve target components before their following ".." or suffix.
		// Joining the whole target here would clean across unresolved links.
		parts = append(strings.Split(filepath.FromSlash(target), string(filepath.Separator)), parts[index+1:]...)
		current = filepath.Dir(current)
		index = -1
		depth++
	}

	return filepath.Clean(current), nil
}
