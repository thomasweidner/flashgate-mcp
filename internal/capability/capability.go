// Package capability defines FlashGate's functional authorization vocabulary.
//
// Functional capabilities describe what an effective policy permits. They are
// intentionally independent from profile names and from risk classifications;
// those layers may grant or constrain capabilities but are not capabilities
// themselves.
package capability

import "sort"

// Name is a stable functional capability identifier.
type Name string

const (
	FilesystemRead         Name = "filesystem.read"
	FilesystemWrite        Name = "filesystem.write"
	SearchExecute          Name = "search.execute"
	ProcessObserve         Name = "process.observe"
	ProcessManage          Name = "process.manage"
	ProcessControlExternal Name = "process.control.external"
	CommandExecute         Name = "command.execute"
	SystemRead             Name = "system.read"
)

var known = map[Name]struct{}{
	FilesystemRead: {}, FilesystemWrite: {}, SearchExecute: {},
	ProcessObserve: {}, ProcessManage: {}, ProcessControlExternal: {},
	CommandExecute: {}, SystemRead: {},
}

// IsKnown reports whether name belongs to the closed Version 1.0 functional
// capability vocabulary.
func IsKnown(name Name) bool {
	_, ok := known[name]
	return ok
}

// Set is an immutable set of validated functional capabilities.
type Set struct{ names map[Name]struct{} }

// NewSet constructs a capability set. Unknown names are rejected so profile
// or risk labels cannot accidentally become authorization rights.
func NewSet(names ...Name) (Set, bool) {
	result := Set{names: make(map[Name]struct{}, len(names))}
	for _, name := range names {
		if !IsKnown(name) {
			return Set{}, false
		}
		result.names[name] = struct{}{}
	}
	return result, true
}

// Has reports whether the set contains name. Unknown names always return false.
func (s Set) Has(name Name) bool {
	if !IsKnown(name) {
		return false
	}
	_, ok := s.names[name]
	return ok
}

// Names returns the set in deterministic identifier order.
func (s Set) Names() []Name {
	result := make([]Name, 0, len(s.names))
	for name := range s.names {
		result = append(result, name)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
