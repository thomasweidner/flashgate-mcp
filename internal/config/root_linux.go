//go:build linux

package config

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

type linuxMount struct {
	device, root, point, filesystem string
}

func checkRootAliases(source string, guard *security.PathGuard) error {
	table, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return security.ErrProtectedPath
	}
	defer table.Close()
	var filesystem syscall.Statfs_t
	if syscall.Fstatfs(int(table.Fd()), &filesystem) != nil || filesystem.Type != 0x9fa0 {
		return security.ErrProtectedPath
	}
	// Resolve mountpoint spelling through the kernel, including casefolded
	// directory names. These handles carry metadata access only.
	source, err = linuxMountPath(source)
	if err != nil {
		return security.ErrProtectedPath
	}
	root, err := linuxMountPath(guard.EffectiveRoot())
	if err != nil {
		return security.ErrProtectedPath
	}
	return checkLinuxMounts(table, source, root)
}

func linuxMountPath(path string) (string, error) {
	const pathOnly = 0x200000 // Linux O_PATH
	fd, err := syscall.Open(path, pathOnly|syscall.O_CLOEXEC, 0)
	if err != nil {
		return "", err
	}
	defer syscall.Close(fd)
	resolved, err := os.Readlink("/proc/self/fd/" + strconv.Itoa(fd))
	if err != nil || !filepath.IsAbs(resolved) || strings.HasSuffix(resolved, " (deleted)") {
		return "", security.ErrProtectedPath
	}
	return resolved, nil
}

func checkLinuxMounts(table io.Reader, source, root string) error {
	const maxTableBytes = 4 * 1024 * 1024
	input := &io.LimitedReader{R: table, N: maxTableBytes + 1}
	scanner := bufio.NewScanner(input)
	var mounts []linuxMount
	for scanner.Scan() {
		mount, ok := parseLinuxMount(scanner.Text())
		if !ok {
			return security.ErrProtectedPath
		}
		mounts = append(mounts, mount)
	}
	if scanner.Err() != nil || input.N == 0 {
		return security.ErrProtectedPath
	}
	sourceMount, ok := coveringMount(mounts, source)
	if !ok {
		return security.ErrProtectedPath
	}
	rootMount, ok := coveringMount(mounts, root)
	if !ok {
		return security.ErrProtectedPath
	}
	relative, _ := mountRelative(sourceMount.point, source)
	location := filepath.Join(sourceMount.root, relative)
	for _, mount := range mounts {
		_, nested := mountRelative(root, mount.point)
		if mount != rootMount && !nested {
			continue
		}
		if !comparableLinuxMount(sourceMount, mount) {
			return security.ErrProtectedPath
		}
		// Btrfs subvolumes can appear under a top-level mount and also as
		// separate mounts with different device numbers.
		if mount.device == sourceMount.device || mount.filesystem == "btrfs" && sourceMount.filesystem == "btrfs" {
			if rel, ok := mountRelative(mount.root, location); ok {
				if _, inside := mountRelative(root, filepath.Join(mount.point, rel)); inside {
					return security.ErrProtectedPath
				}
			}
		}
	}
	return nil
}

func parseLinuxMount(line string) (linuxMount, bool) {
	left, right, ok := strings.Cut(line, " - ")
	fields := strings.Split(left, " ")
	filesystem, _, _ := strings.Cut(right, " ")
	if !ok || len(fields) < 6 || fields[2] == "" || filesystem == "" {
		return linuxMount{}, false
	}
	root, point := unescapeMountPath.Replace(fields[3]), unescapeMountPath.Replace(fields[4])
	if !filepath.IsAbs(root) || !filepath.IsAbs(point) {
		return linuxMount{}, false
	}
	return linuxMount{fields[2], root, point, filesystem}, true
}

var unescapeMountPath = strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, "\\")

func coveringMount(mounts []linuxMount, path string) (linuxMount, bool) {
	var best linuxMount
	ambiguous := false
	for _, mount := range mounts {
		if _, ok := mountRelative(mount.point, path); !ok {
			continue
		}
		switch {
		case len(mount.point) > len(best.point):
			best, ambiguous = mount, false
		case mount.point == best.point:
			ambiguous = true
		}
	}
	return best, best.point != "" && !ambiguous
}

// mountRelative projects a namespace path onto its filesystem mount root.
func mountRelative(parent, name string) (string, bool) {
	if parent == name {
		return ".", true
	}
	return strings.CutPrefix(name, strings.TrimSuffix(parent, "/")+"/")
}

func comparableLinuxMount(source, data linuxMount) bool {
	switch data.filesystem {
	case "ext2", "ext3", "ext4", "xfs", "btrfs", "tmpfs", "zfs", "f2fs", "overlay":
		// A distinct overlay can present another filesystem's backing files.
		return source.device == data.device || source.filesystem != "overlay" && data.filesystem != "overlay"
	default:
		return false
	}
}
