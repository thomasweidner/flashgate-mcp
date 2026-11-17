package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Move moves or renames a file or directory on the same filesystem volume.
func (f *LocalFileSystem) Move(source string, target string, overwrite bool, admission ...func() error) error {
	f.effects.Lock()
	defer f.effects.Unlock()

	sourcePath, err := f.guard.ResolveExisting(source)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}

	targetPath, err := f.guard.ResolveForCreate(target)
	if err != nil {
		return err
	}

	if pathsEquivalent(sourcePath.String(), targetPath.String()) {
		return ErrSamePath
	}

	sourceOperand, err := f.inspectEffect(sourcePath.String(), true)
	if err != nil {
		return err
	}
	sourceInfo := sourceOperand.info
	if sourceInfo == nil {
		return ErrNotFound
	}

	targetOperand, err := f.inspectEffect(targetPath.String(), true)
	if err != nil {
		return err
	}
	targetInfo := targetOperand.info
	targetExists := targetInfo != nil
	if targetExists && os.SameFile(sourceInfo, targetInfo) {
		return ErrSamePath
	}

	effectiveSource, err := filepath.EvalSymlinks(sourcePath.String())
	if err != nil {
		return err
	}
	effectiveTarget, err := effectivePathForComparison(targetPath.String())
	if err != nil {
		return err
	}
	if pathsEquivalent(effectiveSource, effectiveTarget) {
		return ErrSamePath
	}

	if sourceInfo.IsDir() && isStrictDescendant(effectiveSource, effectiveTarget) {
		return ErrMoveIntoSelf
	}

	sameVolume, err := pathsOnSameVolume(sourcePath.String(), targetPath.String())
	if err != nil {
		return err
	}
	if !sameVolume {
		return renameOnSameVolume(sourcePath.String(), targetPath.String(), false, os.Rename)
	}

	if targetExists {
		if !overwrite {
			return ErrFileExists
		}
		if sourceInfo.IsDir() || targetInfo.IsDir() {
			return ErrMoveTypeMismatch
		}
	}

	if err := revalidateMoveState(sourcePath.String(), targetPath.String(), sourceInfo, targetInfo, targetExists); err != nil {
		return err
	}
	if sourceInfo.IsDir() {
		if _, err := f.inspectTree(sourceOperand, false); err != nil {
			return err
		}
	}
	if err := f.recheckEffect(targetOperand, true); err != nil {
		return err
	}
	if err := f.recheckEffect(sourceOperand, true); err != nil {
		return err
	}

	if err := admitEffect(admission); err != nil {
		return err
	}
	return renameOnSameVolume(sourcePath.String(), targetPath.String(), sameVolume, os.Rename)
}

func effectivePathForComparison(path string) (string, error) {
	current := filepath.Clean(path)
	missingSuffix := make([]string, 0)

	for {
		effective, err := filepath.EvalSymlinks(current)
		if err == nil {
			for index := len(missingSuffix) - 1; index >= 0; index-- {
				effective = filepath.Join(effective, missingSuffix[index])
			}
			return filepath.Clean(effective), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		missingSuffix = append(missingSuffix, filepath.Base(current))
		current = parent
	}
}

func revalidateMoveState(source string, target string, expectedSource os.FileInfo, expectedTarget os.FileInfo, targetExists bool) error {
	currentSource, err := os.Stat(source)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	if expectedSource.IsDir() != currentSource.IsDir() || !os.SameFile(expectedSource, currentSource) {
		return ErrMovePathChanged
	}

	currentTarget, err := os.Stat(target)
	if targetExists {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return ErrMovePathChanged
			}
			return err
		}
		if expectedTarget.IsDir() != currentTarget.IsDir() || !os.SameFile(expectedTarget, currentTarget) {
			return ErrMovePathChanged
		}
		if os.SameFile(currentSource, currentTarget) {
			return ErrSamePath
		}
		return nil
	}

	if err == nil {
		return ErrMovePathChanged
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func renameOnSameVolume(source string, target string, sameVolume bool, rename func(string, string) error) error {
	if !sameVolume {
		return ErrCrossVolumeMoveUnsupported
	}
	if err := rename(source, target); err != nil {
		if isCrossVolumeError(err) {
			return ErrCrossVolumeMoveUnsupported
		}
		return err
	}
	return nil
}

func isStrictDescendant(parent string, candidate string) bool {
	relative, err := filepath.Rel(parent, candidate)
	if err != nil || relative == "." {
		return false
	}

	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
