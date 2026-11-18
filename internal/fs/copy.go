package fs

import (
	"io"
	"os"
)

// Copy copies a file. Directory copy is intentionally unsupported.
func (f *LocalFileSystem) Copy(source string, target string, overwrite bool, admission ...func() error) error {
	f.effects.Lock()
	defer f.effects.Unlock()
	for _, path := range []string{source, target} {
		if err := f.validateFileOperandSpelling(path); err != nil {
			return err
		}
	}

	sourcePath, err := f.guard.ResolveExisting(source)
	if err != nil {
		return err
	}

	targetPath, err := f.guard.ResolveForCreate(target)
	if err != nil {
		return err
	}

	sourceOperand, err := f.inspectEffect(sourcePath.String(), false)
	if err != nil {
		return err
	}
	sourceInfo := sourceOperand.info
	if err := f.checkOperandName(source, sourceOperand, false); err != nil {
		return err
	}
	if sourceInfo == nil {
		return ErrNotFound
	}

	if sourceInfo.IsDir() {
		return ErrCopyDirectoryUnsupported
	}

	if sourceInfo.Size() > min(f.limits.MaxFileSize, f.limits.MaxWriteSize, f.limits.MaxScanSize) {
		return ErrLimitExceeded
	}

	targetOperand, err := f.inspectEffect(targetPath.String(), true)
	if err != nil {
		return err
	}
	if err := f.checkOperandName(target, targetOperand, true); err != nil {
		return err
	}
	if info := targetOperand.info; info != nil {
		if !overwrite {
			return ErrFileExists
		}
		if info.IsDir() {
			return ErrPathIsDirectory
		}
		if os.SameFile(info, sourceInfo) {
			return ErrSamePath
		}
	}

	if err := f.recheckEffect(sourceOperand, false); err != nil {
		return err
	}
	sourceFile, err := os.Open(sourceOperand.path)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	checkSource := func() error {
		if err := f.checkOpened(sourceFile, sourceOperand, false); err != nil {
			return err
		}
		return admitEffect(admission)
	}
	targetFile, err := f.openDestination(targetOperand, sourceInfo.Mode().Perm(), checkSource)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if err := f.checkDestination(targetFile, targetOperand); err != nil {
		return err
	}
	// Copy at most the admitted size even if an external writer appends
	// to the source after the size check.
	_, err = io.Copy(targetFile, io.LimitReader(sourceFile, sourceInfo.Size()))
	return err
}
