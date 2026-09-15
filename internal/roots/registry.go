// Package roots owns the application-level mapping from opaque root IDs to
// root-confined filesystem implementations.
package roots

import (
	"errors"
	"path"
	"sort"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

const DefaultID = "default"

var (
	ErrInvalidEntry           = errors.New("invalid root entry")
	ErrDuplicateID            = errors.New("duplicate root id")
	ErrNoRoots                = errors.New("no roots configured")
	ErrUnknownRoot            = errors.New("unknown root id")
	ErrAccessDenied           = errors.New("root access denied")
	ErrInvalidLimits          = errors.New("invalid root limits")
	ErrInvalidFileTypes       = errors.New("invalid root file types")
	ErrInvalidLinkRules       = errors.New("invalid root link rules")
	ErrLinkPolicyMismatch     = errors.New("root link rules do not match filesystem policy")
	ErrInvalidCapabilities    = errors.New("invalid root capabilities")
	ErrCapabilityDenied       = errors.New("root capability denied")
	ErrWorkingDirectoryDenied = errors.New("process working directory denied")
)

// Capability is a set of functional operations authorized for a root. It is
// deliberately separate from coarse filesystem access so both gates must pass.
type Capability uint8

const (
	FilesystemRead Capability = 1 << iota
	FilesystemWrite
	FilesystemReadWrite = FilesystemRead | FilesystemWrite
)

func capabilitiesFor(access Access) Capability {
	var capabilities Capability
	if access&Read != 0 {
		capabilities |= FilesystemRead
	}
	if access&Write != 0 {
		capabilities |= FilesystemWrite
	}
	return capabilities
}

// SymlinkRule controls classic symbolic-link traversal for a root.
type SymlinkRule uint8

const (
	DenySymlinks SymlinkRule = iota + 1
	FollowInternalSymlinks
)

// ReparsePointRule controls Windows non-symlink reparse points. They remain
// fail-closed because the current resolver cannot safely evaluate them.
type ReparsePointRule uint8

const DenyReparsePoints ReparsePointRule = 1

// LinkRules is the explicit per-root symlink and Windows reparse-point policy.
type LinkRules struct {
	Symlinks      SymlinkRule
	ReparsePoints ReparsePointRule
}

func (r LinkRules) valid() bool {
	return (r.Symlinks == DenySymlinks || r.Symlinks == FollowInternalSymlinks) &&
		r.ReparsePoints == DenyReparsePoints
}

func linkRulesFor(filesystem fs.FileSystem) LinkRules {
	symlinks := DenySymlinks
	if filesystem.PathPolicy().FollowSymlinks {
		symlinks = FollowInternalSymlinks
	}
	return LinkRules{Symlinks: symlinks, ReparsePoints: DenyReparsePoints}
}

// Access is a set of operations permitted for a named root.
type Access uint8

const (
	Read Access = 1 << iota
	Write
	ReadWrite = Read | Write
)

// Limits bounds data handled on behalf of one named root. Limits for features
// that are not implemented yet are retained here so future scan and temporary
// data paths cannot accidentally fall back to an unbounded policy.
type Limits struct {
	MaxFileBytes      int64
	MaxResultBytes    int64
	MaxScanBytes      int64
	MaxTemporaryBytes int64
}

// FileTypes is a portable, extension-based allowlist for file-content access.
// Extensions are canonical lower-case ASCII values including the leading dot.
type FileTypes struct {
	AllowAll   bool
	Extensions []string
}

// AllFileTypes preserves the compatible single-root behavior.
func AllFileTypes() FileTypes { return FileTypes{AllowAll: true} }

func (f FileTypes) valid() bool {
	if f.AllowAll {
		return len(f.Extensions) == 0
	}
	if len(f.Extensions) == 0 {
		return false
	}
	previous := ""
	for _, extension := range f.Extensions {
		if len(extension) < 2 || extension[0] != '.' || extension != strings.ToLower(extension) ||
			strings.ContainsAny(extension, `/\\`) || extension == previous {
			return false
		}
		for _, character := range extension[1:] {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') {
				return false
			}
		}
		previous = extension
	}
	return sort.StringsAreSorted(f.Extensions)
}

// Allows reports whether a relative file path has an allowed extension. Both
// slash forms are normalized before matching so policy does not vary by host OS.
func (f FileTypes) Allows(filePath string) bool {
	if f.AllowAll {
		return true
	}
	extension := strings.ToLower(path.Ext(strings.ReplaceAll(filePath, `\`, "/")))
	index := sort.SearchStrings(f.Extensions, extension)
	return index < len(f.Extensions) && f.Extensions[index] == extension
}

// DefaultLimits preserves the limits used by the compatible single-root path.
func DefaultLimits(maxFileBytes, maxResultBytes int64) Limits {
	return Limits{
		MaxFileBytes:      maxFileBytes,
		MaxResultBytes:    maxResultBytes,
		MaxScanBytes:      maxFileBytes,
		MaxTemporaryBytes: maxFileBytes,
	}
}

func (l Limits) valid() bool {
	return l.MaxFileBytes > 0 && l.MaxResultBytes > 0 &&
		l.MaxScanBytes > 0 && l.MaxTemporaryBytes > 0
}

type Entry struct {
	ID                      string
	FileSystem              fs.FileSystem
	Access                  Access
	Limits                  Limits
	FileTypes               FileTypes
	LinkRules               LinkRules
	Capabilities            Capability
	ProcessWorkingDirectory bool
}

// Root is the resolved, authorized policy and filesystem for one root.
type Root struct {
	FileSystem              fs.FileSystem
	Limits                  Limits
	FileTypes               FileTypes
	LinkRules               LinkRules
	Capabilities            Capability
	ProcessWorkingDirectory bool
}

// Registry is an immutable collection of named, independently confined roots.
type Registry struct {
	entries map[string]Entry
}

func New(entries []Entry) (*Registry, error) {
	if len(entries) == 0 {
		return nil, ErrNoRoots
	}
	registry := &Registry{entries: make(map[string]Entry, len(entries))}
	for _, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" || entry.FileSystem == nil ||
			entry.Access == 0 || entry.Access&^ReadWrite != 0 {
			return nil, ErrInvalidEntry
		}
		if !entry.Limits.valid() {
			return nil, ErrInvalidLimits
		}
		if !entry.FileTypes.valid() {
			return nil, ErrInvalidFileTypes
		}
		if !entry.LinkRules.valid() {
			return nil, ErrInvalidLinkRules
		}
		if entry.LinkRules != linkRulesFor(entry.FileSystem) {
			return nil, ErrLinkPolicyMismatch
		}
		if entry.Capabilities == 0 || entry.Capabilities&^FilesystemReadWrite != 0 ||
			entry.Capabilities&^capabilitiesFor(entry.Access) != 0 {
			return nil, ErrInvalidCapabilities
		}
		if _, exists := registry.entries[entry.ID]; exists {
			return nil, ErrDuplicateID
		}
		entry.FileTypes.Extensions = append([]string(nil), entry.FileTypes.Extensions...)
		registry.entries[entry.ID] = entry
	}
	return registry, nil
}

// Single preserves the current single-root deployment as the default named root.
func Single(filesystem fs.FileSystem) (*Registry, error) {
	return SingleWithPolicy(filesystem, ReadWrite, DefaultLimits(10*1024*1024, 16*1024*1024))
}

// SingleWithAccess preserves the current single-root deployment with an
// explicit policy derived from its effective profile.
func SingleWithAccess(filesystem fs.FileSystem, access Access) (*Registry, error) {
	return SingleWithPolicy(filesystem, access, DefaultLimits(10*1024*1024, 16*1024*1024))
}

// SingleWithPolicy preserves the current single-root deployment with explicit
// access and resource policies derived from configuration.
func SingleWithPolicy(filesystem fs.FileSystem, access Access, limits Limits) (*Registry, error) {
	return New([]Entry{{ID: DefaultID, FileSystem: filesystem, Access: access, Limits: limits, FileTypes: AllFileTypes(), LinkRules: linkRulesFor(filesystem), Capabilities: capabilitiesFor(access)}})
}

// FileSystem returns a root only when all requested access is permitted.
func (r *Registry) FileSystem(id string, required Access) (fs.FileSystem, error) {
	root, err := r.Root(id, required)
	return root.FileSystem, err
}

// Root returns the complete root policy only when both the requested access
// and its corresponding filesystem capabilities are permitted.
func (r *Registry) Root(id string, required Access) (Root, error) {
	return r.RootWithCapability(id, required, capabilitiesFor(required))
}

// RootWithCapability returns the root only when both its coarse access and
// functional capability mapping authorize the requested operation.
func (r *Registry) RootWithCapability(id string, required Access, requiredCapability Capability) (Root, error) {
	entry, ok := r.entries[id]
	if !ok {
		return Root{}, ErrUnknownRoot
	}
	if required == 0 || required&^ReadWrite != 0 || entry.Access&required != required {
		return Root{}, ErrAccessDenied
	}
	if requiredCapability == 0 || requiredCapability&^FilesystemReadWrite != 0 ||
		entry.Capabilities&requiredCapability != requiredCapability {
		return Root{}, ErrCapabilityDenied
	}
	return rootFromEntry(entry), nil
}

// WorkingDirectoryRoot resolves a root only when it explicitly permits use as
// a managed process working directory. This policy is independent of
// filesystem read/write access and capabilities.
func (r *Registry) WorkingDirectoryRoot(id string) (Root, error) {
	entry, ok := r.entries[id]
	if !ok {
		return Root{}, ErrUnknownRoot
	}
	if !entry.ProcessWorkingDirectory {
		return Root{}, ErrWorkingDirectoryDenied
	}
	return rootFromEntry(entry), nil
}

func rootFromEntry(entry Entry) Root {
	fileTypes := entry.FileTypes
	fileTypes.Extensions = append([]string(nil), entry.FileTypes.Extensions...)
	return Root{
		FileSystem:              entry.FileSystem,
		Limits:                  entry.Limits,
		FileTypes:               fileTypes,
		LinkRules:               entry.LinkRules,
		Capabilities:            entry.Capabilities,
		ProcessWorkingDirectory: entry.ProcessWorkingDirectory,
	}
}

func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.entries))
	for id := range r.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
