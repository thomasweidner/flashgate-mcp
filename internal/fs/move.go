package fs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/resource"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

// Move moves or renames a file or directory on the same filesystem volume.
func (f *LocalFileSystem) Move(source string, target string, overwrite bool, admission ...func() error) error {
	f.effects.Lock()
	defer f.effects.Unlock()
	if err := f.validateFileOperandSpelling(source); err != nil {
		return err
	}
	// The source determines whether a missing target is a structural directory.
	// Keep native spelling safety before resolution; apply the portable file
	// policy below after classifying both operands.
	if err := validateNativeOperandSpelling(target); err != nil {
		return err
	}

	sourcePath, err := f.guard.ResolveExisting(source)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}

	sourceOperand, err := f.inspectEffect(sourcePath.String(), true)
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
	if !sourceInfo.IsDir() {
		if err := f.validateFileOperandSpelling(target); err != nil {
			return err
		}
	}
	targetPath, err := f.guard.ResolveForCreate(target)
	if err != nil {
		return err
	}
	if pathsEquivalent(sourcePath.String(), targetPath.String()) {
		return ErrSamePath
	}

	targetOperand, err := f.inspectEffect(targetPath.String(), true)
	if err != nil {
		return err
	}
	targetInfo := targetOperand.info
	if err := f.checkOperandName(target, targetOperand, !sourceInfo.IsDir()); err != nil {
		return err
	}
	if !sourceInfo.IsDir() {
		if err := f.checkFileType(targetOperand, true); err != nil {
			return err
		}
	}
	if err := f.checkLinkProjection(sourceOperand, targetOperand); err != nil {
		return err
	}
	targetExists := targetInfo != nil
	if targetExists && os.SameFile(sourceInfo, targetInfo) {
		return ErrSamePath
	}

	effectiveSource, err := effectivePathForComparison(sourcePath.String())
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
	var checkTree func() error
	if sourceInfo.IsDir() && !sourceOperand.leafLink {
		meter, err := resource.NewMeter(f.limits.ResourceLimits())
		if err != nil {
			return err
		}
		checkTree = func() error {
			_, err := f.inspectTreeWithMeter(sourceOperand, false, meter, targetOperand.path)
			return err
		}
		if err := checkTree(); err != nil {
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
	return f.renameNative(sourceOperand, targetOperand, checkTree)
}

func effectivePathForComparison(path string) (string, error) {
	return security.EffectivePath(path)
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
