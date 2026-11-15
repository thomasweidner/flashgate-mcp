// Package capability models functional rights for the transport-neutral core.
package capability

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ID is the exact, case-sensitive name of a functional right.
// Only IDs in a validated Catalog can participate in a decision.
type ID string

const (
	FilesystemRead         ID = "filesystem.read"
	FilesystemWrite        ID = "filesystem.write"
	SearchExecute          ID = "search.execute"
	ProcessObserve         ID = "process.observe"
	ProcessManage          ID = "process.manage"
	ProcessControlExternal ID = "process.control.external"
	CommandExecute         ID = "command.execute"
	SystemRead             ID = "system.read"
)

// Valid reports whether id has two or more dot-separated segments, each
// starting with an ASCII lowercase letter and containing only ASCII lowercase
// letters or digits. An ID is at most 128 bytes.
func (id ID) Valid() bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	atStart, hasNamespace := true, false
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z':
			atStart = false
		case atStart:
			return false
		case c == '.':
			atStart, hasNamespace = true, true
		case c < '0' || c > '9':
			return false
		}
	}
	return hasNamespace && !atStart
}

// Definition names and describes one functional right. The description is
// review metadata. It has no effect on evaluation.
type Definition struct {
	ID          ID
	Description string
}

// BaseDefinitions returns an independent copy of the FlashGate basis.
func BaseDefinitions() []Definition {
	return []Definition{
		{FilesystemRead, "Inspect filesystem metadata and read filesystem content."},
		{FilesystemWrite, "Create, modify, move or remove filesystem data."},
		{SearchExecute, "Execute a bounded search."},
		{ProcessObserve, "Inspect permitted host process information."},
		{ProcessManage, "Operate on server-managed processes."},
		{ProcessControlExternal, "Control processes outside the server-managed lifecycle."},
		{CommandExecute, "Execute a server-approved typed command."},
		{SystemRead, "Read explicitly permitted system information."},
	}
}

var (
	ErrInvalidID             = errors.New("invalid capability ID")
	ErrInvalidDefinition     = errors.New("invalid capability definition")
	ErrDuplicateDefinition   = errors.New("duplicate capability definition")
	ErrConflictingDefinition = errors.New("conflicting capability definition")
	ErrInvalidCatalog        = errors.New("invalid capability catalog")
	ErrUnknownID             = errors.New("unknown capability ID")
	ErrDuplicateID           = errors.New("duplicate capability ID")
	ErrEmptyRequirement      = errors.New("empty capability requirement")
	ErrIncompatibleSets      = errors.New("incompatible capability sets")
)

// Catalog holds one immutable snapshot of reviewed definitions. Its zero value
// is invalid. Copying a Catalog preserves the snapshot identity.
type Catalog struct {
	snapshot *catalogSnapshot
}

type catalogSnapshot struct {
	definitions map[ID]Definition
}

// NewCatalog validates every definition before creating a snapshot. Duplicate
// IDs are errors even when their descriptions are identical.
func NewCatalog(definitions ...Definition) (Catalog, error) {
	if len(definitions) == 0 {
		return Catalog{}, ErrInvalidCatalog
	}
	ordered := slices.Clone(definitions)
	slices.SortFunc(ordered, func(a, b Definition) int {
		return cmp.Compare(a.ID, b.ID)
	})
	byID := make(map[ID]Definition, len(ordered))
	for i := 0; i < len(ordered); {
		first := ordered[i]
		if !first.ID.Valid() {
			return Catalog{}, ErrInvalidID
		}
		next, conflict := i, false
		for next < len(ordered) && ordered[next].ID == first.ID {
			description := ordered[next].Description
			if !validDescription(description) {
				return Catalog{}, ErrInvalidDefinition
			}
			conflict = conflict || description != first.Description
			next++
		}
		if conflict {
			return Catalog{}, ErrConflictingDefinition
		}
		if next-i > 1 {
			return Catalog{}, ErrDuplicateDefinition
		}
		byID[first.ID] = first
		i = next
	}
	return Catalog{snapshot: &catalogSnapshot{definitions: byID}}, nil
}

func validDescription(description string) bool {
	return len(description) > 0 && len(description) <= 256 &&
		utf8.ValidString(description) && strings.TrimSpace(description) == description &&
		strings.IndexFunc(description, unicode.IsControl) == -1
}

// Lookup returns the definition for an exact ID in this snapshot.
func (c Catalog) Lookup(id ID) (Definition, bool) {
	if c.snapshot == nil {
		return Definition{}, false
	}
	definition, ok := c.snapshot.definitions[id]
	return definition, ok
}
