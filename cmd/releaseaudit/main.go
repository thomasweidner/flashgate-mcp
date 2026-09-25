package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
)

const maximumEntrySize = 64 << 20

type archiveEntry struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Length int64  `json:"length"`
	SHA256 string `json:"sha256,omitempty"`
	data   []byte
}

type inventoryReport struct {
	Schema   string         `json:"schema"`
	Status   string         `json:"status"`
	Artifact string         `json:"artifact"`
	Entries  []archiveEntry `json:"entries"`
	Errors   []string       `json:"errors"`
}

type comparisonReport struct {
	Schema         string   `json:"schema"`
	Status         string   `json:"status"`
	ArchiveSHA256  string   `json:"archiveSha256"`
	ChecksumSHA256 string   `json:"checksumSha256"`
	BinarySHA256   string   `json:"binarySha256"`
	InventoryCount int      `json:"inventoryCount"`
	Errors         []string `json:"errors"`
}

type leakFinding struct {
	Rule   string `json:"rule"`
	Source string `json:"source"`
}

type allowedMarker struct {
	Marker string `json:"marker"`
	Count  int    `json:"count"`
}

type leakReport struct {
	Schema         string          `json:"schema"`
	Status         string          `json:"status"`
	Artifact       string          `json:"artifact"`
	ScannedEntries int             `json:"scannedEntries"`
	Findings       []leakFinding   `json:"findings"`
	AllowedMarkers []allowedMarker `json:"allowedMarkers"`
	Errors         []string        `json:"errors"`
}

type stringList []string

func (values *stringList) String() string {
	return strings.Join(*values, ",")
}

func (values *stringList) Set(value string) error {
	if value == "" {
		return errors.New("forbidden value must not be empty")
	}
	*values = append(*values, value)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: releaseaudit inventory|compare|scan|scan-file|source")
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "inventory":
		err = runInventory(os.Args[2:])
	case "compare":
		err = runCompare(os.Args[2:])
	case "scan":
		err = runScan(os.Args[2:])
	case "scan-file":
		err = runScanFile(os.Args[2:])
	case "source":
		err = runSource(os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "releaseaudit: %v\n", err)
		os.Exit(1)
	}
}

func runScanFile(arguments []string) error {
	flags := flag.NewFlagSet("scan-file", flag.ContinueOnError)
	filePath := flags.String("file", "", "binary or other file")
	reportPath := flags.String("report", "", "JSON report path")
	var forbidden stringList
	flags.Var(&forbidden, "forbidden", "host-specific value to reject")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *filePath == "" || *reportPath == "" {
		return errors.New("scan-file requires --file and --report")
	}

	report := leakReport{
		Schema:         "flashgate-release-leak-scan/v1",
		Status:         "PASS",
		Artifact:       filepath.Base(*filePath),
		ScannedEntries: 1,
		Findings:       []leakFinding{},
		AllowedMarkers: []allowedMarker{},
		Errors:         []string{},
	}
	data, err := os.ReadFile(*filePath)
	if err != nil {
		report.Status = "FAIL"
		report.Errors = append(report.Errors, err.Error())
	} else {
		scanBytes(filepath.Base(*filePath), data, forbidden, &report)
	}
	if len(report.Findings) > 0 || len(report.Errors) > 0 {
		report.Status = "FAIL"
	}
	if err := writeJSON(*reportPath, report); err != nil {
		return err
	}
	if report.Status != "PASS" {
		return errors.New("file leak scan failed")
	}
	return nil
}

func runInventory(arguments []string) error {
	flags := flag.NewFlagSet("inventory", flag.ContinueOnError)
	artifact := flags.String("artifact", "", "release archive")
	reportPath := flags.String("report", "", "JSON report path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *artifact == "" || *reportPath == "" {
		return errors.New("inventory requires --artifact and --report")
	}

	entries, err := readArchive(*artifact)
	report := inventoryReport{
		Schema:   "flashgate-release-inventory/v1",
		Status:   "PASS",
		Artifact: filepath.Base(*artifact),
		Entries:  entries,
		Errors:   []string{},
	}
	if err != nil {
		report.Status = "FAIL"
		report.Errors = []string{err.Error()}
	}
	if writeErr := writeJSON(*reportPath, report); writeErr != nil {
		return writeErr
	}
	return err
}

func runCompare(arguments []string) error {
	flags := flag.NewFlagSet("compare", flag.ContinueOnError)
	artifactA := flags.String("artifact-a", "", "first release archive")
	checksumA := flags.String("checksum-a", "", "first checksum file")
	artifactB := flags.String("artifact-b", "", "second release archive")
	checksumB := flags.String("checksum-b", "", "second checksum file")
	binarySuffix := flags.String("binary-suffix", "", "expected binary path suffix")
	reportPath := flags.String("report", "", "JSON report path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"artifact-a":    *artifactA,
		"checksum-a":    *checksumA,
		"artifact-b":    *artifactB,
		"checksum-b":    *checksumB,
		"binary-suffix": *binarySuffix,
		"report":        *reportPath,
	} {
		if value == "" {
			return fmt.Errorf("compare requires --%s", name)
		}
	}

	report := comparisonReport{
		Schema: "flashgate-release-reproducibility/v1",
		Status: "PASS",
		Errors: []string{},
	}
	var comparisonErrors []string

	archiveHashA, err := hashFile(*artifactA)
	if err != nil {
		comparisonErrors = append(comparisonErrors, err.Error())
	}
	archiveHashB, err := hashFile(*artifactB)
	if err != nil {
		comparisonErrors = append(comparisonErrors, err.Error())
	}
	if archiveHashA != archiveHashB {
		comparisonErrors = append(comparisonErrors, "archive hashes differ")
	}
	report.ArchiveSHA256 = archiveHashA

	checksumHashA, err := hashFile(*checksumA)
	if err != nil {
		comparisonErrors = append(comparisonErrors, err.Error())
	}
	checksumHashB, err := hashFile(*checksumB)
	if err != nil {
		comparisonErrors = append(comparisonErrors, err.Error())
	}
	if checksumHashA != checksumHashB {
		comparisonErrors = append(comparisonErrors, "checksum-file hashes differ")
	}
	report.ChecksumSHA256 = checksumHashA

	entriesA, errA := readArchive(*artifactA)
	entriesB, errB := readArchive(*artifactB)
	if errA != nil {
		comparisonErrors = append(comparisonErrors, errA.Error())
	}
	if errB != nil {
		comparisonErrors = append(comparisonErrors, errB.Error())
	}
	if errA == nil && errB == nil {
		report.InventoryCount = len(entriesA)
		if inventoryIdentity(entriesA) != inventoryIdentity(entriesB) {
			comparisonErrors = append(comparisonErrors, "archive inventories differ")
		}
		binaryHashes := make([]string, 0, 2)
		for _, entries := range [][]archiveEntry{entriesA, entriesB} {
			matches := make([]string, 0, 1)
			for _, entry := range entries {
				if entry.Type == "file" && strings.HasSuffix(entry.Path, *binarySuffix) {
					matches = append(matches, entry.SHA256)
				}
			}
			if len(matches) != 1 {
				comparisonErrors = append(
					comparisonErrors,
					"expected exactly one binary entry per archive",
				)
			} else {
				binaryHashes = append(binaryHashes, matches[0])
			}
		}
		if len(binaryHashes) == 2 {
			report.BinarySHA256 = binaryHashes[0]
			if binaryHashes[0] != binaryHashes[1] {
				comparisonErrors = append(comparisonErrors, "binary hashes differ")
			}
		}
	}

	if len(comparisonErrors) > 0 {
		report.Status = "FAIL"
		report.Errors = comparisonErrors
	}
	if err := writeJSON(*reportPath, report); err != nil {
		return err
	}
	if report.Status != "PASS" {
		return errors.New("release reproducibility comparison failed")
	}
	return nil
}

func runScan(arguments []string) error {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	artifact := flags.String("artifact", "", "release archive")
	checksum := flags.String("checksum", "", "checksum file")
	reportPath := flags.String("report", "", "JSON report path")
	var forbidden stringList
	flags.Var(&forbidden, "forbidden", "host-specific value to reject")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *artifact == "" || *checksum == "" || *reportPath == "" {
		return errors.New("scan requires --artifact, --checksum, and --report")
	}

	report := leakReport{
		Schema:         "flashgate-release-leak-scan/v1",
		Status:         "PASS",
		Artifact:       filepath.Base(*artifact),
		Findings:       []leakFinding{},
		AllowedMarkers: []allowedMarker{},
		Errors:         []string{},
	}
	entries, err := readArchive(*artifact)
	if err != nil {
		report.Status = "FAIL"
		report.Errors = append(report.Errors, err.Error())
	} else {
		report.ScannedEntries = len(entries)
		for _, entry := range entries {
			if entry.Type != "file" {
				continue
			}
			scanBytes(entry.Path, entry.data, forbidden, &report)
		}
		checksumBytes, readErr := os.ReadFile(*checksum)
		if readErr != nil {
			report.Status = "FAIL"
			report.Errors = append(report.Errors, readErr.Error())
		} else {
			scanBytes(filepath.Base(*checksum), checksumBytes, forbidden, &report)
		}
	}

	if len(report.Findings) > 0 || len(report.Errors) > 0 {
		report.Status = "FAIL"
	}
	if err := writeJSON(*reportPath, report); err != nil {
		return err
	}
	if report.Status != "PASS" {
		return errors.New("release leak scan failed")
	}
	return nil
}

func normalizeArchivePath(value string) (string, error) {
	if value == "" || strings.ContainsRune(value, '\x00') ||
		strings.Contains(value, `\`) || strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("unsafe archive path %q", value)
	}
	trimmed := strings.TrimSuffix(value, "/")
	if trimmed == "" {
		return "", fmt.Errorf("empty normalized archive path %q", value)
	}
	cleaned := path.Clean(trimmed)
	if cleaned != trimmed || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("noncanonical archive path %q", value)
	}
	for _, component := range strings.Split(cleaned, "/") {
		if component == "" || component == "." || component == ".." {
			return "", fmt.Errorf("unsafe archive component in %q", value)
		}
	}
	return cleaned, nil
}

func readArchive(archivePath string) ([]archiveEntry, error) {
	switch {
	case strings.HasSuffix(archivePath, ".zip"):
		return readZIP(archivePath)
	case strings.HasSuffix(archivePath, ".tar.gz"):
		return readTarGZ(archivePath)
	default:
		return nil, fmt.Errorf("unsupported archive type: %s", archivePath)
	}
}

func readZIP(archivePath string) ([]archiveEntry, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()

	seen := make(map[string]struct{}, len(archive.File))
	entries := make([]archiveEntry, 0, len(archive.File))
	for _, file := range archive.File {
		name, err := normalizeArchivePath(file.Name)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate normalized ZIP path: %s", name)
		}
		seen[name] = struct{}{}

		mode := file.Mode()
		if mode&os.ModeSymlink != 0 || mode&(os.ModeDevice|os.ModeNamedPipe|os.ModeSocket) != 0 {
			return nil, fmt.Errorf("unsupported ZIP entry type: %s", name)
		}
		if strings.HasSuffix(file.Name, "/") || mode.IsDir() {
			if !strings.HasSuffix(file.Name, "/") || !mode.IsDir() {
				return nil, fmt.Errorf("inconsistent ZIP directory type: %s", name)
			}
			entries = append(entries, archiveEntry{Path: name, Type: "directory"})
			continue
		}
		if file.UncompressedSize64 > maximumEntrySize {
			return nil, fmt.Errorf("ZIP entry exceeds the audit limit: %s", name)
		}
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(reader, maximumEntrySize+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(data) > maximumEntrySize || uint64(len(data)) != file.UncompressedSize64 {
			return nil, fmt.Errorf("ZIP entry length mismatch: %s", name)
		}
		entries = append(entries, fileEntry(name, data))
	}
	sortEntries(entries)
	return entries, nil
}

func readTarGZ(archivePath string) ([]archiveEntry, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	seen := make(map[string]struct{})
	var entries []archiveEntry
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		name, err := normalizeArchivePath(header.Name)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate normalized TAR path: %s", name)
		}
		seen[name] = struct{}{}
		switch {
		case header.Typeflag == tar.TypeDir:
			if header.Size != 0 {
				return nil, fmt.Errorf("TAR directory has data: %s", name)
			}
			entries = append(entries, archiveEntry{Path: name, Type: "directory"})
		case header.Typeflag == tar.TypeReg || header.Typeflag == 0:
			if header.Size < 0 || header.Size > maximumEntrySize {
				return nil, fmt.Errorf("TAR entry exceeds the audit limit: %s", name)
			}
			data, err := io.ReadAll(io.LimitReader(reader, maximumEntrySize+1))
			if err != nil {
				return nil, err
			}
			if int64(len(data)) != header.Size {
				return nil, fmt.Errorf("TAR entry length mismatch: %s", name)
			}
			entries = append(entries, fileEntry(name, data))
		default:
			return nil, fmt.Errorf("unsupported TAR entry type for %s", name)
		}
	}
	sortEntries(entries)
	return entries, nil
}

func fileEntry(name string, data []byte) archiveEntry {
	digest := sha256.Sum256(data)
	return archiveEntry{
		Path:   name,
		Type:   "file",
		Length: int64(len(data)),
		SHA256: hex.EncodeToString(digest[:]),
		data:   data,
	}
}

func sortEntries(entries []archiveEntry) {
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].Path < entries[right].Path
	})
}

func inventoryIdentity(entries []archiveEntry) string {
	var builder strings.Builder
	for _, entry := range entries {
		fmt.Fprintf(
			&builder,
			"%s\t%s\t%d\t%s\n",
			entry.Path,
			entry.Type,
			entry.Length,
			entry.SHA256,
		)
	}
	return builder.String()
}

func hashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func scanBytes(
	source string,
	data []byte,
	forbidden []string,
	report *leakReport,
) {
	texts := []string{
		string(data),
		printableASCII(data),
		printableUTF16LE(data),
	}
	rules := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{
			"windows-user-or-temporary-path",
			regexp.MustCompile(`(?i)[A-Z]:\\(?:Users\\[^\\\s]+|Windows\\Temp|Temp)\\[^\x00\r\n]{1,240}`),
		},
		{
			"linux-home-or-temporary-path",
			regexp.MustCompile(`(?:/home/[^/\s\x00]+|/tmp/[^\s\x00]{1,240})`),
		},
		{
			"github-token",
			regexp.MustCompile(`(?:gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,})`),
		},
		{
			"aws-access-key",
			regexp.MustCompile(`(?:AKIA|ASIA)[0-9A-Z]{16}`),
		},
		{
			"credential-url",
			regexp.MustCompile(`(?i)https?://[^/@\s:]+:[^/@\s]+@`),
		},
		{
			"private-key-block",
			regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----[\r\n]+[A-Za-z0-9+/=\r\n]{32,}-----END`),
		},
	}

	seen := make(map[string]struct{})
	addFinding := func(rule string) {
		key := rule + "\x00" + source
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		report.Findings = append(report.Findings, leakFinding{Rule: rule, Source: source})
	}

	for _, text := range texts {
		for _, rule := range rules {
			if rule.pattern.MatchString(text) {
				addFinding(rule.name)
			}
		}
		for _, value := range forbidden {
			if len(value) >= 3 && strings.Contains(text, value) {
				addFinding("forbidden-host-value")
			}
		}
	}

	for _, marker := range []string{
		"-----BEGIN PRIVATE KEY-----",
		"-----BEGIN RSA PRIVATE KEY-----",
	} {
		count := strings.Count(string(data), marker)
		if count > 0 {
			report.AllowedMarkers = append(
				report.AllowedMarkers,
				allowedMarker{Marker: marker, Count: count},
			)
		}
	}
}

func printableASCII(data []byte) string {
	var builder strings.Builder
	run := make([]byte, 0, 128)
	flush := func() {
		if len(run) >= 6 {
			builder.Write(run)
			builder.WriteByte('\n')
		}
		run = run[:0]
	}
	for _, value := range data {
		if value >= 0x20 && value <= 0x7e {
			run = append(run, value)
		} else {
			flush()
		}
	}
	flush()
	return builder.String()
}

func printableUTF16LE(data []byte) string {
	var builder strings.Builder
	var run []uint16
	flush := func() {
		if len(run) >= 6 {
			builder.WriteString(string(utf16.Decode(run)))
			builder.WriteByte('\n')
		}
		run = run[:0]
	}
	for index := 0; index+1 < len(data); index += 2 {
		value := uint16(data[index]) | uint16(data[index+1])<<8
		if value >= 0x20 && value <= 0x7e {
			run = append(run, value)
		} else {
			flush()
		}
	}
	flush()
	return builder.String()
}

func writeJSON(reportPath string, value any) error {
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(reportPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(value)
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}
