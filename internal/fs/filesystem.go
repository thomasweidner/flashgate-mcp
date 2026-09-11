package fs

import (
	"errors"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

var (
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

// FileSystem defines filesystem operations used by MCP tools.
type FileSystem interface {
	List(path string) ([]Entry, error)
	Read(path string, maxBytes int64) ([]byte, error)
	Stat(path string) (Metadata, error)
	ValidatePath(path string, mustExist bool) error
	Write(path string, content []byte, overwrite bool) error
	Mkdir(path string) (bool, error)
	Delete(path string, recursive bool) error
	Move(source string, target string, overwrite bool) error
	Copy(source string, target string, overwrite bool) error
}

// ValidatePath applies the configured path policy without changing filesystem state.
func (f *LocalFileSystem) ValidatePath(path string, mustExist bool) error {
	if mustExist {
		_, err := f.guard.ResolveExisting(path)
		return err
	}
	_, err := f.guard.ResolveForCreate(path)
	return err
}

// LocalFileSystem implements FileSystem using the local operating system.
type LocalFileSystem struct {
	guard  *security.PathGuard
	limits Limits
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

	return &LocalFileSystem{
		guard:  guard,
		limits: limits,
	}, nil
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
