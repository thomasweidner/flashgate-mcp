package fs

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
)

func (f *LocalFileSystem) checksIdentity() bool {
	return f.protection != nil || f.fileTypes.Restricted()
}

func (f *LocalFileSystem) validateFileOperandSpelling(raw string) error {
	name := filepath.Base(raw)
	// Leave empty/navigation/root handling to the existing PathGuard contract.
	if strings.TrimSpace(raw) == "" || name == "." || name == ".." || filepath.IsAbs(raw) || filepath.VolumeName(raw) != "" {
		return nil
	}
	if f.fileTypes.Restricted() && !config.ValidFileBasename(name) || ambiguousNativeFileName(name) {
		return ErrFileTypeDenied
	}
	return nil
}

// Keep the caller's basename as well as the resolved one. Native absolute-path
// conversion can erase a Windows trailing dot before returning a SafePath.
func (f *LocalFileSystem) checkOperandName(raw string, operand effectPath, prospective bool) error {
	if operand.info != nil && operand.info.IsDir() || operand.info == nil && !prospective {
		return nil
	}
	name := filepath.Base(raw)
	if !f.fileTypes.AllowsName(name) || ambiguousNativeFileName(name) {
		return ErrFileTypeDenied
	}
	return nil
}

// checkFileType never opens contents. Directories are structural operands;
// absent leaves are checked as files only when an operation creates a file.
func (f *LocalFileSystem) checkFileType(operand effectPath, prospective bool) error {
	if operand.info != nil {
		if operand.info.IsDir() {
			return nil
		}
		if !operand.info.Mode().IsRegular() {
			return ErrFileTypeDenied
		}
	} else if !prospective {
		return nil
	}
	effective := operand.effective
	if effective == "" {
		var err error
		effective, err = effectivePathForComparison(operand.path)
		if err != nil {
			return ErrFileTypeDenied
		}
	}
	for _, name := range []string{filepath.Base(operand.path), filepath.Base(effective)} {
		if !f.fileTypes.AllowsName(name) || ambiguousNativeFileName(name) {
			return ErrFileTypeDenied
		}
	}
	if operand.info != nil && (!f.fileTypes.AllowsName(operand.info.Name()) || ambiguousNativeFileName(operand.info.Name())) {
		return ErrFileTypeDenied
	}
	return nil
}

func ambiguousNativeFileName(name string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	// Native Windows mappings are not made safe by unrestricted matching.
	// In particular, reject ADS and trailing-dot/space normalization before I/O.
	if name == "" || !utf8.ValidString(name) || strings.ContainsAny(name, `<>:"/\|?*`) ||
		strings.HasPrefix(name, " ") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return true
	}
	for _, ch := range name {
		if ch < 0x20 {
			return true
		}
	}
	base, _, _ := strings.Cut(strings.ToUpper(name), ".")
	// Device recognition discards stem spaces only to deny an alias, never to
	// repair a filename into an extension match.
	base = strings.TrimRight(base, " ")
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return true
	}
	if strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT") {
		suffix := base[3:]
		return len(suffix) == 1 && suffix[0] >= '1' && suffix[0] <= '9' || suffix == "¹" || suffix == "²" || suffix == "³"
	}
	return false
}

// A relative link moved to another parent has no proved future target. Keep
// this route denied under a restriction rather than classify its old target.
func (f *LocalFileSystem) checkLinkProjection(source, target effectPath) error {
	if !f.fileTypes.Restricted() {
		return nil
	}
	entry, err := os.Lstat(source.path)
	if err != nil {
		return ErrFileTypeDenied
	}
	if entry.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	link, err := os.Readlink(source.path)
	if err != nil || (!filepath.IsAbs(link) && !sameEffectInfo(source.parent, target.parent)) {
		return ErrFileTypeDenied
	}
	return nil
}
