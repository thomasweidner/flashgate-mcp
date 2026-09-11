package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/fs"
)

func TestHashFilesReturnsOrderedFingerprintsAndCounters(t *testing.T) {
	fake := newFakeFileSystem()
	fake.readFunc = func(path string, _ int64) ([]byte, error) {
		return map[string][]byte{"a.txt": []byte("abc"), "empty": []byte{}}[path], nil
	}
	result, rpcErr := NewHashFilesTool(fake, 1024).Execute(context.Background(), json.RawMessage(`{"paths":["a.txt","empty"]}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	got := result.(hashFilesResult)
	if got.Completed != 2 || got.Failed != 0 || got.BytesHashed != 3 || len(got.Items) != 2 {
		t.Fatalf("unexpected result: %#v", got)
	}
	if got.Items[0].Fingerprint != "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("unexpected fingerprint: %q", got.Items[0].Fingerprint)
	}
}

func TestHashFilesEnforcesAggregateLimit(t *testing.T) {
	fake := newFakeFileSystem()
	fake.readFunc = func(_ string, maxBytes int64) ([]byte, error) {
		if maxBytes < 2 {
			return nil, fs.ErrLimitExceeded
		}
		return []byte("ok"), nil
	}
	result, rpcErr := NewHashFilesTool(fake, 2).Execute(context.Background(), json.RawMessage(`{"paths":["first","last"]}`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	got := result.(hashFilesResult)
	if got.Completed != 1 || got.Failed != 1 || got.BytesHashed != 2 || got.Items[1].Error != "limit_exceeded" {
		t.Fatalf("unexpected bounded result: %#v", got)
	}
}

func TestHashFilesRejectsInvalidArgumentsBeforeReading(t *testing.T) {
	for _, raw := range []string{`{}`, `{"paths":[]}`, `{"paths":[""]}`, `{"paths":["a"],"maxBytes":0}`, `{"paths":null}`} {
		fake := newFakeFileSystem()
		_, rpcErr := NewHashFilesTool(fake, 1024).Execute(context.Background(), json.RawMessage(raw))
		if rpcErr == nil || len(fake.readPaths) != 0 {
			t.Fatalf("expected preflight rejection for %s, got error=%#v reads=%v", raw, rpcErr, fake.readPaths)
		}
	}
}
