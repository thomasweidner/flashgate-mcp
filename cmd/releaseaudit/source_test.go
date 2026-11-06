package main

import (
	"errors"
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
		{"reserved development version", []byte(developmentVersion + "\n"), false},
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
		candidate       bool
		valid           bool
	}{
		{"release", valid, true, false, true},
		{"unreleased", "## [Unreleased]\n\n- Work in progress.\n", false, false, true},
		{"missing-section", "## [Unreleased]\n", true, false, false},
		{"missing-unreleased", "## [0.1.0] - 2026-09-24\n- Entry\n", true, false, false},
		{"duplicate-unreleased", "## [Unreleased]\n## [Unreleased]\n", false, false, false},
		{"duplicate-version", valid + "\n## [0.1.0] - 2026-09-25\n- Again\n", true, false, false},
		{"empty-notes", "## [Unreleased]\n## [0.1.0] - 2026-09-24\n### Added\n", true, false, false},
		{"malformed-date", "## [Unreleased]\n## [0.1.0] - 2026-99-99\n- Entry\n", true, false, false},
		{"ambiguous-heading", "## [Unreleased]\n## [0.1.0] pending\n- Entry\n", true, false, false},
		{"candidate undated", "## [Unreleased]\n\n## [0.1.0]\n\n- Candidate.\n", false, true, false},
		{"candidate dated", valid, false, true, true},
		{"candidate missing section", "## [Unreleased]\n", false, true, false},
		{"candidate wrong section", "## [Unreleased]\n## [0.2.0]\n- Wrong.\n", false, true, false},
		{"candidate empty section", "## [Unreleased]\n## [0.1.0]\n### Added\n", false, true, false},
		{"candidate duplicate section", "## [Unreleased]\n## [0.1.0]\n- First.\n## [0.1.0]\n- Second.\n", false, true, false},
		{"candidate malformed section", "## [Unreleased]\n## [0.1.0] pending\n- Entry\n", false, true, false},
		{"duplicate pending entry", "## [Unreleased]\n- Work\n- Work\n", false, false, false},
		{"empty pending entry", "## [Unreleased]\n- Work\n- \n", false, false, false},
		{"historical undated notes", "## [Unreleased]\n## [0.1.0]\n- Historical notes.\n", false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, notes, err := auditChangelog(tc.changelog, "0.1.0", tc.release, tc.candidate)
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

func TestCandidateSourceMode(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("0.1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("## [Unreleased]\n\n## [0.1.0] - 2026-09-24\n\n- Candidate.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSource([]string{"--root", root, "--candidate"}); err == nil {
		t.Fatal("dated section without a Git freeze passed")
	}
	if err := runSource([]string{"--root", root, "--candidate", "--release"}); err == nil {
		t.Fatal("candidate and release accepted together")
	}
	if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("## [Unreleased]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runSource([]string{"--root", root, "--candidate"}); err == nil {
		t.Fatal("candidate without version section passed")
	}
}

func TestFreezeIncrementPrerelease(t *testing.T) {
	for _, tc := range []struct {
		previous, current string
		allowed, patch    bool
	}{
		{"0.4.0-rc.1", "0.4.0-rc.2", true, false},
		{"0.4.0-rc.9", "0.4.0-rc.10", true, false},
		{"0.4.0-rc.10", "0.4.0-rc.9", false, false},
		{"0.4.0-rc.18446744073709551615", "0.4.0-rc.18446744073709551616", true, false},
		{"0.4.0-alpha.1", "0.4.0-alpha.beta", true, false},
		{"0.4.0-alpha.beta", "0.4.0-beta", true, false},
		{"0.4.0-beta", "0.4.0-beta.2", true, false},
		{"0.4.0-beta.2", "0.4.0-beta", false, false},
		{"0.4.0-rc.2", "0.4.0", true, false},
		{"0.3.1-rc.1", "0.3.1-rc.2", true, true},
		{"0.3.1-rc.2", "0.3.1", true, true},
		{"0.4.0", "0.4.0-rc.2", false, false},
		{"0.4.0-rc.2", "0.4.0-rc.2", false, false},
		{"0.4.0-rc.2+first", "0.4.0-rc.2+second", false, false},
		{"0.4.0-rc.2+first", "0.4.0-rc.3+second", true, false},
		{"0.4.0-rc.2", "0.4.0-rc.03", false, false},
	} {
		t.Run(tc.previous+" to "+tc.current, func(t *testing.T) {
			patch, err := freezeIncrement(tc.previous, tc.current)
			if (err == nil) != tc.allowed || patch != tc.patch {
				t.Fatalf("want allowed=%t patch=%t, got patch=%t err=%v", tc.allowed, tc.patch, patch, err)
			}
		})
	}
}

func TestDevelopmentAndReleaseFreeze(t *testing.T) {
	const previous = "0.3.0"
	const published = "## [Unreleased]\n\n## [0.3.0] - 2026-10-23\n\n- Previous release.\n"
	const feature = "### Added\n\n- New public feature.\n\n"
	const fix = "### Fixed\n\n- Corrected an existing behavior.\n\n"
	withPending := func(entries string) string {
		return strings.Replace(published, "## [Unreleased]\n\n", "## [Unreleased]\n\n"+entries, 1)
	}
	freeze := func(value, entries string) string {
		return withPending("## [" + value + "] - 2026-11-06\n\n" + entries)
	}
	const rc1 = "0.4.0-rc.1"
	rcParent := freeze(rc1, feature)
	freezeAfterRC := func(value string) string {
		return strings.Replace(rcParent, "## [Unreleased]\n\n",
			"## [Unreleased]\n\n## ["+value+"] - 2026-11-06\n\n"+fix, 1)
	}
	for _, pending := range []string{feature, feature + fix} {
		if _, _, err := auditChangelog(withPending(pending), previous, false, false); err != nil {
			t.Fatal("development with unchanged VERSION failed", err)
		}
	}
	for _, tc := range []struct {
		name, value, parent, current  string
		priorVersion, tag, gitFailure string
		wantError                     string
		tagged, pass                  bool
	}{
		{name: "MINOR freeze after consecutive development changes", value: "0.4.0", parent: withPending(feature + fix), current: freeze("0.4.0", feature+fix), pass: true},
		{name: "isolated PATCH with main-only feature excluded", value: "0.3.1", parent: withPending(fix), current: freeze("0.3.1", fix), pass: true},
		{name: "PATCH on feature-bearing main", value: "0.3.1", parent: withPending(feature + fix), current: freeze("0.3.1", fix)},
		{name: "PATCH newly adds pending feature", value: "0.3.1", parent: published, current: strings.Replace(freeze("0.3.1", fix), "## [Unreleased]\n\n", "## [Unreleased]\n\n"+feature, 1)},
		{name: "PATCH notes ship feature", value: "0.3.1", parent: published, current: freeze("0.3.1", feature)},
		{name: "PATCH lacks published ancestry", value: "0.3.1", parent: published, current: freeze("0.3.1", fix), gitFailure: "merge-base"},
		{name: "manual dispatch on development main", value: previous, parent: published, current: withPending(feature)},
		{name: "candidate with no new notes", value: "0.4.0", parent: published, current: withPending(feature)},
		{name: "candidate notes already assigned", value: "0.4.0", parent: freeze("0.4.0", feature), current: freeze("0.4.0", feature)},
		{name: "published version cannot mint new candidate bytes", value: "0.4.0", parent: published, current: freeze("0.4.0", feature), tag: strings.Repeat("b", 40)},
		{name: "next RC freeze", value: "0.4.0-rc.2", priorVersion: rc1, parent: rcParent, current: freezeAfterRC("0.4.0-rc.2"), pass: true},
		{name: "final freeze after RC", value: "0.4.0", priorVersion: rc1, parent: rcParent, current: freezeAfterRC("0.4.0"), pass: true},
		{name: "next RC tag already exists", value: "0.4.0-rc.2", priorVersion: rc1, parent: rcParent, current: freezeAfterRC("0.4.0-rc.2"), tag: testCandidateCommit, wantError: "unused VERSION"},
		{name: "tag lookup fails closed", value: "0.4.0", parent: published, current: freeze("0.4.0", feature), gitFailure: "for-each-ref"},
		{name: "working tree observation failed", value: "0.4.0", parent: published, current: freeze("0.4.0", feature), gitFailure: "status"},
		{name: "dirty working tree", value: "0.4.0", parent: published, current: freeze("0.4.0", feature)},
		{name: "wrong Git worktree root", value: "0.4.0", parent: published, current: freeze("0.4.0", feature)},
		{name: "VERSION differs from HEAD", value: "0.4.0", parent: published, current: freeze("0.4.0", feature)},
		{name: "CHANGELOG differs from HEAD", value: "0.4.0", parent: published, current: freeze("0.4.0", feature)},
		{name: "exact tagged freeze", value: "0.4.0", parent: published, current: freeze("0.4.0", feature), tagged: true, tag: testCandidateCommit, pass: true},
		{name: "tag points elsewhere", value: "0.4.0", parent: published, current: freeze("0.4.0", feature), tagged: true, tag: strings.Repeat("b", 40)},
		{name: "tag points at commit with wrong notes", value: "0.4.0", parent: published, current: freeze("0.4.1", feature), tagged: true, tag: testCandidateCommit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			git := func(args ...string) ([]byte, error) {
				if args[0] == tc.gitFailure {
					return nil, errors.New("Git observation failed")
				}
				switch strings.Join(args, " ") {
				case "rev-parse --show-prefix":
					if tc.name == "wrong Git worktree root" {
						return []byte("subdirectory/\n"), nil
					}
					return []byte("\n"), nil
				case "status --porcelain=v1 --untracked-files=all":
					if tc.name == "dirty working tree" {
						return []byte(" M cmd/server/main.go\n"), nil
					}
					return nil, nil
				case "show HEAD:VERSION":
					if tc.name == "VERSION differs from HEAD" {
						return []byte(previous + "\n"), nil
					}
					return []byte(tc.value + "\n"), nil
				case "show HEAD:CHANGELOG.md":
					if tc.name == "CHANGELOG differs from HEAD" {
						return []byte(published), nil
					}
					return []byte(tc.current), nil
				case "merge-base --is-ancestor refs/tags/v0.3.0^{commit} HEAD^1":
					return nil, nil
				case "for-each-ref --format=%(refname) refs/tags/v" + tc.value:
					if tc.tag != "" {
						return []byte("refs/tags/v" + tc.value + "\n"), nil
					}
					return nil, nil
				case "rev-parse HEAD":
					return []byte(testCandidateCommit + "\n"), nil
				case "rev-parse --verify refs/tags/v" + tc.value + "^{commit}":
					return []byte(tc.tag + "\n"), nil
				case "show HEAD^1:VERSION":
					if tc.priorVersion != "" {
						return []byte(tc.priorVersion + "\n"), nil
					}
					return []byte(previous + "\n"), nil
				case "show refs/tags/v0.3.0:VERSION":
					return []byte(previous + "\n"), nil
				case "show HEAD^1:CHANGELOG.md":
					return []byte(tc.parent), nil
				case "show refs/tags/v0.3.0:CHANGELOG.md":
					return []byte(published), nil
				default:
					t.Fatalf("unexpected Git request: %q", args)
					return nil, nil
				}
			}
			_, _, err := auditChangelog(tc.current, tc.value, tc.tagged, !tc.tagged)
			if err == nil {
				err = auditFreeze(git, tc.value, tc.current, tc.tagged)
			}
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%t, got %v", tc.pass, err)
			}
			if tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)) {
				t.Fatalf("want error containing %q, got %v", tc.wantError, err)
			}
		})
	}
	if err := validateCandidateIdentity(developmentVersion, testCandidateCommit); err == nil {
		t.Fatal("development artifact accepted as a release identity")
	}
}
