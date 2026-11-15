package capability

import "slices"

// Set is an immutable collection of granted IDs from one Catalog snapshot.
// Its zero value is invalid. Construct an empty grant with Catalog.NewSet.
type Set struct {
	snapshot *catalogSnapshot
	ids      []ID
}

// NewSet validates explicit grants and stores them in ascending ID order.
func (c Catalog) NewSet(ids ...ID) (Set, error) {
	if c.snapshot == nil {
		return Set{}, ErrInvalidCatalog
	}
	ordered := slices.Clone(ids)
	slices.Sort(ordered)
	for i, id := range ordered {
		if !id.Valid() {
			return Set{}, ErrInvalidID
		}
		if _, known := c.Lookup(id); !known {
			return Set{}, ErrUnknownID
		}
		if i > 0 && id == ordered[i-1] {
			return Set{}, ErrDuplicateID
		}
	}
	return Set{snapshot: c.snapshot, ids: ordered}, nil
}

// IDs returns an independent copy in ascending ID order.
func (s Set) IDs() []ID {
	return slices.Clone(s.ids)
}

// Contains reports whether an exact ID is explicitly granted.
func (s Set) Contains(id ID) bool {
	_, found := slices.BinarySearch(s.ids, id)
	return found
}

// Equal reports whether both sets belong to the same snapshot and grant the
// same rights.
func (s Set) Equal(other Set) bool {
	return s.compatible(other) && slices.Equal(s.ids, other.ids)
}

// Union returns the explicit grants in either set.
func (s Set) Union(other Set) (Set, error) {
	if !s.compatible(other) {
		return Set{}, ErrIncompatibleSets
	}
	ids := slices.Concat(s.ids, other.ids)
	slices.Sort(ids)
	return Set{snapshot: s.snapshot, ids: slices.Compact(ids)}, nil
}

// Intersect returns only grants present in both sets.
func (s Set) Intersect(other Set) (Set, error) {
	if !s.compatible(other) {
		return Set{}, ErrIncompatibleSets
	}
	var ids []ID
	for _, id := range s.ids {
		if other.Contains(id) {
			ids = append(ids, id)
		}
	}
	return Set{snapshot: s.snapshot, ids: ids}, nil
}

func (s Set) compatible(other Set) bool {
	return s.snapshot != nil && s.snapshot == other.snapshot
}

// Requirement declares all rights required for one operation or variant.
// Its zero value represents a missing declaration and always denies.
type Requirement struct {
	required Set
}

// NewRequirement validates a nonempty list of distinct required rights.
func (c Catalog) NewRequirement(ids ...ID) (Requirement, error) {
	if len(ids) == 0 {
		return Requirement{}, ErrEmptyRequirement
	}
	required, err := c.NewSet(ids...)
	if err != nil {
		return Requirement{}, err
	}
	return Requirement{required: required}, nil
}

// SatisfiedBy reports functional satisfaction only. The caller must apply
// the other authorization and availability checks before dispatch.
func (r Requirement) SatisfiedBy(grants Set) bool {
	if len(r.required.ids) == 0 || !r.required.compatible(grants) {
		return false
	}
	for _, id := range r.required.ids {
		if !grants.Contains(id) {
			return false
		}
	}
	return true
}
