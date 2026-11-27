package fs

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type partialAuditReader struct {
	err error
}

func (r partialAuditReader) Read(buffer []byte) (int, error) {
	return copy(buffer, "part"), r.err
}

type partialAuditWriter struct {
	err error
}

func (w partialAuditWriter) Write(buffer []byte) (int, error) {
	return min(2, len(buffer)), w.err
}

func TestReadAtMostPreservesBytesOnPartialReadError(t *testing.T) {
	failure := errors.New("fixture read failure")
	content, read, err := readAtMost(partialAuditReader{failure}, 16, 16)
	if content != nil || read == nil || *read != 4 || !errors.Is(err, failure) {
		t.Fatalf("read result = %q, %v, %v", content, read, err)
	}
	_, read, err = readAtMost(strings.NewReader("before I/O"), 0, 16)
	if read != nil || !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("pre-I/O rejection = %v, %v", read, err)
	}
}

func TestCopyMeasuredSeparatesPartialReadsAndWrites(t *testing.T) {
	readError := errors.New("fixture read failure")
	writeError := errors.New("fixture write failure")
	for _, tc := range []struct {
		name, data string
		source     io.Reader
		target     io.Writer
		want       error
	}{
		{"partial write", "source", strings.NewReader("source"), partialAuditWriter{writeError}, writeError},
		{"short write", "source", strings.NewReader("source"), partialAuditWriter{}, io.ErrShortWrite},
		{"partial read", "part", partialAuditReader{readError}, partialAuditWriter{}, io.ErrShortWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := copyMeasured(tc.target, tc.source, 16)
			if !errors.Is(err, tc.want) || got.Read == nil || *got.Read != uint64(len(tc.data)) ||
				got.Written == nil || *got.Written != 2 {
				t.Fatalf("incorrect partial counts: %+v (%v)", got, err)
			}
		})
	}
	got, err := copyMeasured(io.Discard, partialAuditReader{readError}, 16)
	if !errors.Is(err, readError) || got.Read == nil || *got.Read != 4 || got.Written == nil || *got.Written != 4 {
		t.Fatalf("incorrect read failure counts: %+v (%v)", got, err)
	}
	got, err = copyMeasured(io.Discard, strings.NewReader(""), 16)
	if err != nil || got.Read == nil || *got.Read != 0 || got.Written != nil {
		t.Fatalf("an unattempted write was measured: %+v (%v)", got, err)
	}
}
