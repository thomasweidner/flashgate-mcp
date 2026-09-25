package releaseevidence

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	d := t.TempDir()
	artifact := filepath.Join(d, "flashgate-mcp_1.0.0_linux_x64.tar.gz")
	mustWrite(t, artifact, "archive")
	s := sha256.Sum256([]byte("archive"))
	checksum := artifact + ".sha256"
	mustWrite(t, checksum, fmt.Sprintf("%x  %s\n", s, filepath.Base(artifact)))
	mod := filepath.Join(d, "go.mod")
	mustWrite(t, mod, "module example.test/app\n\ngo 1.26.4\n\nrequire example.test/dep v1.2.3\n")
	sum := filepath.Join(d, "go.sum")
	mustWrite(t, sum, "example.test/dep v1.2.3 h1:abc=\n")
	paths, err := Generate(Options{Artifact: artifact, Checksum: checksum, GoMod: mod, GoSum: sum, OutputDirectory: filepath.Join(d, "out"), Version: "1.0.0", Commit: strings.Repeat("a", 40), SourceTime: "2026-01-01T00:00:00Z", Platform: "linux", Architecture: "x64"})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 3 {
		t.Fatalf("paths=%d", len(paths))
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(string(b), "\n") {
			t.Errorf("%s lacks final newline", p)
		}
	}
}

func TestGenerateRejectsChecksumMismatch(t *testing.T) {
	d := t.TempDir()
	a := filepath.Join(d, "a.zip")
	mustWrite(t, a, "a")
	c := a + ".sha256"
	mustWrite(t, c, strings.Repeat("0", 64))
	_, err := Generate(Options{Artifact: a, Checksum: c, GoMod: "x", GoSum: "y", OutputDirectory: d, Version: "1.0.0", Commit: strings.Repeat("a", 40), SourceTime: "2026-01-01T00:00:00Z", Platform: "linux", Architecture: "x64"})
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err=%v", err)
	}
}
func mustWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}
