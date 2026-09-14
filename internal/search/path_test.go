package search

import (
	"context"
	"errors"
	"reflect"
	"testing"

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
