package search

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

type listingFS struct {
	entries map[string][]fs.Entry
	content map[string][]byte
	calls   []string
	reads   []string
	err     error
}

func (f *listingFS) List(path string) ([]fs.Entry, error) {
	f.calls = append(f.calls, path)
	if f.err != nil {
		return nil, f.err
	}
	return append([]fs.Entry(nil), f.entries[path]...), nil
}

func (f *listingFS) Read(path string, maxBytes int64) ([]byte, error) {
	f.reads = append(f.reads, path)
	if f.err != nil {
		return nil, f.err
	}
	content := f.content[path]
	if int64(len(content)) > maxBytes {
		return nil, fs.ErrFileTooLarge
	}
	return append([]byte(nil), content...), nil
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

func TestPathPatternsUseRelativePathsAndExclusionsWin(t *testing.T) {
	filesystem := &listingFS{entries: map[string][]fs.Entry{
		".":        {{Name: "docs", IsDir: true}, {Name: "top.md"}},
		"docs":     {{Name: "guide.md"}, {Name: "draft.md"}, {Name: "sub", IsDir: true}},
		"docs/sub": {{Name: "nested.md"}},
	}}
	service, _ := NewPathService(filesystem, 10)

	got, err := service.SearchFilteredPatterns(context.Background(), ".", "", NameMatchLiteral, MetadataFilter{}, PathPatterns{
		Include: []string{"docs/*.md"},
		Exclude: []string{"docs/draft.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []Path{{Path: "docs/guide.md"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected path-pattern results\nwant: %#v\n got: %#v", want, got)
	}
	if wantCalls := []string{".", "docs", "docs/sub"}; !reflect.DeepEqual(filesystem.calls, wantCalls) {
		t.Fatalf("unmatched directories must remain traversable: %#v", filesystem.calls)
	}
}

func TestPathPatternsRejectInvalidSyntaxBeforeTraversal(t *testing.T) {
	tooMany := make([]string, maxPathPatterns+1)
	for i := range tooMany {
		tooMany[i] = string(rune('a' + i))
	}
	for _, patterns := range []PathPatterns{
		{Include: []string{""}}, {Include: []string{"["}}, {Exclude: []string{`docs\*.md`}},
		{Include: []string{"same", "same"}}, {Include: []string{strings.Repeat("a", maxPatternBytes+1)}}, {Exclude: tooMany},
	} {
		filesystem := &listingFS{}
		service, _ := NewPathService(filesystem, 10)
		results, err := service.SearchFilteredPatterns(context.Background(), ".", "", NameMatchLiteral, MetadataFilter{}, patterns)
		if !errors.Is(err, ErrInvalidPathPatterns) || results != nil || len(filesystem.calls) != 0 {
			t.Fatalf("expected pre-traversal rejection, results=%#v calls=%#v err=%v", results, filesystem.calls, err)
		}
	}
}

func TestContentSearchAppliesPathPatternsBeforeRead(t *testing.T) {
	filesystem := &listingFS{
		entries: map[string][]fs.Entry{".": {{Name: "keep.txt", Size: 1}, {Name: "skip.txt", Size: 1}}},
		content: map[string][]byte{"keep.txt": []byte("x"), "skip.txt": []byte("x")},
	}
	service, _ := NewPathService(filesystem, 10)
	got, err := service.SearchLiteralPatterns(context.Background(), ".", "", NameMatchLiteral, MetadataFilter{}, PathPatterns{
		Include: []string{"*.txt"}, Exclude: []string{"skip.txt"},
	}, "x", LiteralLimits{MaxFiles: 10, MaxBytesPerFile: 10, MaxScannedBytes: 20, MaxMatchesPerFile: 10, MaxMatches: 10, MaxResponseBytes: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []LiteralMatch{{Path: "keep.txt", ByteOffset: 0}}) || !reflect.DeepEqual(filesystem.reads, []string{"keep.txt"}) {
		t.Fatalf("unexpected matches=%#v reads=%#v", got, filesystem.reads)
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

func TestLiteralSearchReturnsDeterministicByteOffsetsAndComposesFilters(t *testing.T) {
	filesystem := &listingFS{
		entries: map[string][]fs.Entry{
			".":    {{Name: "z.txt", Size: 11}, {Name: "docs", IsDir: true}},
			"docs": {{Name: "a.txt", Size: 8}, {Name: "skip.go", Size: 6}},
		},
		content: map[string][]byte{"z.txt": []byte("needle x ne"), "docs/a.txt": []byte("needle--"), "docs/skip.go": []byte("needle")},
	}
	service, _ := NewPathService(filesystem, 10)
	got, err := service.SearchLiteral(context.Background(), ".", "*.txt", NameMatchPattern, MetadataFilter{Type: EntryTypeFile}, "ne", LiteralLimits{
		MaxFiles: 10, MaxBytesPerFile: 100, MaxScannedBytes: 100, MaxMatchesPerFile: 10, MaxMatches: 10, MaxResponseBytes: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []LiteralMatch{{Path: "docs/a.txt", ByteOffset: 0}, {Path: "z.txt", ByteOffset: 0}, {Path: "z.txt", ByteOffset: 9}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected literal matches\nwant: %#v\n got: %#v", want, got)
	}
	if !reflect.DeepEqual(filesystem.reads, []string{"z.txt", "docs/a.txt"}) {
		t.Fatalf("unexpected reads: %#v", filesystem.reads)
	}
}

func TestLiteralSearchRejectsInvalidRequestBeforeTraversal(t *testing.T) {
	invalidUTF8 := string([]byte{0xff})
	for _, tc := range []struct {
		term   string
		filter MetadataFilter
		limits LiteralLimits
	}{
		{term: "", limits: LiteralLimits{MaxFiles: 1, MaxBytesPerFile: 1, MaxScannedBytes: 1, MaxMatchesPerFile: 1, MaxMatches: 1, MaxResponseBytes: 1}},
		{term: invalidUTF8, limits: LiteralLimits{MaxFiles: 1, MaxBytesPerFile: 1, MaxScannedBytes: 1, MaxMatchesPerFile: 1, MaxMatches: 1, MaxResponseBytes: 1}},
		{term: "x", filter: MetadataFilter{Type: EntryTypeDirectory}, limits: LiteralLimits{MaxFiles: 1, MaxBytesPerFile: 1, MaxScannedBytes: 1, MaxMatchesPerFile: 1, MaxMatches: 1, MaxResponseBytes: 1}},
		{term: "x", limits: LiteralLimits{}},
	} {
		filesystem := &listingFS{}
		service, _ := NewPathService(filesystem, 10)
		got, err := service.SearchLiteral(context.Background(), ".", "", NameMatchLiteral, tc.filter, tc.term, tc.limits)
		if !errors.Is(err, ErrInvalidLiteralSearch) || got != nil || len(filesystem.calls) != 0 {
			t.Fatalf("expected pre-traversal rejection, got=%#v calls=%#v err=%v", got, filesystem.calls, err)
		}
	}
}

func TestLiteralSearchEnforcesIncrementalBudgets(t *testing.T) {
	base := &listingFS{entries: map[string][]fs.Entry{".": {{Name: "a", Size: 4}, {Name: "b", Size: 4}}}, content: map[string][]byte{"a": []byte("xxxx"), "b": []byte("xxxx")}}
	service, _ := NewPathService(base, 10)
	limits := LiteralLimits{MaxFiles: 1, MaxBytesPerFile: 4, MaxScannedBytes: 8, MaxMatchesPerFile: 4, MaxMatches: 8, MaxResponseBytes: 1000}
	if got, err := service.SearchLiteral(context.Background(), ".", "", NameMatchLiteral, MetadataFilter{}, "x", limits); !errors.Is(err, ErrScanLimitExceeded) || got != nil {
		t.Fatalf("expected scan limit, got=%#v err=%v", got, err)
	}

	base = &listingFS{entries: map[string][]fs.Entry{".": {{Name: "a", Size: 4}}}, content: map[string][]byte{"a": []byte("xxxx")}}
	service, _ = NewPathService(base, 10)
	limits.MaxFiles, limits.MaxMatchesPerFile = 1, 2
	if got, err := service.SearchLiteral(context.Background(), ".", "", NameMatchLiteral, MetadataFilter{}, "x", limits); !errors.Is(err, ErrMatchLimitExceeded) || got != nil {
		t.Fatalf("expected match limit, got=%#v err=%v", got, err)
	}

	limits.MaxMatchesPerFile, limits.MaxResponseBytes = 4, 20
	if got, err := service.SearchLiteral(context.Background(), ".", "", NameMatchLiteral, MetadataFilter{}, "x", limits); !errors.Is(err, ErrResponseLimitExceeded) || got != nil {
		t.Fatalf("expected response limit, got=%#v err=%v", got, err)
	}
}

func TestRegexSearchReturnsDeterministicByteOffsetsAndComposesFilters(t *testing.T) {
	filesystem := &listingFS{
		entries: map[string][]fs.Entry{
			".":    {{Name: "z.txt", Size: 9}, {Name: "docs", IsDir: true}},
			"docs": {{Name: "a.txt", Size: 8}, {Name: "skip.go", Size: 6}},
		},
		content: map[string][]byte{"z.txt": []byte("ab12 cd3"), "docs/a.txt": []byte("x99 y007"), "docs/skip.go": []byte("z12345")},
	}
	service, _ := NewPathService(filesystem, 10)
	got, err := service.SearchRegex(context.Background(), ".", "*.txt", NameMatchPattern, MetadataFilter{Type: EntryTypeFile}, `[a-z][0-9]+`, LiteralLimits{
		MaxFiles: 10, MaxBytesPerFile: 100, MaxScannedBytes: 100, MaxMatchesPerFile: 10, MaxMatches: 10, MaxResponseBytes: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []LiteralMatch{{Path: "docs/a.txt", ByteOffset: 0}, {Path: "docs/a.txt", ByteOffset: 4}, {Path: "z.txt", ByteOffset: 1}, {Path: "z.txt", ByteOffset: 6}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected regex matches\nwant: %#v\n got: %#v", want, got)
	}
	if !reflect.DeepEqual(filesystem.reads, []string{"z.txt", "docs/a.txt"}) {
		t.Fatalf("unexpected reads: %#v", filesystem.reads)
	}
}

func TestRegexSearchRejectsInvalidExpressionBeforeTraversal(t *testing.T) {
	limits := LiteralLimits{MaxFiles: 1, MaxBytesPerFile: 1, MaxScannedBytes: 1, MaxMatchesPerFile: 1, MaxMatches: 1, MaxResponseBytes: 1}
	for _, expression := range []string{"", "[", string([]byte{0xff})} {
		filesystem := &listingFS{}
		service, _ := NewPathService(filesystem, 10)
		got, err := service.SearchRegex(context.Background(), ".", "", NameMatchLiteral, MetadataFilter{}, expression, limits)
		if !errors.Is(err, ErrInvalidRegexSearch) || got != nil || len(filesystem.calls) != 0 {
			t.Fatalf("expression %q: expected pre-traversal rejection, got=%#v calls=%#v err=%v", expression, got, filesystem.calls, err)
		}
	}
}

func TestRegexSearchEnforcesExistingContentBudgets(t *testing.T) {
	filesystem := &listingFS{entries: map[string][]fs.Entry{".": {{Name: "a", Size: 4}}}, content: map[string][]byte{"a": []byte("xxxx")}}
	service, _ := NewPathService(filesystem, 10)
	limits := LiteralLimits{MaxFiles: 1, MaxBytesPerFile: 4, MaxScannedBytes: 4, MaxMatchesPerFile: 2, MaxMatches: 4, MaxResponseBytes: 1000}
	if got, err := service.SearchRegex(context.Background(), ".", "", NameMatchLiteral, MetadataFilter{}, `x`, limits); !errors.Is(err, ErrMatchLimitExceeded) || got != nil {
		t.Fatalf("expected match limit, got=%#v err=%v", got, err)
	}
}
