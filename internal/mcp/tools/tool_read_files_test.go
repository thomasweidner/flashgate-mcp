package tools

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type batchReadFileSystem struct {
	*fakeFileSystem
	content map[string][]byte
	errors  map[string]error
	calls   []string
	limits  []int64
}

func newBatchReadFileSystem() *batchReadFileSystem {
	return &batchReadFileSystem{fakeFileSystem: newFakeFileSystem(), content: map[string][]byte{}, errors: map[string]error{}}
}

func (f *batchReadFileSystem) Read(path string, maxBytes int64) ([]byte, error) {
	f.calls = append(f.calls, path)
	f.limits = append(f.limits, maxBytes)
	return f.content[path], f.errors[path]
}

func TestReadFilesToolDefinition(t *testing.T) {
	t.Parallel()
	definition := NewReadFilesTool(newFakeFileSystem(), 1024).Definition()
	if definition.Name != readFilesToolName || definition.Title != "Read Files" || definition.OutputSchema == nil {
		t.Fatalf("unexpected definition: %#v", definition)
	}
}

func TestReadFilesToolPreservesOrderAndPartialFailures(t *testing.T) {
	fake := newBatchReadFileSystem()
	fake.content["a.txt"] = []byte("alpha")
	fake.errors["missing.txt"] = fs.ErrNotFound
	fake.content["empty.txt"] = []byte{}

	value, rpcErr := NewReadFilesTool(fake, 1024).Execute(context.Background(), json.RawMessage(`{"paths":["a.txt","missing.txt","empty.txt"],"maxBytesPerFile":64}`))
	if rpcErr != nil {
		t.Fatalf("unexpected error: %#v", rpcErr)
	}
	result := value.(readFilesResult)
	if result.Accepted != 3 || result.Completed != 2 || result.Failed != 1 || result.TotalBytes != 5 {
		t.Fatalf("unexpected counters: %#v", result)
	}
	if !reflect.DeepEqual(fake.calls, []string{"a.txt", "missing.txt", "empty.txt"}) || !reflect.DeepEqual(fake.limits, []int64{64, 64, 64}) {
		t.Fatalf("unexpected reads: calls=%v limits=%v", fake.calls, fake.limits)
	}
	if result.Results[0].Content == nil || *result.Results[0].Content != "alpha" || result.Results[1].Error != "not_found" || result.Results[2].Content == nil || *result.Results[2].Content != "" {
		t.Fatalf("unexpected ordered results: %#v", result.Results)
	}
}

func TestReadFilesToolEnforcesAggregateByteLimit(t *testing.T) {
	fake := newBatchReadFileSystem()
	fake.content["first"] = []byte("1234")
	fake.content["second"] = []byte("5")
	value, rpcErr := NewReadFilesTool(fake, 4).Execute(context.Background(), json.RawMessage(`{"paths":["first","second"]}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result := value.(readFilesResult)
	if result.Completed != 1 || result.Failed != 1 || result.Results[1].Error != "aggregate_limit_exceeded" || len(fake.calls) != 1 {
		t.Fatalf("aggregate limit not enforced: result=%#v calls=%v", result, fake.calls)
	}
}

func TestReadFilesToolValidatesWholeBatchBeforeReading(t *testing.T) {
	tests := []string{`{}`, `{"paths":[]}`, `{"paths":["ok",""]}`, `{"paths":["ok","  "]}`, `{"paths":["ok"],"maxBytesPerFile":0}`, `{"paths":null}`}
	for _, raw := range tests {
		fake := newBatchReadFileSystem()
		_, rpcErr := NewReadFilesTool(fake, 10).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams || len(fake.calls) != 0 {
			t.Fatalf("expected preflight rejection for %s, error=%#v calls=%v", raw, rpcErr, fake.calls)
		}
	}

	paths := make([]string, maxReadFilesPaths+1)
	for i := range paths {
		paths[i] = "file"
	}
	raw, _ := json.Marshal(readFilesArguments{Paths: paths})
	fake := newBatchReadFileSystem()
	_, rpcErr := NewReadFilesTool(fake, 10).Execute(context.Background(), raw)
	if rpcErr == nil || len(fake.calls) != 0 {
		t.Fatalf("expected oversized batch rejection, got %#v", rpcErr)
	}
}

func TestReadFilesErrorCategories(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{fs.ErrFileTooLarge, "limit_exceeded"}, {fs.ErrLimitExceeded, "limit_exceeded"},
		{fs.ErrPathIsDirectory, "path_is_directory"}, {fs.ErrNotFound, "not_found"},
		{errors.New("host detail"), "read_failed"},
	}
	for _, tc := range tests {
		if got := readFilesErrorCategory(tc.err); got != tc.want {
			t.Fatalf("got %q, want %q", got, tc.want)
		}
	}
}
