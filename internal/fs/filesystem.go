package fs

import (
	"errors"
	"sync"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

var (
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
	ErrLimitExceeded = errors.New("filesystem limit exceeded")

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
	MaxWriteBytes    int64
	MaxListEntries   int
	MaxCopyBytes     int64
	MaxDeleteEntries int
}

// DefaultLimits returns conservative filesystem limits.
func DefaultLimits() Limits {
	return Limits{
		MaxWriteBytes:    10 * 1024 * 1024,
		MaxListEntries:   1000,
		MaxCopyBytes:     10 * 1024 * 1024,
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
}

// FileSystem is the named-root boundary for local filesystem consumers.
type FileSystem interface {
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
	guard      *security.PathGuard
	limits     Limits
	protection *security.RootProtection
	effects    *sync.Mutex
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
func NewLocalFileSystemWithPolicyAndLimits(root string, policy security.Policy, limits Limits) (*LocalFileSystem, error) {
	if err := validateLimits(limits); err != nil {
		return nil, err
	}

	guard, err := security.NewPathGuardWithPolicy(root, policy)
	if err != nil {
		return nil, err
	}

	return newLocalFileSystem(guard, limits), nil
}

// NewLocalFileSystemWithGuard creates a filesystem from a validated PathGuard.
func NewLocalFileSystemWithGuard(guard *security.PathGuard, limits Limits) (*LocalFileSystem, error) {
	if guard == nil {
		return nil, security.ErrEmptyRoot
	}
	if err := validateLimits(limits); err != nil {
		return nil, err
	}
	return newLocalFileSystem(guard, limits), nil
}

// NewProtectedFileSystems constructs all root filesystems in binding order.
// A single lock coordinates effects even when distinct roots overlap.
func NewProtectedFileSystems(bindings []*security.RootProtection, limits Limits) ([]*LocalFileSystem, error) {
	if err := validateLimits(limits); err != nil {
		return nil, err
	}
	effects := new(sync.Mutex)
	filesystems := make([]*LocalFileSystem, 0, len(bindings))
	for _, binding := range bindings {
		if binding == nil || binding.Guard() == nil {
			return nil, security.ErrProtectedPath
		}
		filesystems = append(filesystems, &LocalFileSystem{
			guard: binding.Guard(), limits: limits, protection: binding, effects: effects,
		})
	}
	return filesystems, nil
}

func newLocalFileSystem(guard *security.PathGuard, limits Limits) *LocalFileSystem {
	return &LocalFileSystem{
		guard:   guard,
		limits:  limits,
		effects: new(sync.Mutex),
	}
}

func validateLimits(limits Limits) error {
	if limits.MaxWriteBytes <= 0 ||
		limits.MaxListEntries <= 0 ||
		limits.MaxCopyBytes <= 0 ||
		limits.MaxDeleteEntries <= 0 {
		return ErrLimitExceeded
	}

	return nil
}
