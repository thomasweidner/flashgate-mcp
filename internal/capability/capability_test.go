package capability_test

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
)

func TestID(t *testing.T) {
	for _, id := range []capability.ID{"a.b", "a1.b2", "processes.control", "example.a.b", capability.ID("a." + strings.Repeat("b", 126))} {
		if !id.Valid() {
			t.Errorf("valid ID %q rejected", id)
		}
	}
	for _, id := range []capability.ID{
		"", "admin", ".a", "a.", "a..b", "a.1b", "A.b", "a.B", "a.*", "a.b*",
		"a.b ", " a.b", "a_b.c", "a-b.c", "a/b", "a.ö", "a.b\x00",
		capability.ID("a." + strings.Repeat("b", 127)),
	} {
		if id.Valid() {
			t.Errorf("invalid ID %q accepted", id)
		}
	}
}

func TestCatalog(t *testing.T) {
	definitions := capability.BaseDefinitions()
	want := []capability.ID{
		"files.read", "files.write", "search.run", "processes.read",
		"processes.manage", "processes.control", "commands.run", "system.read",
	}
	if len(definitions) != len(want) {
		t.Fatalf("got %d definitions, want %d", len(definitions), len(want))
	}
	catalog := mustCatalog(t, definitions...)
	original := definitions[0]
	definitions[0] = capability.Definition{}
	if fresh := capability.BaseDefinitions(); fresh[0] != original {
		t.Fatal("base definitions share mutable storage")
	}
	for _, id := range want {
		got, ok := catalog.Lookup(id)
		if !ok || got.ID != id || got.Description == "" {
			t.Errorf("missing definition for %s: %+v", id, got)
		}
	}
	got, _ := catalog.Lookup(original.ID)
	if got != original {
		t.Fatal("catalog did not preserve the validated definition")
	}
	if _, ok := catalog.Lookup("files"); ok {
		t.Fatal("namespace resolved as a right")
	}
	if _, ok := (capability.Catalog{}).Lookup(original.ID); ok {
		t.Fatal("zero catalog resolved a right")
	}
}

func TestCatalogRejectsInvalidDefinitions(t *testing.T) {
	read := capability.Definition{ID: capability.FilesRead, Description: "Read files.", MinimumRisk: capability.RiskRead}
	conflict := capability.Definition{ID: read.ID, Description: "Something else.", MinimumRisk: capability.RiskRead}
	cases := []struct {
		name        string
		definitions []capability.Definition
		want        error
	}{
		{"empty", nil, capability.ErrInvalidCatalog},
		{"invalid ID", []capability.Definition{read, {ID: "filesystem.*", Description: "Wildcard.", MinimumRisk: capability.RiskRead}}, capability.ErrInvalidID},
		{"duplicate", []capability.Definition{read, read}, capability.ErrDuplicateDefinition},
		{"conflict takes precedence", []capability.Definition{read, read, conflict}, capability.ErrConflictingDefinition},
	}
	for _, description := range []string{"", " ", " leading", "trailing ", "line\nbreak", "bad\xff", strings.Repeat("a", 257)} {
		cases = append(cases, struct {
			name        string
			definitions []capability.Definition
			want        error
		}{"description " + description, []capability.Definition{read, {ID: read.ID, Description: description, MinimumRisk: capability.RiskRead}}, capability.ErrInvalidDefinition})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for range 2 {
				catalog, err := capability.NewCatalog(tc.definitions...)
				if !errors.Is(err, tc.want) {
					t.Fatalf("got %v, want %v", err, tc.want)
				}
				if _, ok := catalog.Lookup(read.ID); ok {
					t.Fatal("invalid input returned a partially valid catalog")
				}
				slices.Reverse(tc.definitions)
			}
		})
	}
}

func TestSetOperations(t *testing.T) {
	c := mustCatalog(t, capability.BaseDefinitions()...)
	read, write, search := capability.FilesRead, capability.FilesWrite, capability.SearchRun
	input := []capability.ID{write, read}
	granted := mustSet(t, c, input...)
	input[1] = search
	exported := granted.IDs()
	exported[0] = search
	if !slices.Equal(granted.IDs(), []capability.ID{read, write}) || !granted.Equal(mustSet(t, c, read, write)) {
		t.Fatal("grants depend on input order or mutable caller slices")
	}
	empty := mustSet(t, c)
	other := mustSet(t, c, search, write)
	union, err := granted.Union(other)
	if err != nil || !slices.Equal(union.IDs(), []capability.ID{read, write, search}) {
		t.Fatalf("union = %v, %v", union.IDs(), err)
	}
	intersection, err := granted.Intersect(other)
	if err != nil || !intersection.Equal(mustSet(t, c, write)) {
		t.Fatalf("intersection = %v, %v", intersection.IDs(), err)
	}
	disjoint, err := granted.Intersect(empty)
	if err != nil || !disjoint.Equal(empty) || empty.Equal(granted) {
		t.Fatalf("empty intersection = %v, %v", disjoint.IDs(), err)
	}

	unrelated := mustCatalog(t, capability.BaseDefinitions()...)
	for _, invalid := range []capability.Set{{}, mustSet(t, unrelated, read, write)} {
		if granted.Equal(invalid) {
			t.Fatal("compared sets from unrelated snapshots")
		}
		if got, err := granted.Union(invalid); !errors.Is(err, capability.ErrIncompatibleSets) || got.Contains(read) {
			t.Fatalf("invalid union: %v, %v", got.IDs(), err)
		}
		if got, err := granted.Intersect(invalid); !errors.Is(err, capability.ErrIncompatibleSets) || got.Contains(read) {
			t.Fatalf("invalid intersection: %v, %v", got.IDs(), err)
		}
	}
}

func TestConstructorsRejectUnusableRights(t *testing.T) {
	c := mustCatalog(t, capability.BaseDefinitions()...)
	read := capability.FilesRead
	for _, tc := range []struct {
		ids  []capability.ID
		want error
	}{
		{[]capability.ID{read, "filesystem.*"}, capability.ErrInvalidID},
		{[]capability.ID{read, "filesystem.share"}, capability.ErrUnknownID},
		{[]capability.ID{read, read}, capability.ErrDuplicateID},
	} {
		if set, err := c.NewSet(tc.ids...); !errors.Is(err, tc.want) || set.Contains(read) {
			t.Errorf("invalid set %v = %v, %v", tc.ids, set.IDs(), err)
		}
		if req, err := c.NewRequirement(tc.ids...); !errors.Is(err, tc.want) || req.SatisfiedBy(mustSet(t, c, read)) {
			t.Errorf("invalid requirement %v: %v", tc.ids, err)
		}
	}
	var zero capability.Catalog
	if _, err := zero.NewSet(read); !errors.Is(err, capability.ErrInvalidCatalog) {
		t.Errorf("zero catalog set: %v", err)
	}
	if _, err := zero.NewRequirement(read); !errors.Is(err, capability.ErrInvalidCatalog) {
		t.Errorf("zero catalog requirement: %v", err)
	}
	if req, err := c.NewRequirement(); !errors.Is(err, capability.ErrEmptyRequirement) || req.SatisfiedBy(mustSet(t, c)) {
		t.Errorf("empty requirement: %v", err)
	}
}

func TestDeclarativeExtension(t *testing.T) {
	base := mustCatalog(t, capability.BaseDefinitions()...)
	const future capability.ID = "example.inspect"
	const privilegedLooking capability.ID = "example.inspect.admin"
	definitions := append(capability.BaseDefinitions(),
		capability.Definition{ID: future, Description: "Inspect a synthetic fixture.", MinimumRisk: capability.RiskRead},
		capability.Definition{ID: privilegedLooking, Description: "Inspect synthetic administrative data.", MinimumRisk: capability.RiskRead},
	)
	extended := mustCatalog(t, definitions...)
	if _, ok := extended.Lookup(future); !ok {
		t.Fatal("extension not available through normal lookup")
	}
	if _, err := base.NewRequirement(future); !errors.Is(err, capability.ErrUnknownID) {
		t.Fatalf("base accepted an unknown right: %v", err)
	}
	read := capability.FilesRead
	input := []capability.ID{future, read}
	requirement, err := extended.NewRequirement(input...)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = privilegedLooking
	baseRequirement, err := extended.NewRequirement(read)
	if err != nil {
		t.Fatal(err)
	}
	if !baseRequirement.SatisfiedBy(mustSet(t, extended, read)) {
		t.Fatal("extension changed the existing requirement")
	}
	for _, tc := range []struct {
		name   string
		grants capability.Set
		want   bool
	}{
		{"all", mustSet(t, extended, read, future), true},
		{"all plus extra", mustSet(t, extended, privilegedLooking, future, read), true},
		{"missing extension", mustSet(t, extended, read), false},
		{"missing base right", mustSet(t, extended, future), false},
		{"higher-sounding ID", mustSet(t, extended, privilegedLooking, read), false},
		{"empty grants", mustSet(t, extended), false},
		{"uninitialized grants", capability.Set{}, false},
		{"old snapshot", mustSet(t, base, read), false},
		{"independent snapshot", mustSet(t, mustCatalog(t, definitions...), read, future), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, available := range []bool{false, true} {
				// Dispatch would additionally require adapter availability.
				// The functional result is computed from the same grants.
				functional := requirement.SatisfiedBy(tc.grants)
				dispatch := functional && available
				if functional != tc.want || dispatch != (tc.want && available) {
					t.Errorf("available=%v: functional=%v, dispatch=%v", available, functional, dispatch)
				}
			}
		})
	}
	if (capability.Requirement{}).SatisfiedBy(mustSet(t, extended, read, future)) {
		t.Fatal("missing declaration passed")
	}
}

func TestConcurrentEvaluation(t *testing.T) {
	c := mustCatalog(t, capability.BaseDefinitions()...)
	read, write := capability.FilesRead, capability.FilesWrite
	req, err := c.NewRequirement(read, write)
	if err != nil {
		t.Fatal(err)
	}
	grants := mustSet(t, c, write, read)
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			for range 100 {
				ids := grants.IDs()
				ids[0] = capability.CommandsRun
				if _, known := c.Lookup(read); !known || !req.SatisfiedBy(grants) {
					t.Error("shared immutable input changed")
				}
			}
		})
	}
	group.Wait()
	if allocs := testing.AllocsPerRun(100, func() { req.SatisfiedBy(grants) }); allocs != 0 {
		t.Errorf("evaluation allocated %g times", allocs)
	}
}

func mustCatalog(t *testing.T, definitions ...capability.Definition) capability.Catalog {
	t.Helper()
	c, err := capability.NewCatalog(definitions...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustSet(t *testing.T, c capability.Catalog, ids ...capability.ID) capability.Set {
	t.Helper()
	s, err := c.NewSet(ids...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
