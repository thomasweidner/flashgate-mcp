package fs

import (
	"io"
	"os"
)

// Copy copies a file. Directory copy is intentionally unsupported.
func (f *LocalFileSystem) Copy(source string, target string, overwrite bool) error {
	f.effects.Lock()
	defer f.effects.Unlock()

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
	if sourceInfo == nil {
		return ErrNotFound
	}

	if sourceInfo.IsDir() {
		return ErrCopyDirectoryUnsupported
	}

	if sourceInfo.Size() > f.limits.MaxCopyBytes {
		return ErrLimitExceeded
	}

	targetOperand, err := f.inspectEffect(targetPath.String(), true)
	if err != nil {
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
	checkSource := func() error { return f.checkOpened(sourceFile, sourceOperand, false) }
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
