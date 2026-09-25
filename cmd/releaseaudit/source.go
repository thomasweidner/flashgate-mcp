package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thomasweidner/flashgate-mcp/internal/version"
)

const maximumChangelogSize = 2 << 20

var releaseHeading = regexp.MustCompile(`^## \[([^]]+)\] - ([0-9]{4}-[0-9]{2}-[0-9]{2})$`)

type sourceReport struct {
	Schema       string   `json:"schema"`
	Status       string   `json:"status"`
	Version      string   `json:"version"`
	ExpectedTag  string   `json:"expectedTag"`
	Release      bool     `json:"release"`
	ReleaseDate  string   `json:"releaseDate,omitempty"`
	ReleaseNotes string   `json:"releaseNotes,omitempty"`
	Errors       []string `json:"errors"`
}

func runSource(arguments []string) error {
	flags := flag.NewFlagSet("source", flag.ContinueOnError)
	root := flags.String("root", "", "repository root")
	release := flags.Bool("release", false, "require release notes for VERSION")
	reportPath := flags.String("report", "", "optional JSON report path")
	notesPath := flags.String("notes-output", "", "optional generated release notes path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *root == "" || flags.NArg() != 0 {
		return errors.New("source requires --root and no positional arguments")
	}
	if *notesPath != "" && !*release {
		return errors.New("--notes-output requires --release")
	}
	report := sourceReport{
		Schema: "flashgate-release-source/v1", Status: "PASS",
		Release: *release, Errors: []string{},
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
	} else if len(changelog) == 0 || len(changelog) > maximumChangelogSize ||
		!utf8.Valid(changelog) || bytes.HasPrefix(changelog, []byte{0xef, 0xbb, 0xbf}) {
		report.Errors = append(report.Errors, "CHANGELOG.md has invalid encoding or size")
	} else {
		date, notes, auditErr := auditChangelog(string(changelog), value, *release)
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
	if !utf8.Valid(data) || bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return "", errors.New("VERSION must be UTF-8 without BOM")
	}
	value := strings.TrimSuffix(string(data), "\n")
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", errors.New("VERSION must contain one SemVer line with optional LF")
	}
	if _, err := version.WindowsFileVersion(value); err != nil {
		return "", fmt.Errorf("VERSION: %w", err)
	}
	return value, nil
}

func auditChangelog(contents, value string, release bool) (string, string, error) {
	lines := strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n")
	unreleasedCount := 0
	versionCount := 0
	date := ""
	var notes []string
	inRelease := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			inRelease = false
			if line == "## [Unreleased]" {
				unreleasedCount++
			}
			if strings.HasPrefix(line, "## ["+value+"]") {
				versionCount++
				match := releaseHeading.FindStringSubmatch(line)
				if match == nil || match[1] != value {
					return "", "", errors.New("release heading must be ## [VERSION] - YYYY-MM-DD")
				}
				if _, err := time.Parse("2006-01-02", match[2]); err != nil {
					return "", "", errors.New("release heading has invalid date")
				}
				date = match[2]
				inRelease = true
			}
			continue
		}
		if inRelease {
			notes = append(notes, line)
		}
	}
	if unreleasedCount != 1 {
		return "", "", errors.New("CHANGELOG.md requires exactly one [Unreleased] section")
	}
	if release && versionCount != 1 {
		return "", "", errors.New("CHANGELOG.md requires exactly one release section for VERSION")
	}
	if versionCount > 1 {
		return "", "", errors.New("CHANGELOG.md has duplicate release sections for VERSION")
	}
	if !release {
		return date, "", nil
	}
	hasEntry := false
	for _, line := range notes {
		if strings.HasPrefix(line, "- ") && strings.TrimSpace(strings.TrimPrefix(line, "- ")) != "" {
			hasEntry = true
		}
	}
	if !hasEntry {
		return "", "", errors.New("release notes require at least one nonempty entry")
	}
	return date, "# FlashGate MCP " + value + "\n\n" + strings.TrimSpace(strings.Join(notes, "\n")) + "\n", nil
}
