//go:build linux

package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const linuxServiceConfig = "/etc/flashgate-mcp/" + defaultFileName

func defaultPath(role Role) (string, error) {
	if role == SystemService {
		return linuxServiceConfig, nil
	}
	uid := uint32(os.Geteuid())
	// Go's CGO-free os/user fallback can consult HOME when no account
	// record exists. Read only the local account mapping under root trust.
	accountFile, err := openLinuxFile("/etc/passwd", 0, true)
	if err != nil {
		return "", err
	}
	defer accountFile.Close()
	const maxAccountBytes = 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(accountFile, maxAccountBytes+1))
	if err != nil || len(data) > maxAccountBytes {
		return "", os.ErrPermission
	}
	home, err := linuxAccountHome(data, uid)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".flashgate", defaultFileName), nil
}

func linuxAccountHome(data []byte, uid uint32) (string, error) {
	var home string
	id := strconv.FormatUint(uint64(uid), 10)
	for line := range bytes.SplitSeq(data, []byte{'\n'}) {
		fields := bytes.Split(line, []byte{':'})
		if len(fields) != 7 || string(fields[2]) != id {
			continue
		}
		candidate := string(fields[5])
		if home != "" || !validLinuxPath(candidate) {
			return "", os.ErrPermission
		}
		home = candidate
	}
	if home == "" {
		return "", os.ErrPermission
	}
	return home, nil
}

func readTrustedConfig(path string, role Role) (trustedDocument, error) {
	var document trustedDocument
	err := withTrustedConfig(path, role, func(file *os.File) error {
		var err error
		document, err = readLinuxConfig(file)
		return err
	})
	if err != nil {
		return trustedDocument{}, err
	}
	return document, nil
}

func withTrustedConfig(path string, role Role, inspect func(*os.File) error) error {
	uid, system := uint32(os.Geteuid()), role == SystemService
	file, err := openLinuxFile(path, uid, system)
	if err != nil {
		return sourceError(err)
	}
	defer file.Close()
	if err := inspect(file); err != nil {
		return err
	}
	if err := verifyLinuxNode(int(file.Fd()), uid, system, false); err != nil {
		return sourceError(err)
	}
	return nil
}

func readLinuxConfig(file *os.File) (trustedDocument, error) {
	var before, after syscall.Stat_t
	if err := syscall.Fstat(int(file.Fd()), &before); err != nil {
		return trustedDocument{}, sourceError(err)
	}
	if before.Size > MaxDocumentBytes {
		return trustedDocument{}, oversizedDocument()
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxDocumentBytes+1))
	if err != nil {
		return trustedDocument{}, sourceError(err)
	}
	if len(data) > MaxDocumentBytes {
		return trustedDocument{}, oversizedDocument()
	}
	if err := syscall.Fstat(int(file.Fd()), &after); err != nil {
		return trustedDocument{}, sourceError(err)
	}
	if before.Size != int64(len(data)) || before.Size != after.Size || before.Mtim != after.Mtim ||
		before.Ctim != after.Ctim {
		return trustedDocument{}, invalidSourcePath()
	}
	info, err := file.Stat()
	if err != nil {
		return trustedDocument{}, sourceError(err)
	}
	return trustedDocument{data: data, info: info}, nil
}

func validLinuxPath(path string) bool {
	return len(path) <= 4096 && path != "/" && filepath.IsAbs(path) &&
		filepath.Clean(path) == path && strings.IndexByte(path, 0) < 0
}

// Every component is opened relative to the already verified parent. A
// concurrent name change cannot switch the parent used by the next open.
func openLinuxFile(path string, uid uint32, system bool) (*os.File, error) {
	if !validLinuxPath(path) {
		return nil, os.ErrPermission
	}
	const directoryFlags = syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
	parent, err := syscall.Open("/", directoryFlags, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = syscall.Close(parent) }()
	if err := verifyLinuxNode(parent, uid, system, true); err != nil {
		return nil, err
	}
	components := strings.Split(path[1:], "/")
	for _, component := range components[:len(components)-1] {
		child, err := syscall.Openat(parent, component, directoryFlags, 0)
		if err != nil {
			return nil, missingLinuxPath(err)
		}
		_ = syscall.Close(parent)
		parent = child
		if err := verifyLinuxNode(parent, uid, system, true); err != nil {
			return nil, err
		}
	}
	fd, err := syscall.Openat(parent, components[len(components)-1],
		syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, missingLinuxPath(err)
	}
	if err := verifyLinuxNode(fd, uid, system, false); err != nil {
		_ = syscall.Close(fd)
		return nil, err
	}
	return os.NewFile(uintptr(fd), defaultFileName), nil
}

func missingLinuxPath(err error) error {
	if errors.Is(err, syscall.ENOENT) {
		return errMissingFile
	}
	return err
}

func verifyLinuxNode(fd int, uid uint32, system, directory bool) error {
	var metadata syscall.Stat_t
	if err := syscall.Fstat(fd, &metadata); err != nil {
		return err
	}
	if !trustedLinuxNode(&metadata, uid, system, directory) {
		return os.ErrPermission
	}
	// Remote and user-space filesystems can synthesize owner and mode.
	var filesystem syscall.Statfs_t
	if err := syscall.Fstatfs(fd, &filesystem); err != nil {
		return err
	}
	switch uint32(filesystem.Type) {
	case 0xef53, 0x58465342, 0x9123683e, 0x01021994, 0x794c7630, 0x2fc12fc1, 0xf2f52010:
		// ext family, XFS, Btrfs, tmpfs, OverlayFS, ZFS, F2FS.
		return nil
	default:
		return os.ErrPermission
	}
}

func trustedLinuxNode(st *syscall.Stat_t, uid uint32, system, directory bool) bool {
	if directory {
		trustedOwner := st.Uid == 0 || !system && st.Uid == uid
		replacementProtected := st.Mode&0o022 == 0 || st.Mode&syscall.S_ISVTX != 0
		return st.Mode&syscall.S_IFMT == syscall.S_IFDIR && trustedOwner && replacementProtected
	}
	owner := uid
	if system {
		owner = 0
	}
	return st.Mode&syscall.S_IFMT == syscall.S_IFREG && st.Nlink == 1 && st.Uid == owner && st.Mode&0o022 == 0
}
