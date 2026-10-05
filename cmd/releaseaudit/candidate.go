package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/version"
)

const candidateSchema = "flashgate-current-candidate/v1"
const recordSchema = "flashgate-current-candidate-record/v1"

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type candidateTarget struct {
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	Extension    string `json:"extension"`
}

var candidateTargets = []candidateTarget{
	{"windows", "x64", "zip"},
	{"windows", "arm64", "zip"},
	{"linux", "x64", "tar.gz"},
	{"linux", "arm64", "tar.gz"},
}

type candidateRecord struct {
	Schema               string `json:"schema"`
	Version              string `json:"version"`
	SourceCommitSHA      string `json:"sourceCommitSha"`
	Platform             string `json:"platform"`
	Architecture         string `json:"architecture"`
	Archive              string `json:"archive"`
	Checksum             string `json:"checksum"`
	ArtifactSHA256       string `json:"artifactSha256"`
	ChecksumSHA256       string `json:"checksumSha256"`
	VerificationEvidence string `json:"verificationEvidence"`
	VerificationSHA256   string `json:"verificationSha256"`
	CompareReport        string `json:"compareReport"`
	CompareSHA256        string `json:"compareSha256"`
	LeakReport           string `json:"leakReport"`
	LeakSHA256           string `json:"leakSha256"`
}

type candidateManifest struct {
	Schema          string            `json:"schema"`
	Version         string            `json:"version"`
	SourceCommitSHA string            `json:"sourceCommitSha"`
	Records         []candidateRecord `json:"records"`
}

func targetFor(platform, architecture string) (candidateTarget, error) {
	for _, target := range candidateTargets {
		if target.Platform == platform && target.Architecture == architecture {
			return target, nil
		}
	}
	return candidateTarget{}, fmt.Errorf("unknown candidate target %s/%s", platform, architecture)
}

func candidateNames(value string, target candidateTarget) (string, string, string, string, string) {
	base := fmt.Sprintf("flashgate-mcp_%s_%s_%s.%s", value, target.Platform, target.Architecture, target.Extension)
	suffix := target.Platform + "-" + target.Architecture
	return base, base + ".sha256", "verify-" + suffix + ".txt", "repro-" + suffix + ".json", "leak-" + suffix + ".json"
}

func validateCandidateIdentity(value, commit string) error {
	if _, err := version.WindowsFileVersion(value); err != nil {
		return fmt.Errorf("invalid candidate VERSION: %w", err)
	}
	if !commitPattern.MatchString(commit) {
		return errors.New("source commit must be a full lowercase SHA-1")
	}
	return nil
}

func strictCandidateJSON(name string, value any) error {
	info, err := os.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("candidate JSON must be a regular file")
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	return strictCandidateBytes(data, value)
}

func strictCandidateBytes(data []byte, value any) error {
	if len(data) == 0 || len(data) > 1<<20 {
		return errors.New("candidate JSON size invalid")
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("candidate JSON has trailing content")
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok || seen[key] {
					return errors.New("candidate JSON duplicate or invalid key")
				}
				seen[key] = true
				if err := walk(); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
		default:
			return errors.New("candidate JSON delimiter invalid")
		}
		_, err = decoder.Token()
		return err
	}
	if err := walk(); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("candidate JSON has trailing content")
	}
	return nil
}

func candidateRegularFile(dir, name string) (string, error) {
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\\`) || name == "." || name == ".." {
		return "", fmt.Errorf("unsafe candidate filename %q", name)
	}
	path := filepath.Join(dir, name)
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("candidate file is not regular: %s", name)
	}
	return path, nil
}

func candidateHash(dir, name, expected string) error {
	if !hashPattern.MatchString(expected) {
		return fmt.Errorf("invalid SHA-256 for %s", name)
	}
	path, err := candidateRegularFile(dir, name)
	if err != nil {
		return err
	}
	actual, err := hashFile(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("SHA-256 mismatch for %s", name)
	}
	return nil
}

func validateVerificationEvidence(contents []byte, record candidateRecord) error {
	text := strings.ReplaceAll(string(contents), "\r\n", "\n")
	fields := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			fields[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	if fields["Status"] != "PASS" || fields["Version"] != record.Version || fields["PublicArch"] != record.Architecture ||
		fields["SourceCommit"] != record.SourceCommitSHA ||
		filepath.Base(fields["ArchivePath"]) != record.Archive || filepath.Base(fields["ChecksumPath"]) != record.Checksum ||
		fields["ErrorCount"] != "0" {
		return errors.New("BL-248 verifier evidence does not bind a passing target")
	}
	if value := fields["Sha256"]; value != "" && !strings.EqualFold(value, record.ArtifactSHA256) {
		return errors.New("BL-248 verifier SHA-256 differs")
	}
	return nil
}

func verifyCandidateRecord(dir string, record candidateRecord) error {
	if record.Schema != recordSchema {
		return errors.New("candidate record schema mismatch")
	}
	if err := validateCandidateIdentity(record.Version, record.SourceCommitSHA); err != nil {
		return err
	}
	target, err := targetFor(record.Platform, record.Architecture)
	if err != nil {
		return err
	}
	archive, checksum, evidence, compare, leak := candidateNames(record.Version, target)
	if record.Archive != archive || record.Checksum != checksum || record.VerificationEvidence != evidence ||
		record.CompareReport != compare || record.LeakReport != leak {
		return errors.New("candidate record has noncanonical filenames")
	}
	for _, item := range []struct{ name, hash string }{
		{archive, record.ArtifactSHA256}, {checksum, record.ChecksumSHA256},
		{evidence, record.VerificationSHA256}, {compare, record.CompareSHA256}, {leak, record.LeakSHA256},
	} {
		if err := candidateHash(dir, item.name, item.hash); err != nil {
			return err
		}
	}
	checksumBytes, err := os.ReadFile(filepath.Join(dir, checksum))
	if err != nil {
		return err
	}
	if string(checksumBytes) != record.ArtifactSHA256+"  "+archive+"\n" {
		return errors.New("candidate checksum content mismatch")
	}
	evidenceBytes, err := os.ReadFile(filepath.Join(dir, evidence))
	if err != nil {
		return err
	}
	if err := validateVerificationEvidence(evidenceBytes, record); err != nil {
		return err
	}
	var repro comparisonReport
	if err := strictCandidateJSON(filepath.Join(dir, compare), &repro); err != nil {
		return err
	}
	if repro.Schema != "flashgate-release-reproducibility/v1" || repro.Status != "PASS" ||
		repro.ArchiveSHA256 != record.ArtifactSHA256 || repro.ChecksumSHA256 != record.ChecksumSHA256 ||
		repro.BinarySHA256 == "" || repro.InventoryCount < 1 || len(repro.Errors) != 0 {
		return errors.New("BL-248 reproducibility evidence mismatch")
	}
	var scan leakReport
	if err := strictCandidateJSON(filepath.Join(dir, leak), &scan); err != nil {
		return err
	}
	if scan.Schema != "flashgate-release-leak-scan/v1" || scan.Status != "PASS" || scan.Artifact != archive ||
		scan.ScannedEntries < 1 || len(scan.Errors) != 0 || len(scan.Findings) != 0 {
		return errors.New("BL-248 leak evidence mismatch")
	}
	return nil
}

func runCandidateRecord(arguments []string) error {
	flags := flag.NewFlagSet("candidate-record", flag.ContinueOnError)
	root := flags.String("root", "", "candidate source repository root")
	dir := flags.String("dir", "", "directory containing verified candidate files")
	value := flags.String("version", "", "root VERSION")
	commit := flags.String("commit", "", "full source commit")
	platform := flags.String("platform", "", "target platform")
	architecture := flags.String("architecture", "", "public architecture")
	output := flags.String("output", "", "new candidate record path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *root == "" || *dir == "" || *output == "" || flags.NArg() != 0 {
		return errors.New("candidate-record requires --root, --dir, --output, and no positional arguments")
	}
	if err := validateCandidateIdentity(*value, *commit); err != nil {
		return err
	}
	canonical, err := readRepositoryVersion(filepath.Join(*root, "VERSION"))
	if err != nil {
		return err
	}
	if canonical != *value {
		return errors.New("candidate VERSION differs from repository root")
	}
	target, err := targetFor(*platform, *architecture)
	if err != nil {
		return err
	}
	archive, checksum, evidence, compare, leak := candidateNames(*value, target)
	record := candidateRecord{Schema: recordSchema, Version: *value, SourceCommitSHA: *commit, Platform: *platform,
		Architecture: *architecture, Archive: archive, Checksum: checksum, VerificationEvidence: evidence, CompareReport: compare, LeakReport: leak}
	for _, item := range []struct {
		name string
		set  func(string)
	}{
		{archive, func(s string) { record.ArtifactSHA256 = s }}, {checksum, func(s string) { record.ChecksumSHA256 = s }},
		{evidence, func(s string) { record.VerificationSHA256 = s }}, {compare, func(s string) { record.CompareSHA256 = s }},
		{leak, func(s string) { record.LeakSHA256 = s }},
	} {
		path, err := candidateRegularFile(*dir, item.name)
		if err != nil {
			return err
		}
		hash, err := hashFile(path)
		if err != nil {
			return err
		}
		item.set(hash)
	}
	if err := verifyCandidateRecord(*dir, record); err != nil {
		return err
	}
	return writeJSON(*output, record)
}

func verifyCandidateManifest(dir string, manifest candidateManifest) error {
	if manifest.Schema != candidateSchema {
		return errors.New("candidate manifest schema mismatch")
	}
	if err := validateCandidateIdentity(manifest.Version, manifest.SourceCommitSHA); err != nil {
		return err
	}
	if len(manifest.Records) != len(candidateTargets) {
		return errors.New("candidate matrix must have exactly four targets")
	}
	for index, expected := range candidateTargets {
		record := manifest.Records[index]
		if record.Platform != expected.Platform || record.Architecture != expected.Architecture ||
			record.Version != manifest.Version || record.SourceCommitSHA != manifest.SourceCommitSHA {
			return fmt.Errorf("candidate matrix identity/order mismatch at %d", index)
		}
		if err := verifyCandidateRecord(dir, record); err != nil {
			return err
		}
		recordName := "record-" + expected.Platform + "-" + expected.Architecture + ".json"
		var stored candidateRecord
		if err := strictCandidateJSON(filepath.Join(dir, recordName), &stored); err != nil {
			return err
		}
		if !reflect.DeepEqual(stored, record) {
			return errors.New("candidate record file differs from manifest")
		}
	}
	return nil
}

func runCandidateManifest(arguments []string) error {
	flags := flag.NewFlagSet("candidate-manifest", flag.ContinueOnError)
	root := flags.String("root", "", "candidate source repository root")
	dir := flags.String("dir", "", "directory containing four verified target records and files")
	value := flags.String("version", "", "root VERSION")
	commit := flags.String("commit", "", "full source commit")
	output := flags.String("output", "", "new manifest path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *root == "" || *dir == "" || *output == "" || flags.NArg() != 0 {
		return errors.New("candidate-manifest requires --root, --dir, --output, and no positional arguments")
	}
	canonical, err := readRepositoryVersion(filepath.Join(*root, "VERSION"))
	if err != nil {
		return err
	}
	if canonical != *value {
		return errors.New("candidate VERSION differs from repository root")
	}
	manifest := candidateManifest{Schema: candidateSchema, Version: *value, SourceCommitSHA: *commit, Records: make([]candidateRecord, 0, 4)}
	for _, target := range candidateTargets {
		name := "record-" + target.Platform + "-" + target.Architecture + ".json"
		var record candidateRecord
		if err := strictCandidateJSON(filepath.Join(*dir, name), &record); err != nil {
			return err
		}
		manifest.Records = append(manifest.Records, record)
	}
	if err := verifyCandidateManifest(*dir, manifest); err != nil {
		return err
	}
	return writeJSON(*output, manifest)
}

func runCandidatePromote(arguments []string) error {
	flags := flag.NewFlagSet("candidate-promote", flag.ContinueOnError)
	dir := flags.String("dir", "", "directory containing verified candidate files")
	manifestPath := flags.String("manifest", "", "strict candidate manifest")
	destination := flags.String("destination", "", "local promotion root")
	value := flags.String("version", "", "expected root VERSION")
	commit := flags.String("commit", "", "expected source commit")
	current := flags.Bool("current", false, "update mutable current alias after promotion")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *dir == "" || *manifestPath == "" || *destination == "" || *value == "" || *commit == "" || flags.NArg() != 0 {
		return errors.New("candidate-promote requires --dir, --manifest, --destination, --version, --commit")
	}
	var manifest candidateManifest
	if err := strictCandidateJSON(*manifestPath, &manifest); err != nil {
		return err
	}
	if manifest.Version != *value || manifest.SourceCommitSHA != *commit {
		return errors.New("candidate promotion identity mismatch")
	}
	if err := verifyCandidateManifest(*dir, manifest); err != nil {
		return err
	}
	return promoteCandidate(*dir, *destination, *manifestPath, manifest, *current)
}

func promotedNames(manifest candidateManifest) []string {
	names := []string{"candidate-manifest.json"}
	for _, record := range manifest.Records {
		names = append(names, record.Archive, record.Checksum, record.VerificationEvidence, record.CompareReport, record.LeakReport,
			"record-"+record.Platform+"-"+record.Architecture+".json")
	}
	return names
}

func sameCandidateFiles(sourceDir, targetDir, manifestPath string, manifest candidateManifest) error {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		return err
	}
	if len(entries) != len(promotedNames(manifest)) {
		return errors.New("immutable candidate target has unexpected file inventory")
	}
	for _, name := range promotedNames(manifest) {
		source := filepath.Join(sourceDir, name)
		if name == "candidate-manifest.json" {
			source = manifestPath
		}
		target, err := candidateRegularFile(targetDir, name)
		if err != nil {
			return err
		}
		sourceHash, err := hashFile(source)
		if err != nil {
			return err
		}
		targetHash, err := hashFile(target)
		if err != nil {
			return err
		}
		if sourceHash != targetHash {
			return fmt.Errorf("immutable candidate bytes differ: %s", name)
		}
	}
	return nil
}

func promoteCandidate(sourceDir, destination, manifestPath string, manifest candidateManifest, current bool) error {
	return promoteCandidateWithOps(sourceDir, destination, manifestPath, manifest, current, candidateAliasOps{os.Rename, os.Remove})
}

// These two operations are scoped to promotion so failure tests can exercise
// alias replacement and cleanup without changing process-wide filesystem state.
type candidateAliasOps struct {
	rename func(string, string) error
	remove func(string) error
}

func promoteCandidateWithOps(sourceDir, destination, manifestPath string, manifest candidateManifest, current bool, ops candidateAliasOps) (result error) {
	// The local destination is caller-controlled. Reject symlinked parents and existing
	// ambiguous state before creating any promoted file.
	root, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if err := rejectSymlinkParents(root); err != nil {
		return err
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var boundManifest candidateManifest
	if err := strictCandidateBytes(manifestBytes, &boundManifest); err != nil {
		return err
	}
	if !reflect.DeepEqual(boundManifest, manifest) {
		return errors.New("candidate manifest changed before promotion")
	}
	digest := sha256.Sum256(manifestBytes)
	identity := manifest.Version + "_" + manifest.SourceCommitSHA + "_" + hex.EncodeToString(digest[:])
	immutable := filepath.Join(root, identity)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(root, ".candidate-promotion.lock")
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("candidate promotion lock: %w", err)
	}
	if err := lock.Close(); err != nil {
		os.Remove(lockPath)
		return err
	}
	alias := filepath.Join(root, "current")
	var original []byte
	var originalMode os.FileMode
	aliasExists, aliasTouched, created := false, false, false
	var tempPath, backupPath string
	remove := func(path string) error {
		if path == "" {
			return nil
		}
		err := ops.remove(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer func() {
		// Retain original bytes through cleanup and release the lock last.
		result = errors.Join(result, remove(tempPath))
		if result == nil {
			result = remove(backupPath)
		}
		if result == nil {
			result = remove(lockPath)
		}
		if result != nil {
			restored := !aliasTouched
			if aliasTouched {
				if err := os.Remove(alias); err != nil && !errors.Is(err, os.ErrNotExist) {
					result = errors.Join(result, fmt.Errorf("current rollback removal: %w", err))
				} else if aliasExists {
					// The original bytes remain bound even if backup cleanup or
					// restoration fails; use a direct exclusive restore as fallback.
					if err := ops.rename(backupPath, alias); err != nil {
						file, restoreErr := os.OpenFile(alias, os.O_WRONLY|os.O_CREATE|os.O_EXCL, originalMode.Perm())
						if restoreErr == nil {
							_, writeErr := file.Write(original)
							restoreErr = errors.Join(writeErr, file.Close(), os.Chmod(alias, originalMode.Perm()))
						}
						result = errors.Join(result, fmt.Errorf("current backup restore: %w", err), restoreErr)
					}
				}
				if aliasExists {
					bytes, readErr := os.ReadFile(alias)
					info, statErr := os.Lstat(alias)
					restored = readErr == nil && statErr == nil && info.Mode().IsRegular() &&
						string(bytes) == string(original) && info.Mode().Perm() == originalMode.Perm()
				} else {
					_, err := os.Lstat(alias)
					restored = errors.Is(err, os.ErrNotExist)
				}
				if !restored {
					result = errors.Join(result, errors.New("current rollback parity is not established; original backup retained"))
				}
			}
			// Direct cleanup is a distinct recovery operation after a failed
			// update/cleanup operation, not a repeated promotion attempt.
			for _, path := range []string{tempPath, backupPath, lockPath} {
				if path == backupPath && !restored {
					continue
				}
				if path != "" {
					if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
						result = errors.Join(result, fmt.Errorf("promotion rollback cleanup: %w", err))
					}
				}
			}
			if created {
				result = rollbackNewCandidate(immutable, manifest, result)
			}
		}
	}()
	// Preflight under the destination-wide lock, before exposing any new identity.
	if current {
		if info, err := os.Lstat(alias); err == nil {
			if !info.Mode().IsRegular() {
				return errors.New("current alias is not a regular file")
			}
			aliasExists, originalMode = true, info.Mode()
			original, err = os.ReadFile(alias)
			if err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if info, err := os.Lstat(immutable); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("immutable candidate target is not a real directory")
		}
		if err := candidateHash(immutable, "candidate-manifest.json", hex.EncodeToString(digest[:])); err != nil {
			return err
		}
		if err := verifyCandidateManifest(immutable, manifest); err != nil {
			return err
		}
		if err := sameCandidateFiles(sourceDir, immutable, manifestPath, manifest); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	} else {
		stage, err := os.MkdirTemp(root, ".candidate-stage-")
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, os.RemoveAll(stage)) }()
		for _, name := range promotedNames(manifest) {
			source := filepath.Join(sourceDir, name)
			if name == "candidate-manifest.json" {
				source = manifestPath
			}
			if err := copyCandidateBytes(source, filepath.Join(stage, name)); err != nil {
				return err
			}
		}
		if err := candidateHash(stage, "candidate-manifest.json", hex.EncodeToString(digest[:])); err != nil {
			return err
		}
		if err := verifyCandidateManifest(stage, manifest); err != nil {
			return err
		}
		if err := sameCandidateFiles(sourceDir, stage, manifestPath, manifest); err != nil {
			return err
		}
		if err := os.Rename(stage, immutable); err != nil {
			return err
		}
		created = true
		if err := candidateHash(immutable, "candidate-manifest.json", hex.EncodeToString(digest[:])); err != nil {
			return err
		}
		if err := verifyCandidateManifest(immutable, manifest); err != nil {
			return err
		}
		if err := sameCandidateFiles(sourceDir, immutable, manifestPath, manifest); err != nil {
			return err
		}
	}
	if current {
		temp, err := os.CreateTemp(root, ".current-")
		if err != nil {
			return err
		}
		tempPath = temp.Name()
		if _, err := temp.WriteString(identity + "\n"); err != nil {
			temp.Close()
			return err
		}
		if err := temp.Close(); err != nil {
			return err
		}
		if aliasExists {
			backup, err := os.CreateTemp(root, ".current-backup-")
			if err != nil {
				return err
			}
			backupPath = backup.Name()
			if err := backup.Close(); err != nil {
				return err
			}
			if err := remove(backupPath); err != nil {
				return err
			}
			if err := ops.rename(alias, backupPath); err != nil {
				return err
			}
			aliasTouched = true
		}
		if err := ops.rename(tempPath, alias); err != nil {
			return err
		}
		aliasTouched = true
	}
	return nil
}

func rollbackNewCandidate(dir string, manifest candidateManifest, cause error) error {
	for _, name := range promotedNames(manifest) {
		path, err := candidateRegularFile(dir, name)
		if err != nil {
			return fmt.Errorf("promotion readback failed: %w; rollback blocked: %v", cause, err)
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("promotion readback failed: %w; rollback blocked: %v", cause, err)
		}
	}
	if err := os.Remove(dir); err != nil {
		return fmt.Errorf("promotion readback failed: %w; rollback blocked: %v", cause, err)
	}
	return cause
}

func copyCandidateBytes(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("source candidate is not a regular file")
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func rejectSymlinkParents(path string) error {
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("promotion path has a symbolic-link component")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
	}
}
