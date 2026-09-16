package command

import (
	"bytes"
	"errors"
	"sync"
	"testing"
)

func TestBoundedOutputKeepsStreamsSeparateAndMarksTruncation(t *testing.T) {
	output, err := NewBoundedOutput(5, 3)
	if err != nil {
		t.Fatal(err)
	}

	if n, err := output.Stdout.Write([]byte("hello world")); err != nil || n != 11 {
		t.Fatalf("stdout Write() = (%d, %v), want (11, nil)", n, err)
	}
	if n, err := output.Stderr.Write([]byte("bad")); err != nil || n != 3 {
		t.Fatalf("stderr Write() = (%d, %v), want (3, nil)", n, err)
	}

	stdout := output.Stdout.Snapshot()
	stderr := output.Stderr.Snapshot()
	if !bytes.Equal(stdout.Bytes, []byte("hello")) || stdout.TotalBytes != 11 || !stdout.Truncated {
		t.Fatalf("stdout snapshot = %#v", stdout)
	}
	if !bytes.Equal(stderr.Bytes, []byte("bad")) || stderr.TotalBytes != 3 || stderr.Truncated {
		t.Fatalf("stderr snapshot = %#v", stderr)
	}
}

func TestBoundedStreamContinuesDrainingAfterLimit(t *testing.T) {
	stream, err := NewBoundedStream(2)
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range [][]byte{[]byte("ab"), []byte("cd"), []byte("ef")} {
		if n, err := stream.Write(chunk); err != nil || n != len(chunk) {
			t.Fatalf("Write(%q) = (%d, %v)", chunk, n, err)
		}
	}
	got := stream.Snapshot()
	if !bytes.Equal(got.Bytes, []byte("ab")) || got.TotalBytes != 6 || !got.Truncated {
		t.Fatalf("Snapshot() = %#v", got)
	}
}

func TestBoundedStreamSnapshotDoesNotAliasBuffer(t *testing.T) {
	stream, err := NewBoundedStream(4)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = stream.Write([]byte("data"))
	first := stream.Snapshot()
	first.Bytes[0] = 'X'
	if got := stream.Snapshot(); !bytes.Equal(got.Bytes, []byte("data")) {
		t.Fatalf("Snapshot() exposed mutable storage: %q", got.Bytes)
	}
}

func TestBoundedOutputRejectsInvalidLimits(t *testing.T) {
	for _, limits := range [][2]int64{{0, 1}, {-1, 1}, {1, 0}, {1, -1}} {
		if _, err := NewBoundedOutput(limits[0], limits[1]); !errors.Is(err, ErrInvalidOutputLimit) {
			t.Fatalf("NewBoundedOutput(%d, %d) error = %v", limits[0], limits[1], err)
		}
	}
}

func TestBoundedStreamsAllowConcurrentDraining(t *testing.T) {
	output, err := NewBoundedOutput(1000, 1000)
	if err != nil {
		t.Fatal(err)
	}

	var writers sync.WaitGroup
	for _, stream := range []*BoundedStream{output.Stdout, output.Stderr} {
		stream := stream
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range 100 {
				_, _ = stream.Write([]byte("0123456789"))
			}
		}()
	}
	writers.Wait()

	for name, snapshot := range map[string]OutputSnapshot{"stdout": output.Stdout.Snapshot(), "stderr": output.Stderr.Snapshot()} {
		if len(snapshot.Bytes) != 1000 || snapshot.TotalBytes != 1000 || snapshot.Truncated {
			t.Fatalf("%s snapshot = %#v", name, snapshot)
		}
	}
}
