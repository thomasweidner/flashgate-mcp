package search

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

type listingFS struct {
	entries map[string][]fs.Entry
	calls   []string
	err     error
}

func (f *listingFS) List(path string) ([]fs.Entry, error) {
	f.calls = append(f.calls, path)
	if f.err != nil {
		return nil, f.err
	}
	return append([]fs.Entry(nil), f.entries[path]...), nil
}

func TestPathSearchReturnsRootRelativePathsInPortableOrder(t *testing.T) {
	filesystem := &listingFS{entries: map[string][]fs.Entry{
		".":     {{Name: "z.txt"}, {Name: "a", IsDir: true}, {Name: "ä.txt"}},
		"a":     {{Name: "two.txt"}, {Name: "one", IsDir: true}},
		"a/one": {{Name: "deep.txt"}},
	}}
	service, err := NewPathService(filesystem, 10)
	if err != nil {
		t.Fatal(err)
	}

	got, err := service.Search(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}
	want := []Path{
		{Path: "a", IsDir: true},
		{Path: "a/one", IsDir: true},
		{Path: "a/one/deep.txt"},
		{Path: "a/two.txt"},
		{Path: "z.txt"},
		{Path: "ä.txt"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected paths\nwant: %#v\n got: %#v", want, got)
	}
	if wantCalls := []string{".", "a", "a/one"}; !reflect.DeepEqual(filesystem.calls, wantCalls) {
		t.Fatalf("unexpected traversal calls: %#v", filesystem.calls)
	}
}

func TestPathSearchKeepsStartPathRootRelative(t *testing.T) {
	filesystem := &listingFS{entries: map[string][]fs.Entry{"docs": {{Name: "nested", IsDir: true}}, "docs/nested": {{Name: "file.md"}}}}
	service, _ := NewPathService(filesystem, 10)

	got, err := service.Search(context.Background(), "docs")
	if err != nil {
		t.Fatal(err)
	}
	want := []Path{{Path: "docs/nested", IsDir: true}, {Path: "docs/nested/file.md"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected paths: %#v", got)
	}
}

func TestPathSearchFailsClosedAtLimit(t *testing.T) {
	service, _ := NewPathService(&listingFS{entries: map[string][]fs.Entry{".": {{Name: "a"}, {Name: "b"}}}}, 1)
	results, err := service.Search(context.Background(), ".")
	if !errors.Is(err, ErrLimitExceeded) || results != nil {
		t.Fatalf("expected bounded failure, got results=%#v err=%v", results, err)
	}
}

func TestPathSearchHonorsCancellationBeforeFilesystemAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	filesystem := &listingFS{}
	service, _ := NewPathService(filesystem, 10)

	_, err := service.Search(ctx, ".")
	if !errors.Is(err, context.Canceled) || len(filesystem.calls) != 0 {
		t.Fatalf("expected cancellation without access, calls=%#v err=%v", filesystem.calls, err)
	}
}

func TestNewPathServiceRequiresPositiveLimit(t *testing.T) {
	if _, err := NewPathService(&listingFS{}, 0); !errors.Is(err, ErrInvalidLimit) {
		t.Fatalf("expected invalid limit, got %v", err)
	}
}

func TestFilenameSearchMatchesLiteralAndTraversesUnmatchedDirectories(t *testing.T) {
	filesystem := &listingFS{entries: map[string][]fs.Entry{
		".":     {{Name: "target.txt"}, {Name: "other", IsDir: true}},
		"other": {{Name: "target.txt"}, {Name: "TARGET.txt"}},
	}}
	service, _ := NewPathService(filesystem, 10)

	got, err := service.SearchNames(context.Background(), ".", "target.txt", NameMatchLiteral)
	if err != nil {
		t.Fatal(err)
	}
	want := []Path{{Path: "other/target.txt"}, {Path: "target.txt"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected literal matches\nwant: %#v\n got: %#v", want, got)
	}
}

func TestFilenameSearchMatchesPortablePatternAgainstBaseName(t *testing.T) {
	filesystem := &listingFS{entries: map[string][]fs.Entry{
		".":        {{Name: "docs", IsDir: true}, {Name: "top.go"}},
		"docs":     {{Name: "guide.md"}, {Name: "notes.txt"}, {Name: "sub", IsDir: true}},
		"docs/sub": {{Name: "nested.md"}},
	}}
	service, _ := NewPathService(filesystem, 10)

	got, err := service.SearchNames(context.Background(), ".", "*.md", NameMatchPattern)
	if err != nil {
		t.Fatal(err)
	}
	want := []Path{{Path: "docs/guide.md"}, {Path: "docs/sub/nested.md"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected pattern matches\nwant: %#v\n got: %#v", want, got)
	}
}

func TestFilenameSearchRejectsInvalidSelectorBeforeTraversal(t *testing.T) {
	for _, tc := range []struct {
		selector string
		kind     NameMatchKind
	}{
		{"", NameMatchLiteral},
		{"dir/file.txt", NameMatchLiteral},
		{"[", NameMatchPattern},
		{"file.txt", NameMatchKind(99)},
	} {
		filesystem := &listingFS{}
		service, _ := NewPathService(filesystem, 10)
		results, err := service.SearchNames(context.Background(), ".", tc.selector, tc.kind)
		if !errors.Is(err, ErrInvalidNameSelector) || results != nil || len(filesystem.calls) != 0 {
			t.Fatalf("selector %q: expected pre-traversal rejection, results=%#v calls=%#v err=%v", tc.selector, results, filesystem.calls, err)
		}
	}
}

func TestFilenameSearchLimitCountsOnlyMatches(t *testing.T) {
	filesystem := &listingFS{entries: map[string][]fs.Entry{".": {{Name: "a.txt"}, {Name: "b.go"}, {Name: "c.txt"}}}}
	service, _ := NewPathService(filesystem, 1)

	results, err := service.SearchNames(context.Background(), ".", "*.go", NameMatchPattern)
	if err != nil || !reflect.DeepEqual(results, []Path{{Path: "b.go"}}) {
		t.Fatalf("expected one bounded match, results=%#v err=%v", results, err)
	}
}

func TestMetadataSearchFiltersPortableTypeSizeAndTime(t *testing.T) {
	old := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	recent := old.Add(24 * time.Hour)
	filesystem := &listingFS{entries: map[string][]fs.Entry{
		".": {
			{Name: "docs", IsDir: true, ModifiedTime: recent},
			{Name: "small.txt", Size: 4, ModifiedTime: recent},
			{Name: "wanted.txt", Size: 8, ModifiedTime: recent},
			{Name: "old.txt", Size: 8, ModifiedTime: old},
		},
		"docs": {{Name: "nested.txt", Size: 8, ModifiedTime: recent}},
	}}
	service, _ := NewPathService(filesystem, 10)
	minSize, maxSize := int64(8), int64(8)
	got, err := service.SearchFiltered(context.Background(), ".", "*.txt", NameMatchPattern, MetadataFilter{
		Type: EntryTypeFile, MinSizeBytes: &minSize, MaxSizeBytes: &maxSize, ModifiedNotBefore: &recent, ModifiedNotAfter: &recent,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []Path{{Path: "docs/nested.txt"}, {Path: "wanted.txt"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected metadata matches\nwant: %#v\n got: %#v", want, got)
	}
}

func TestMetadataSearchSizeFiltersNeverMatchDirectories(t *testing.T) {
	zero := int64(0)
	service, _ := NewPathService(&listingFS{entries: map[string][]fs.Entry{".": {{Name: "empty", IsDir: true}, {Name: "empty.txt"}}, "empty": {}}}, 10)
	got, err := service.SearchFiltered(context.Background(), ".", "", NameMatchLiteral, MetadataFilter{MinSizeBytes: &zero, MaxSizeBytes: &zero})
	if err != nil || !reflect.DeepEqual(got, []Path{{Path: "empty.txt"}}) {
		t.Fatalf("expected only zero-byte file, results=%#v err=%v", got, err)
	}
}

func TestMetadataSearchRejectsInvalidFiltersBeforeTraversal(t *testing.T) {
	negative, small, large := int64(-1), int64(1), int64(2)
	later := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	earlier := later.Add(-time.Hour)
	filters := []MetadataFilter{
		{Type: EntryType(99)},
		{MinSizeBytes: &negative},
		{MinSizeBytes: &large, MaxSizeBytes: &small},
		{ModifiedNotBefore: &later, ModifiedNotAfter: &earlier},
	}
	for _, filter := range filters {
		filesystem := &listingFS{}
		service, _ := NewPathService(filesystem, 10)
		results, err := service.SearchFiltered(context.Background(), ".", "", NameMatchLiteral, filter)
		if !errors.Is(err, ErrInvalidMetadataFilter) || results != nil || len(filesystem.calls) != 0 {
			t.Fatalf("expected pre-traversal rejection, filter=%#v results=%#v calls=%#v err=%v", filter, results, filesystem.calls, err)
		}
	}
}

func TestMetadataSearchLimitCountsOnlyMatches(t *testing.T) {
	minSize := int64(10)
	filesystem := &listingFS{entries: map[string][]fs.Entry{".": {{Name: "small-a", Size: 1}, {Name: "large", Size: 10}, {Name: "small-b", Size: 2}}}}
	service, _ := NewPathService(filesystem, 1)
	got, err := service.SearchFiltered(context.Background(), ".", "", NameMatchLiteral, MetadataFilter{MinSizeBytes: &minSize})
	if err != nil || !reflect.DeepEqual(got, []Path{{Path: "large"}}) {
		t.Fatalf("expected one bounded metadata match, results=%#v err=%v", got, err)
	}
}
