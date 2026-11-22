package fs

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

var (
	// ErrFileTypeDenied also uses the existing permission-denial adapter path.
	ErrFileTypeDenied = fmt.Errorf("file type denied: %w", os.ErrPermission)
	// ErrUnknownRoot is returned when an exact root ID is not registered.
	ErrUnknownRoot = errors.New("unknown root id")

	// ErrRootAccessDenied indicates a missing right on the selected root.
	ErrRootAccessDenied = errors.New("root access denied")

	// ErrCrossRootOperation is returned when an operation requires one root.
	ErrCrossRootOperation = errors.New("cross-root filesystem operation is not supported")

	// ErrFileTooLarge is returned when a file exceeds the configured read limit.
	ErrFileTooLarge = errors.New("file exceeds maximum allowed size")

	// ErrPathIsDirectory is returned when a file operation receives a directory path.
	ErrPathIsDirectory = errors.New("path is a directory")

	// ErrPathIsNotDirectory is returned when a directory operation receives a non-directory path.
	ErrPathIsNotDirectory = errors.New("path is not a directory")

	// ErrFileExists is returned when writing a file that already exists without overwrite permission.
	ErrFileExists = errors.New("file already exists")

	// ErrDirectoryNotEmpty is returned when deleting a non-empty directory without recursive deletion.
	ErrDirectoryNotEmpty = errors.New("directory is not empty")

	// ErrCopyDirectoryUnsupported is returned when attempting to copy a directory.
	ErrCopyDirectoryUnsupported = errors.New("copying directories is not supported")

	// ErrLimitExceeded is returned when an operation exceeds a configured limit.
	ErrLimitExceeded = resource.ErrLimitExceeded

	// ErrNotFound is returned when an expected filesystem path does not exist.
	ErrNotFound = errors.New("filesystem path not found")

	// ErrSamePath is returned when a move source and target identify the same file.
	ErrSamePath = errors.New("source and target identify the same path")

	// ErrMoveIntoSelf is returned when a directory would be moved into its own subtree.
	ErrMoveIntoSelf = errors.New("directory cannot be moved into its own subtree")

	// ErrCrossVolumeMoveUnsupported is returned when a move would cross filesystem volumes.
	ErrCrossVolumeMoveUnsupported = errors.New("cross-volume move is not supported")

	// ErrMoveTypeMismatch is returned for unsupported source and target type combinations.
	ErrMoveTypeMismatch = errors.New("unsupported move path type combination")

	// ErrMovePathChanged is returned when a move path changes during validation.
	ErrMovePathChanged = errors.New("move path changed during validation")
)

// Limits contains filesystem operation limits.
type Limits struct {
	MaxFileSize      int64
	MaxWriteSize     int64
	MaxScanSize      int64
	MaxResultSize    int64
	MaxTempSize      int64
	MaxListEntries   int
	MaxDeleteEntries int
}

// DefaultLimits returns conservative filesystem limits.
func DefaultLimits() Limits {
	return Limits{
		MaxFileSize:      10 * 1024 * 1024,
		MaxWriteSize:     10 * 1024 * 1024,
		MaxScanSize:      10 * 1024 * 1024,
		MaxResultSize:    16 * 1024 * 1024,
		MaxTempSize:      10 * 1024 * 1024,
		MaxListEntries:   1000,
		MaxDeleteEntries: 1000,
	}
}

// Entry represents a filesystem directory entry.
type Entry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// Metadata represents filesystem metadata.
type Metadata struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// Operand addresses a local filesystem resource by exact ID and relative path.
// The registry binds the ID and PathGuard validates the path at dispatch.
type Operand struct {
	RootID config.RootID
	Path   string
	// AdmitEffect is an optional caller-owned result admission at the existing
	// pre-effect boundary. It never replaces domain authorization or limits.
	AdmitEffect func() error
}

func admitEffect(admission []func() error) error {
	if len(admission) > 1 {
		return ErrLimitExceeded
	}
	if len(admission) == 1 && admission[0] != nil {
		return admission[0]()
	}
	return nil
}

// FileSystem is the named-root boundary for local filesystem consumers.
// Implementations invoke any AdmitEffect hook after domain validation and
// before the first mutation. Hooks only admit a caller's result; they grant
// no access and may not perform I/O or reenter the filesystem. Backend owners
// must prove this boundary before activation.
type FileSystem interface {
	LimitsForRoot(id config.RootID) (resource.Limits, error)
	List(path Operand) ([]Entry, error)
	Read(path Operand, maxBytes int64) ([]byte, error)
	Stat(path Operand) (Metadata, error)
	Write(path Operand, content []byte, overwrite bool) error
	Mkdir(path Operand) (bool, error)
	Delete(path Operand, recursive bool) error
	Move(source, target Operand, overwrite bool) error
	Copy(source, target Operand, overwrite bool) error
}

// LocalFileSystem is the native backend bound to one validated root.
type LocalFileSystem struct {
	fileTypes  config.FileTypePolicy
	guard      *security.PathGuard
	limits     Limits
	protection *security.RootProtection
	exclusions exclusionRoots
	effects    *sync.Mutex
	// Tests in this package inject deterministic namespace drift at the native
	// boundary. Production constructors leave this unset; it grants no rights.
	beforeNative func(string)
}

// NewLocalFileSystem creates a new local filesystem.
func NewLocalFileSystem(root string) (*LocalFileSystem, error) {
	return NewLocalFileSystemWithPolicy(root, security.DefaultPolicy())
}

// NewLocalFileSystemWithPolicy creates a new local filesystem with an explicit policy.
func NewLocalFileSystemWithPolicy(root string, policy security.Policy) (*LocalFileSystem, error) {
	return NewLocalFileSystemWithPolicyAndLimits(root, policy, DefaultLimits())
}

// NewLocalFileSystemWithPolicyAndLimits creates a new local filesystem with an explicit policy and limits.
func NewLocalFileSystemWithPolicyAndLimits(root string, policy security.Policy, limits Limits, fileTypes ...config.FileTypePolicy) (*LocalFileSystem, error) {
	if len(fileTypes) > 1 {
		return nil, ErrFileTypeDenied
	}
	if err := validateLimits(limits); err != nil {
		return nil, err
	}

	guard, err := security.NewPathGuardWithPolicy(root, policy)
	if err != nil {
		return nil, err
	}

	return newLocalFileSystem(guard, limits, fileTypes...)
}

// NewLocalFileSystemWithGuard creates a filesystem from a validated PathGuard.
func NewLocalFileSystemWithGuard(guard *security.PathGuard, limits Limits, fileTypes ...config.FileTypePolicy) (*LocalFileSystem, error) {
	if len(fileTypes) > 1 {
		return nil, ErrFileTypeDenied
	}
	if guard == nil {
		return nil, security.ErrEmptyRoot
	}
	if err := validateLimits(limits); err != nil {
		return nil, err
	}
	return newLocalFileSystem(guard, limits, fileTypes...)
}

// NewProtectedFileSystems constructs all root filesystems in binding order.
// A single lock coordinates effects even when distinct roots overlap.
func NewProtectedFileSystems(bindings []*security.RootProtection, limits Limits) ([]*LocalFileSystem, error) {
	if err := validateLimits(limits); err != nil {
		return nil, err
	}
	values := make([]Limits, len(bindings))
	for i := range values {
		values[i] = limits
	}
	return NewProtectedRootFileSystems(bindings, values)
}

// NewProtectedRootFileSystems binds all immutable limits before publication.
// Aliased and overlapping roots retain their own caps and share effect locking.
func NewProtectedRootFileSystems(bindings []*security.RootProtection, limits []Limits, fileTypes ...config.FileTypePolicy) ([]*LocalFileSystem, error) {
	return NewProtectedRootFileSystemsWithExclusions(bindings, limits, nil, fileTypes...)
}

// NewProtectedRootFileSystemsWithExclusions binds exclusions for the entire
// root set so selecting a different backend cannot bypass an applicable rule.
func NewProtectedRootFileSystemsWithExclusions(bindings []*security.RootProtection, limits []Limits, rules []config.PathExclusions, fileTypes ...config.FileTypePolicy) ([]*LocalFileSystem, error) {
	if len(fileTypes) != 0 && len(fileTypes) != len(bindings) {
		return nil, ErrFileTypeDenied
	}
	if len(bindings) != len(limits) {
		return nil, ErrLimitExceeded
	}
	for _, l := range limits {
		if err := validateLimits(l); err != nil {
			return nil, err
		}
	}
	effects := new(sync.Mutex)
	filesystems := make([]*LocalFileSystem, 0, len(bindings))
	for i, binding := range bindings {
		if binding == nil || binding.Guard() == nil {
			return nil, security.ErrProtectedPath
		}
		filesystems = append(filesystems, &LocalFileSystem{
			guard: binding.Guard(), limits: limits[i], protection: binding, effects: effects,
		})
		if len(fileTypes) != 0 {
			filesystems[i].fileTypes = fileTypes[i]
		}
		if err := filesystems[i].validateNativeRoot(); err != nil {
			return nil, err
		}
	}
	if err := bindExclusionRoots(filesystems, rules); err != nil {
		return nil, err
	}
	return filesystems, nil
}

func newLocalFileSystem(guard *security.PathGuard, limits Limits, fileTypes ...config.FileTypePolicy) (*LocalFileSystem, error) {
	f := &LocalFileSystem{
		guard:   guard,
		limits:  limits,
		effects: new(sync.Mutex),
	}
	if len(fileTypes) == 1 {
		f.fileTypes = fileTypes[0]
	}
	if err := f.validateNativeRoot(); err != nil {
		return nil, err
	}
	return f, nil
}

func validateLimits(limits Limits) error {
	if limits.MaxWriteSize <= 0 ||
		limits.MaxListEntries <= 0 ||
		limits.ResourceLimits().Validate() != nil ||
		limits.MaxDeleteEntries <= 0 {
		return ErrLimitExceeded
	}

	return nil
}

func (l Limits) ResourceLimits() resource.Limits {
	return resource.Limits{File: l.MaxFileSize, Write: l.MaxWriteSize, Scan: l.MaxScanSize, Result: l.MaxResultSize, Temp: l.MaxTempSize}
}
