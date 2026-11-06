package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thomasweidner/flashgate-mcp/internal/version"
)

const maximumChangelogSize = 2 << 20
const developmentVersion = "0.0.0-dev"

var versionHeading = regexp.MustCompile(`^## \[([^]]+)\](?: - ([0-9]{4}-[0-9]{2}-[0-9]{2}))?$`)

type sourceReport struct {
	Schema       string   `json:"schema"`
	Status       string   `json:"status"`
	Version      string   `json:"version"`
	ExpectedTag  string   `json:"expectedTag"`
	Release      bool     `json:"release"`
	Candidate    bool     `json:"candidate,omitempty"`
	ReleaseDate  string   `json:"releaseDate,omitempty"`
	ReleaseNotes string   `json:"releaseNotes,omitempty"`
	Errors       []string `json:"errors"`
}

func runSource(arguments []string) error {
	flags := flag.NewFlagSet("source", flag.ContinueOnError)
	root := flags.String("root", "", "repository root")
	release := flags.Bool("release", false, "require dated release notes for VERSION")
	candidate := flags.Bool("candidate", false, "validate the exact untagged release freeze at HEAD")
	taggedFreeze := flags.Bool("tagged-freeze", false, "validate the exact tagged release freeze at HEAD")
	reportPath := flags.String("report", "", "optional JSON report path")
	notesPath := flags.String("notes-output", "", "optional generated release notes path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *root == "" || flags.NArg() != 0 {
		return errors.New("source requires --root and no positional arguments")
	}
	if *candidate && *release {
		return errors.New("--candidate and --release are mutually exclusive")
	}
	if *taggedFreeze && !*release {
		return errors.New("--tagged-freeze requires --release")
	}
	if *notesPath != "" && !*release {
		return errors.New("--notes-output requires --release")
	}
	report := sourceReport{
		Schema: "flashgate-release-source/v1", Status: "PASS",
		Release: *release, Candidate: *candidate, Errors: []string{},
	}
	value, err := readRepositoryVersion(filepath.Join(*root, "VERSION"))
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
	} else {
		report.Version = value
		report.ExpectedTag = "v" + value
	}
	changelog, readErr := os.ReadFile(filepath.Join(*root, "CHANGELOG.md"))
	if readErr != nil {
		report.Errors = append(report.Errors, readErr.Error())
	} else {
		date, notes, auditErr := auditChangelog(string(changelog), value, *release, *candidate)
		if auditErr == nil && (*candidate || *taggedFreeze) && err == nil {
			git := func(args ...string) ([]byte, error) {
				command := exec.Command("git", append([]string{"-C", *root}, args...)...)
				command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
				return command.Output()
			}
			auditErr = auditFreeze(git, value, string(changelog), *taggedFreeze)
		}
		if auditErr != nil {
			report.Errors = append(report.Errors, auditErr.Error())
		} else {
			report.ReleaseDate = date
			if *release {
				report.ReleaseNotes = notes
			}
		}
	}
	if len(report.Errors) != 0 {
		report.Status = "FAIL"
	}
	if report.Status == "PASS" && *notesPath != "" {
		if err := writeReleaseNotes(*notesPath, report.ReleaseNotes); err != nil {
			report.Status = "FAIL"
			report.Errors = append(report.Errors, "generated release notes: "+err.Error())
		}
	}
	if *reportPath != "" {
		if err := writeJSON(*reportPath, report); err != nil {
			return err
		}
	}
	if report.Status != "PASS" {
		return errors.New("release source validation failed: " + strings.Join(report.Errors, "; "))
	}
	return nil
}

func writeReleaseNotes(name, notes string) error {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(notes)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func readRepositoryVersion(name string) (string, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return "", fmt.Errorf("VERSION: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 128 {
		return "", errors.New("VERSION must be a nonempty regular file of at most 128 bytes")
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("VERSION: %w", err)
	}
	return parseRepositoryVersion(data)
}

func parseRepositoryVersion(data []byte) (string, error) {
	if len(data) > 128 || !utf8.Valid(data) || bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return "", errors.New("VERSION must be UTF-8 without BOM and at most 128 bytes")
	}
	value := strings.TrimSuffix(string(data), "\n")
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", errors.New("VERSION must contain one SemVer line with optional LF")
	}
	if _, err := version.WindowsFileVersion(value); err != nil {
		return "", fmt.Errorf("VERSION: %w", err)
	}
	if value == developmentVersion {
		return "", errors.New("VERSION cannot use the reserved development version")
	}
	return value, nil
}

type changelogSection struct {
	date    string
	lines   []string
	entries map[string]string
}

func parseChangelog(contents string) (map[string]*changelogSection, error) {
	if len(contents) == 0 || len(contents) > maximumChangelogSize ||
		!utf8.ValidString(contents) || strings.HasPrefix(contents, "\ufeff") {
		return nil, errors.New("CHANGELOG.md has invalid encoding or size")
	}
	sections := make(map[string]*changelogSection)
	var section *changelogSection
	var category string
	for _, line := range strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			match := versionHeading.FindStringSubmatch(line)
			if match == nil {
				return nil, errors.New("invalid changelog section heading")
			}
			name, date := match[1], match[2]
			if _, exists := sections[name]; exists {
				return nil, fmt.Errorf("duplicate changelog section [%s]", name)
			}
			if name == "Unreleased" {
				if date != "" || section != nil {
					return nil, errors.New("[Unreleased] must be the first section and undated")
				}
			} else if _, err := parseRepositoryVersion([]byte(name)); err != nil {
				return nil, fmt.Errorf("invalid changelog version: %w", err)
			}
			if date != "" {
				if _, err := time.Parse("2006-01-02", date); err != nil {
					return nil, errors.New("release heading has invalid date")
				}
			}
			section = &changelogSection{date: date, entries: make(map[string]string)}
			sections[name] = section
			category = ""
		case section != nil:
			section.lines = append(section.lines, line)
			if strings.HasPrefix(line, "### ") {
				category = strings.ToLower(strings.TrimSpace(line[4:]))
			}
			if line == "-" || strings.HasPrefix(line, "- ") {
				entry := strings.TrimSpace(line[1:])
				if entry == "" {
					return nil, errors.New("empty changelog entry")
				}
				if _, exists := section.entries[entry]; exists {
					return nil, errors.New("duplicate changelog entry")
				}
				section.entries[entry] = category
			}
		}
	}
	if sections["Unreleased"] == nil {
		return nil, errors.New("CHANGELOG.md requires exactly one [Unreleased] section")
	}
	for name, section := range sections {
		if name != "Unreleased" && len(section.entries) == 0 {
			return nil, fmt.Errorf("version notes for [%s] require at least one nonempty entry", name)
		}
	}
	return sections, nil
}

func auditChangelog(contents, value string, release, candidate bool) (string, string, error) {
	sections, err := parseChangelog(contents)
	if err != nil {
		return "", "", err
	}
	section := sections[value]
	if !release && !candidate {
		return "", "", nil
	}
	if section == nil || section.date == "" {
		return "", "", errors.New("CHANGELOG.md requires dated release notes for VERSION")
	}
	if candidate {
		return section.date, "", nil
	}
	return section.date, "# FlashGate MCP " + value + "\n\n" + strings.TrimSpace(strings.Join(section.lines, "\n")) + "\n", nil
}

func auditFreeze(git func(...string) ([]byte, error), value, changelog string, tagged bool) error {
	prefix, err := git("rev-parse", "--show-prefix")
	if err != nil || len(bytes.TrimRight(prefix, "\r\n")) != 0 {
		return errors.New("release freeze requires the Git worktree root")
	}
	status, err := git("status", "--porcelain=v1", "--untracked-files=all")
	if err != nil || len(status) != 0 {
		return errors.New("release freeze requires a readable, clean working tree")
	}
	headVersion, err := git("show", "HEAD:VERSION")
	if err != nil {
		return errors.New("release freeze requires VERSION at HEAD")
	}
	committedVersion, err := parseRepositoryVersion(headVersion)
	if err != nil || committedVersion != value {
		return errors.New("release VERSION differs from HEAD")
	}
	headChangelog, err := git("show", "HEAD:CHANGELOG.md")
	if err != nil || string(headChangelog) != changelog {
		return errors.New("release CHANGELOG differs from HEAD")
	}
	tag := "refs/tags/v" + value
	if tagged {
		head, headErr := git("rev-parse", "HEAD")
		commit, tagErr := git("rev-parse", "--verify", tag+"^{commit}")
		if headErr != nil || tagErr != nil || !commitPattern.Match(bytes.TrimSpace(head)) || !bytes.Equal(head, commit) {
			return errors.New("release tag must identify the exact freeze commit")
		}
	} else {
		tags, err := git("for-each-ref", "--format=%(refname)", tag)
		if err != nil || len(bytes.TrimSpace(tags)) != 0 {
			return errors.New("candidate requires a readable tag namespace and an unused VERSION")
		}
	}
	parentVersion, err := git("show", "HEAD^1:VERSION")
	if err != nil {
		return errors.New("candidate requires VERSION from the exact first parent")
	}
	previous, err := parseRepositoryVersion(parentVersion)
	if err != nil {
		return err
	}
	patch, err := freezeIncrement(previous, value)
	if err != nil {
		return err
	}
	parent, err := gitChangelog(git, "HEAD^1")
	if err != nil {
		return err
	}
	if parent[value] != nil {
		return errors.New("candidate notes must be introduced together with VERSION")
	}
	if !patch {
		return nil
	}
	priorTag := "refs/tags/v" + previous
	if err := gitCheck(git, "PATCH freeze must descend from its exact published release tag", "merge-base", "--is-ancestor", priorTag+"^{commit}", "HEAD^1"); err != nil {
		return err
	}
	tagVersion, err := git("show", priorTag+":VERSION")
	if err != nil {
		return errors.New("PATCH base tag must have a readable VERSION")
	}
	publishedVersion, err := parseRepositoryVersion(tagVersion)
	if err != nil || publishedVersion != previous {
		return errors.New("PATCH base tag and VERSION do not match")
	}
	published, err := gitChangelog(git, priorTag)
	if err != nil {
		return err
	}
	if published[previous] == nil || published[previous].date == "" {
		return errors.New("PATCH base tag must have dated release notes")
	}
	frozen, err := parseChangelog(changelog)
	if err != nil {
		return err
	}
	return auditPatchScope(published["Unreleased"], parent["Unreleased"], frozen["Unreleased"], frozen[value])
}

func gitCheck(git func(...string) ([]byte, error), message string, args ...string) error {
	if _, err := git(args...); err != nil {
		return errors.New(message)
	}
	return nil
}

func gitChangelog(git func(...string) ([]byte, error), ref string) (map[string]*changelogSection, error) {
	data, err := git("show", ref+":CHANGELOG.md")
	if err != nil {
		return nil, errors.New("candidate requires readable CHANGELOG from " + ref)
	}
	return parseChangelog(string(data))
}

func freezeIncrement(previous, current string) (patch bool, err error) {
	var oldMajor, oldMinor, oldPatch, major, minor, patchNumber int
	oldCore, err := version.WindowsFileVersion(previous)
	if err != nil {
		return false, err
	}
	core, err := version.WindowsFileVersion(current)
	if err != nil {
		return false, err
	}
	_, _ = fmt.Sscanf(oldCore, "%d.%d.%d", &oldMajor, &oldMinor, &oldPatch)
	_, _ = fmt.Sscanf(core, "%d.%d.%d", &major, &minor, &patchNumber)
	switch {
	case major == oldMajor && minor == oldMinor && patchNumber == oldPatch+1:
		return true, nil
	case major == oldMajor && minor == oldMinor+1 && patchNumber == 0:
		return false, nil
	case major == oldMajor+1 && minor == 0 && patchNumber == 0:
		return false, nil
	case core == oldCore && advancesPrerelease(previous, current):
		return patchNumber != 0, nil
	default:
		return false, errors.New("release freeze requires a new SemVer release increment")
	}
}

// Both versions have already passed SemVer validation.
func advancesPrerelease(previous, current string) bool {
	previous, _, _ = strings.Cut(previous, "+")
	current, _, _ = strings.Cut(current, "+")
	_, oldPrerelease, _ := strings.Cut(previous, "-")
	_, newPrerelease, _ := strings.Cut(current, "-")
	if oldPrerelease == "" || newPrerelease == "" {
		return oldPrerelease != "" && newPrerelease == ""
	}

	oldIDs := strings.Split(oldPrerelease, ".")
	newIDs := strings.Split(newPrerelease, ".")
	for i := range min(len(oldIDs), len(newIDs)) {
		oldID, newID := oldIDs[i], newIDs[i]
		if oldID == newID {
			continue
		}
		oldNumeric := strings.Trim(oldID, "0123456789") == ""
		newNumeric := strings.Trim(newID, "0123456789") == ""
		if oldNumeric != newNumeric {
			return oldNumeric
		}
		if oldNumeric && len(oldID) != len(newID) {
			return len(oldID) < len(newID)
		}
		return oldID < newID
	}
	return len(oldIDs) < len(newIDs)
}

func auditPatchScope(published, parent, pending, release *changelogSection) error {
	isFeature := func(category string) bool {
		return category == "added" || category == "breaking" || category == "breaking changes"
	}
	for _, section := range []*changelogSection{parent, pending} {
		for entry, category := range section.entries {
			if isFeature(category) && published.entries[entry] != category {
				return errors.New("PATCH freeze contains pending feature or breaking notes; use an isolated maintenance line")
			}
		}
	}
	for _, category := range release.entries {
		if isFeature(category) {
			return errors.New("PATCH release notes contain features or breaking changes")
		}
	}
	return nil
}
