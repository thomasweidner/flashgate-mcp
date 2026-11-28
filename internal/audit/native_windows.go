//go:build windows

package audit

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

var (
	lockFileEx        = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")
	unlockFileEx      = syscall.NewLazyDLL("kernel32.dll").NewProc("UnlockFileEx")
	getSecurityInfo   = syscall.NewLazyDLL("advapi32.dll").NewProc("GetSecurityInfo")
	getACE            = syscall.NewLazyDLL("advapi32.dll").NewProc("GetAce")
	convertDescriptor = syscall.NewLazyDLL("advapi32.dll").NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")
	getFinalName      = syscall.NewLazyDLL("kernel32.dll").NewProc("GetFinalPathNameByHandleW")
)

func lockFile(file *os.File, exclusive bool) (bool, error) {
	const failImmediately = 1
	const exclusiveLock = 2
	flags := uintptr(failImmediately)
	if exclusive {
		flags |= exclusiveLock
	}
	// The coordination range is beyond the data kept in the lock file.
	overlap := syscall.Overlapped{OffsetHigh: 1}
	result, _, err := lockFileEx.Call(file.Fd(), flags, 0, 1, 0, uintptr(unsafe.Pointer(&overlap)))
	runtime.KeepAlive(file)
	if result != 0 {
		return true, nil
	}
	if errors.Is(err, syscall.Errno(33)) {
		return false, nil
	}
	return false, err
}

func unlockFile(file *os.File) {
	overlap := syscall.Overlapped{OffsetHigh: 1}
	_, _, _ = unlockFileEx.Call(file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&overlap)))
	runtime.KeepAlive(file)
}

func nativeSamePath(a, b string) bool { return strings.EqualFold(a, b) }

func makeStoreDirectory(parent *os.Root, name string) error {
	if dir, err := parent.Lstat(name); err == nil && dir != nil {
		return os.ErrExist
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	basedir := parent.Name()
	volume := filepath.VolumeName(basedir)
	if len(volume) != 2 {
		return ErrUnavailable
	}
	// Pin every component without delete sharing during the only absolute
	// create. The ACL is supplied at creation and inherited by owned entries.
	dirs, err := pinStoreParents(basedir)
	if err != nil {
		return err
	}
	defer func() {
		for _, dir := range dirs {
			_ = syscall.CloseHandle(dir)
		}
	}()
	attributes, err := storeSecurityAttributes(true)
	if err != nil {
		return err
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(attributes.SecurityDescriptor)) }()
	full, err := syscall.UTF16PtrFromString(`\\?\` + filepath.Join(basedir, name))
	if err != nil {
		return err
	}
	return syscall.CreateDirectory(full, &attributes)
}

func storeSecurityAttributes(directory bool) (syscall.SecurityAttributes, error) {
	actor, err := currentSID()
	if err != nil {
		return syscall.SecurityAttributes{}, err
	}
	var inheritance string
	if directory {
		inheritance = "OICI"
	}
	sddl, err := syscall.UTF16PtrFromString("O:" + actor + "D:P(A;" + inheritance + ";FA;;;" + actor + ")")
	if err != nil {
		return syscall.SecurityAttributes{}, err
	}
	var descriptor uintptr
	ok, _, callErr := convertDescriptor.Call(uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&descriptor)), 0)
	if ok == 0 {
		return syscall.SecurityAttributes{}, callErr
	}
	attributes := syscall.SecurityAttributes{SecurityDescriptor: descriptor}
	attributes.Length = uint32(unsafe.Sizeof(attributes))
	return attributes, nil
}

func makeStorePartition(root *os.Root, name string) error {
	_, err := createStoreChild(root, name, true)
	return err
}

func createStoreFile(root *os.Root, name string) (*os.File, error) {
	return createStoreChild(root, name, false)
}

func createStoreChild(root *os.Root, name string, directory bool) (*os.File, error) {
	if name == "." || filepath.Clean(name) != name || !filepath.IsLocal(name) {
		return nil, ErrUnavailable
	}
	if _, err := root.Lstat(name); err == nil {
		return nil, os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	parentName := filepath.Dir(name)
	parent, err := secureOpen(root, parentName, os.O_RDONLY, 0, true)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	dirs, err := pinStoreParents(filepath.Join(root.Name(), parentName))
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, dir := range dirs {
			_ = syscall.CloseHandle(dir)
		}
	}()
	// The absolute create must address the same directory as the store root's
	// verified handle. Pinning prevents replacement during the create.
	if !sameStoreParent(dirs[len(dirs)-1], parent) {
		return nil, ErrUnavailable
	}
	// Inheritable ACLs alone leave the owner to the access token's default,
	// which can be an administrative group instead of the runtime account.
	attributes, err := storeSecurityAttributes(directory)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = syscall.LocalFree(syscall.Handle(attributes.SecurityDescriptor)) }()
	fullPath := filepath.Join(root.Name(), name)
	full, err := syscall.UTF16PtrFromString(`\\?\` + fullPath)
	if err != nil {
		return nil, err
	}
	if directory {
		return nil, syscall.CreateDirectory(full, &attributes)
	}
	handle, err := syscall.CreateFile(full, syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		&attributes, syscall.CREATE_NEW, syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(handle), fullPath)
	if !ownedNode(file, false) {
		file.Close()
		return nil, ErrUnavailable
	}
	return file, nil
}

func sameStoreParent(pinned syscall.Handle, bound *os.File) bool {
	var pathInfo, boundInfo syscall.ByHandleFileInformation
	if syscall.GetFileInformationByHandle(pinned, &pathInfo) != nil {
		return false
	}
	err := syscall.GetFileInformationByHandle(syscall.Handle(bound.Fd()), &boundInfo)
	runtime.KeepAlive(bound)
	return err == nil && pathInfo.VolumeSerialNumber == boundInfo.VolumeSerialNumber &&
		pathInfo.FileIndexHigh == boundInfo.FileIndexHigh && pathInfo.FileIndexLow == boundInfo.FileIndexLow
}

func pinStoreParents(path string) (handles []syscall.Handle, err error) {
	volume := filepath.VolumeName(path)
	parent := volume + "\\"
	if path == parent {
		return nil, ErrUnavailable
	}
	defer func() {
		if err != nil {
			for _, handle := range handles {
				_ = syscall.CloseHandle(handle)
			}
			handles = nil
		}
	}()
	for _, part := range strings.Split(strings.TrimPrefix(path, parent), "\\") {
		parent = filepath.Join(parent, part)
		text := `\\?\` + parent
		ptr, err := syscall.UTF16PtrFromString(text)
		if err != nil {
			return handles, err
		}
		handle, err := syscall.CreateFile(ptr, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING,
			syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err != nil {
			return handles, err
		}
		handles = append(handles, handle)
		var info syscall.ByHandleFileInformation
		var resolved [4100]uint16
		length, _, _ := getFinalName.Call(uintptr(handle), uintptr(unsafe.Pointer(&resolved[0])), uintptr(len(resolved)), 0)
		if syscall.GetFileInformationByHandle(handle, &info) != nil ||
			info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 ||
			info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 ||
			length == 0 || length >= uintptr(len(resolved)) ||
			!nativeSamePath(text, syscall.UTF16ToString(resolved[:length])) {
			return handles, ErrUnavailable
		}
	}
	return handles, nil
}

func secureOpen(root *os.Root, name string, flags int, mode os.FileMode, directory bool) (*os.File, error) {
	if name != "." && !filepath.IsLocal(name) {
		return nil, ErrUnavailable
	}
	parentName := "."
	for _, part := range strings.Split(filepath.Dir(name), string(filepath.Separator)) {
		if part == "." {
			continue
		}
		// Open each classified component in order. File append can never
		// use an unverified final link or an unverified directory handle.
		parentName = filepath.Join(parentName, part)
		parent, err := root.OpenFile(parentName, syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
		if err != nil {
			return nil, err
		}
		ok := ownedNode(parent, true)
		parent.Close()
		if !ok {
			return nil, ErrUnavailable
		}
	}
	file, err := root.OpenFile(name, flags|syscall.FILE_FLAG_OPEN_REPARSE_POINT, mode)
	if err != nil {
		return nil, err
	}
	if !ownedNode(file, directory) {
		file.Close()
		return nil, ErrUnavailable
	}
	return file, nil
}

func ownedNode(file *os.File, directory bool) bool {
	info, err := file.Stat()
	if err != nil || info.IsDir() != directory || !directory && !info.Mode().IsRegular() {
		return false
	}
	var node syscall.ByHandleFileInformation
	handle := syscall.Handle(file.Fd())
	kind, err := syscall.GetFileType(handle)
	return err == nil && kind == syscall.FILE_TYPE_DISK &&
		syscall.GetFileInformationByHandle(handle, &node) == nil &&
		node.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 &&
		(directory || node.NumberOfLinks == 1) && privateACL(file)
}

var currentSID = sync.OnceValues(loadCurrentSID)

func loadCurrentSID() (string, error) {
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

func privateACL(file *os.File) bool {
	current, err := currentSID()
	if err != nil {
		return false
	}
	type acl struct {
		revision, reserved uint8
		size, count, tail  uint16
	}
	var owner *syscall.SID
	var dacl *acl
	var descriptor syscall.Handle
	const fileObject = 1
	const ownerAndDACL = 5
	result, _, _ := getSecurityInfo.Call(file.Fd(), fileObject, ownerAndDACL,
		uintptr(unsafe.Pointer(&owner)), 0, uintptr(unsafe.Pointer(&dacl)), 0,
		uintptr(unsafe.Pointer(&descriptor)))
	runtime.KeepAlive(file)
	if descriptor != 0 {
		defer func() { _, _ = syscall.LocalFree(descriptor) }()
	}
	if result != 0 || owner == nil || dacl == nil || dacl.count > 128 {
		return false
	}
	actual, err := owner.String()
	if err != nil || actual != current {
		return false
	}
	type aceHeader struct {
		kind, flags uint8
		size        uint16
		mask        uint32
	}
	for i := range dacl.count {
		var ace *aceHeader
		ok, _, _ := getACE.Call(uintptr(unsafe.Pointer(dacl)), uintptr(i), uintptr(unsafe.Pointer(&ace)))
		if ok == 0 || ace == nil || ace.size < 16 {
			return false
		}
		switch ace.kind {
		case 0: // ACCESS_ALLOWED_ACE
			sid := (*syscall.SID)(unsafe.Add(unsafe.Pointer(ace), unsafe.Sizeof(aceHeader{})))
			account, err := sid.String()
			if err != nil || account != current && account != "S-1-5-18" && account != "S-1-5-32-544" {
				return false
			}
		case 1: // ACCESS_DENIED_ACE
		default:
			return false
		}
	}
	return true
}

// FlushFileBuffers confirms segment data. Windows provides no equivalent
// documented directory fsync through the normal directory handle.
func syncDirectory(*os.Root, string) error { return nil }
