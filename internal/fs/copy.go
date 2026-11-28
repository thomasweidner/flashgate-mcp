package fs

import (
	"io"
	"os"
)

// Copy copies a file. Directory copy is intentionally unsupported.
func (f *LocalFileSystem) Copy(source string, target string, overwrite bool, admission ...func() error) error {
	_, err := f.CopyMeasured(source, target, overwrite, admission...)
	return err
}

// CopyMeasured returns independent native read and write observations.
func (f *LocalFileSystem) CopyMeasured(source string, target string, overwrite bool, admission ...func() error) (CopyCounts, error) {
	f.effects.Lock()
	defer f.effects.Unlock()
	for _, path := range []string{source, target} {
		if err := f.validateFileOperandSpelling(path); err != nil {
			return CopyCounts{}, err
		}
	}

	sourcePath, err := f.resolveExisting(source)
	if err != nil {
		return CopyCounts{}, err
	}

	targetPath, err := f.guard.ResolveForCreate(target)
	if err != nil {
		return CopyCounts{}, err
	}

	sourceOperand, err := f.inspectEffect(sourcePath.String(), false)
	if err != nil {
		return CopyCounts{}, err
	}
	sourceInfo := sourceOperand.info
	if err := f.checkOperandName(source, sourceOperand, false); err != nil {
		return CopyCounts{}, err
	}
	if sourceInfo == nil {
		return CopyCounts{}, ErrNotFound
	}

	if sourceInfo.IsDir() {
		return CopyCounts{}, ErrCopyDirectoryUnsupported
	}

	if sourceInfo.Size() > min(f.limits.MaxFileSize, f.limits.MaxWriteSize, f.limits.MaxScanSize) {
		return CopyCounts{}, ErrLimitExceeded
	}

	targetOperand, err := f.inspectEffect(targetPath.String(), true)
	if err != nil {
		return CopyCounts{}, err
	}
	if err := f.checkOperandName(target, targetOperand, true); err != nil {
		return CopyCounts{}, err
	}
	if info := targetOperand.info; info != nil {
		if !overwrite {
			return CopyCounts{}, ErrFileExists
		}
		if info.IsDir() {
			return CopyCounts{}, ErrPathIsDirectory
		}
		if os.SameFile(info, sourceInfo) {
			return CopyCounts{}, ErrSamePath
		}
	}

	if err := f.recheckEffect(sourceOperand, false); err != nil {
		return CopyCounts{}, err
	}
	sourceFile, err := f.openNative(sourceOperand, os.O_RDONLY, 0)
	if err != nil {
		return CopyCounts{}, err
	}
	defer sourceFile.Close()
	checkSource := func() error {
		if err := f.checkOpened(sourceFile, sourceOperand, false); err != nil {
			return err
		}
		return admitEffect(admission)
	}
	targetFile, err := f.openDestination(targetOperand, sourceInfo.Mode().Perm(), checkSource,
		func() error { return f.checkOpened(sourceFile, sourceOperand, false) })
	if err != nil {
		return CopyCounts{}, err
	}
	defer targetFile.Close()

	if err := f.checkDestination(targetFile, targetOperand); err != nil {
		return CopyCounts{}, err
	}
	// Copy at most the admitted size even if an external writer appends
	// to the source after the size check.
	return copyMeasured(targetFile, sourceFile, sourceInfo.Size())
}

type CopyCounts struct {
	Read, Written *uint64
}

// copyWriter observes Write calls and prevents ReaderFrom from collapsing
// the source and destination observations into a single transfer count.
type copyWriter struct {
	io.Writer
	attempted bool
}

func (w *copyWriter) Write(data []byte) (int, error) {
	w.attempted = true
	return w.Writer.Write(data)
}

func copyMeasured(target io.Writer, source io.Reader, limit int64) (counts CopyCounts, err error) {
	reader := &io.LimitedReader{R: source, N: limit}
	writer := &copyWriter{Writer: target}
	written, err := io.Copy(writer, reader)
	if limit > 0 {
		counts.Read = new(uint64(limit - reader.N))
	}
	if writer.attempted {
		counts.Written = new(uint64(written))
	}
	return counts, err
}
