package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadRepositoryVersion(t *testing.T) {
	cases := []struct {
		name  string
		value []byte
		valid bool
	}{
		{"stable without LF", []byte("0.1.0"), true},
		{"stable with LF", []byte("0.1.0\n"), true},
		{"prerelease", []byte("1.2.3-rc.1"), true},
		{"empty", []byte{}, false},
		{"whitespace", []byte(" 0.1.0\n"), false},
		{"bom", []byte("\xef\xbb\xbf0.1.0\n"), false},
		{"leading-v", []byte("v0.1.0\n"), false},
		{"multiline", []byte("0.1.0\n1.0.0\n"), false},
		{"crlf", []byte("0.1.0\r\n"), false},
		{"cr", []byte("0.1.0\r"), false},
		{"trailing whitespace", []byte("0.1.0 \n"), false},
		{"nul only", []byte{0}, false},
		{"nul final", []byte("0.1.0\x00"), false},
		{"nul internal", []byte("0.1\x00.0"), false},
		{"nul before LF", []byte("0.1.0\x00\n"), false},
		{"nul leading", []byte("\x00.1.0\n"), false},
		{"invalid", []byte("1.2\n"), false},
		{"overflow", []byte("65536.0.0\n"), false},
		{"leading-zero", []byte("1.2.3-rc.01\n"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := filepath.Join(t.TempDir(), "VERSION")
			if err := os.WriteFile(name, tc.value, 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := readRepositoryVersion(name)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t, error=%v", tc.valid, err)
			}
		})
	}
	if _, err := readRepositoryVersion(filepath.Join(t.TempDir(), "VERSION")); err == nil {
		t.Fatal("missing VERSION passed")
	}
}

func TestAuditChangelog(t *testing.T) {
	const valid = "# Changelog\n\n## [Unreleased]\n\n## [0.1.0] - 2026-09-24\n\n### Added\n\n- First release.\n"
	cases := []struct {
		name, changelog string
		release         bool
		valid           bool
	}{
		{"release", valid, true, true},
		{"unreleased", "## [Unreleased]\n\n- Work in progress.\n", false, true},
		{"missing-section", "## [Unreleased]\n", true, false},
		{"missing-unreleased", "## [0.1.0] - 2026-09-24\n- Entry\n", true, false},
		{"duplicate-unreleased", "## [Unreleased]\n## [Unreleased]\n", false, false},
		{"duplicate-version", valid + "\n## [0.1.0] - 2026-09-25\n- Again\n", true, false},
		{"empty-notes", "## [Unreleased]\n## [0.1.0] - 2026-09-24\n### Added\n", true, false},
		{"malformed-date", "## [Unreleased]\n## [0.1.0] - 2026-99-99\n- Entry\n", true, false},
		{"ambiguous-heading", "## [Unreleased]\n## [0.1.0] pending\n- Entry\n", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, notes, err := auditChangelog(tc.changelog, "0.1.0", tc.release)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t, error=%v", tc.valid, err)
			}
			if tc.release && tc.valid && !strings.Contains(notes, "First release.") {
				t.Fatal("generated notes lack release entry")
			}
		})
	}
}

func TestRunSourceReport(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("## [Unreleased]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSource([]string{"--root", root}); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "source.json")
	if err := runSource([]string{"--root", root, "--release", "--report", report}); err == nil {
		t.Fatal("release without notes passed")
	}
	data, err := os.ReadFile(report)
	if err != nil || !strings.Contains(string(data), `"status": "FAIL"`) {
		t.Fatalf("missing typed failure report: %v", err)
	}
	valid := []byte("## [Unreleased]\n\n## [0.1.0] - 2026-09-24\n\n- First release.\n")
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), valid, 0o644); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(root, "CHANGELOG.md")
	report = filepath.Join(t.TempDir(), "protected.json")
	if err := runSource([]string{
		"--root", root, "--release", "--report", report,
		"--notes-output", protected,
	}); err == nil {
		t.Fatal("existing changelog was accepted as generated notes output")
	}
	unchanged, err := os.ReadFile(protected)
	if err != nil || string(unchanged) != string(valid) {
		t.Fatalf("changelog changed during rejected notes output: %v", err)
	}
	data, err = os.ReadFile(report)
	if err != nil || !strings.Contains(string(data), `"status": "FAIL"`) {
		t.Fatalf("notes output failure was not reported: %v", err)
	}
}
