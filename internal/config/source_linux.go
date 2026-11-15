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
	accountFile, err := openLinuxFile("/etc/passwd", true)
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

func readTrustedConfig(path string, _ Role) (trustedDocument, error) {
	var document trustedDocument
	err := withTrustedConfig(path, func(file *os.File) error {
		var err error
		document, err = readLinuxConfig(file)
		return err
	})
	if err != nil {
		return trustedDocument{}, err
	}
	return document, nil
}

func withTrustedConfig(path string, inspect func(*os.File) error) error {
	file, err := openLinuxFile(path, false)
	if err != nil {
		return sourceError(err)
	}
	defer file.Close()
	if err := inspect(file); err != nil {
		return err
	}
	current, err := os.Lstat(path)
	if err != nil || !current.Mode().IsRegular() {
		return invalidSourcePath()
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(current, opened) {
		return invalidSourcePath()
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
func openLinuxFile(path string, accountDatabase bool) (*os.File, error) {
	if !validLinuxPath(path) {
		return nil, os.ErrPermission
	}
	directoryFlags := syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
	if !accountDatabase {
		// The TOML walk needs searchable directories only. openat still
		// checks search permission when looking up a child of an O_PATH fd.
		const pathOnly = 0x200000 // Linux O_PATH
		directoryFlags |= pathOnly
	}
	parent, err := syscall.Open("/", directoryFlags, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = syscall.Close(parent) }()
	if accountDatabase {
		if err := checkLinuxAccountNode(parent, syscall.S_IFDIR); err != nil {
			return nil, err
		}
	}
	components := strings.Split(path[1:], "/")
	for _, component := range components[:len(components)-1] {
		child, err := syscall.Openat(parent, component, directoryFlags, 0)
		if err != nil {
			return nil, missingLinuxPath(err)
		}
		_ = syscall.Close(parent)
		parent = child
		if accountDatabase {
			if err := checkLinuxAccountNode(parent, syscall.S_IFDIR); err != nil {
				return nil, err
			}
		}
	}
	fd, err := syscall.Openat(parent, components[len(components)-1],
		syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, missingLinuxPath(err)
	}
	file := os.NewFile(uintptr(fd), defaultFileName)
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, os.ErrPermission
	}
	if accountDatabase {
		err = checkLinuxAccountNode(fd, syscall.S_IFREG)
	}
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func missingLinuxPath(err error) error {
	if errors.Is(err, syscall.ENOENT) {
		return errMissingFile
	}
	return err
}

// The local account database determines the home directory for a default
// source. Its system integrity is separate from the selected TOML policy.
func checkLinuxAccountNode(fd int, kind uint32) error {
	var metadata syscall.Stat_t
	if err := syscall.Fstat(fd, &metadata); err != nil {
		return err
	}
	if metadata.Mode&syscall.S_IFMT != kind || metadata.Uid != 0 || metadata.Mode&0o022 != 0 {
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
