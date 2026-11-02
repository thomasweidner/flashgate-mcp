package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func securityOutputPath(output, evidence string) error {
	work := os.Getenv("FLASHGATE_WORK_ROOT")
	if work == "" || output == "" {
		return errors.New("security-assess requires FLASHGATE_WORK_ROOT and a new --output directory")
	}
	work, err := filepath.Abs(work)
	if err != nil {
		return errors.New("assessment work root invalid")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return errors.New("assessment output invalid")
	}
	info, err := os.Stat(work)
	if err != nil || !info.IsDir() {
		return errors.New("assessment work root unavailable")
	}
	rel, err := filepath.Rel(work, output)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("assessment output must be below task work root")
	}
	if rejectSymlinkParents(filepath.Dir(output)) != nil {
		return errors.New("assessment output parents must exist without links")
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		return errors.New("assessment output already exists or is inaccessible")
	}
	input, err := filepath.Abs(evidence)
	if err != nil {
		return errors.New("assessment evidence path invalid")
	}
	// An output may neither contaminate the verified bundle nor live in a
	// source checkout. Metadata names are inspected only, never mutated.
	for parent := filepath.Dir(output); ; parent = filepath.Dir(parent) {
		if strings.EqualFold(parent, input) {
			return errors.New("assessment output overlaps evidence")
		}
		metadata := filepath.Join(parent, ".git")
		info, probeErr := os.Lstat(metadata)
		if probeErr == nil {
			// Some hosts reserve an empty metadata directory. A real checkout
			// has HEAD, or a regular .git indirection for a linked worktree.
			if !info.IsDir() {
				return errors.New("assessment output must be outside source checkout")
			}
			if _, err := os.Lstat(filepath.Join(metadata, "HEAD")); !errors.Is(err, os.ErrNotExist) {
				return errors.New("assessment output must be outside source checkout")
			}
		} else if !errors.Is(probeErr, os.ErrNotExist) {
			return errors.New("assessment checkout boundary unavailable")
		}
		if parent == filepath.Dir(parent) {
			break
		}
	}
	return nil
}

func runSecurity(arguments []string) error {
	flags := flag.NewFlagSet("security-assess", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dir := flags.String("dir", "", "complete verified supply-chain bundle")
	version := flags.String("version", "", "externally trusted product version")
	commit := flags.String("commit", "", "externally trusted source commit")
	manifest := flags.String("manifest-sha256", "", "externally trusted candidate manifest SHA-256")
	index := flags.String("index-sha256", "", "externally trusted index SHA-256")
	snapshotPath := flags.String("snapshot", "", "offline OSV snapshot")
	online := flags.Bool("online", false, "explicitly enable read-only OSV requests; requires separate external authorization")
	triagePath := flags.String("triage", "", "versioned exact-identity triage JSON; omitted means no exceptions")
	output := flags.String("output", "", "new result directory below FLASHGATE_WORK_ROOT")
	at := flags.String("at", "", "explicit UTC assessment time for reproducible offline evaluation")
	if err := flags.Parse(arguments); err != nil {
		return errors.New("invalid security-assess arguments")
	}
	if flags.NArg() != 0 || *dir == "" || *online == (*snapshotPath != "") || (*online && *at != "") {
		return errors.New("security-assess requires --dir and exactly one of --snapshot or --online; --at is offline only")
	}
	if validateCandidateIdentity(*version, *commit) != nil || !hashPattern.MatchString(*manifest) || !hashPattern.MatchString(*index) {
		return errors.New("security-assess requires valid external version, commit, manifest and index anchors")
	}
	if err := securityOutputPath(*output, *dir); err != nil {
		return err
	}
	now := time.Now().UTC()
	if *at != "" {
		var err error
		now, err = time.Parse(time.RFC3339, *at)
		if err != nil || now.UTC().Format(time.RFC3339) != *at {
			return errors.New("assessment time must be canonical UTC")
		}
	}
	identity := securityIdentity{Version: *version, Commit: *commit, Manifest: *manifest, Index: *index}
	identity, modules, inventoryErr := readSecurityInventory(*dir, identity)
	triage := securityTriage{Schema: "flashgate-security-triage/v1", Exceptions: []securityException{}}
	var snapshot securitySnapshot
	var snapshotBytes, triageBytes []byte
	problem := ""
	if inventoryErr != nil {
		problem = "INVENTORY_VERIFICATION_FAILED"
	} else if *online {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		snapshot = newSecurityOSV().capture(ctx, identity, modules, now)
		cancel()
		snapshotBytes, _ = supplyJSON(snapshot)
	} else {
		var err error
		snapshotBytes, err = publicationReadFile(*snapshotPath)
		if err != nil || strictCandidateBytes(snapshotBytes, &snapshot) != nil {
			problem = "SNAPSHOT_READ_OR_SCHEMA_ERROR"
		}
	}
	if *triagePath != "" && problem == "" {
		var err error
		triageBytes, err = publicationReadFile(*triagePath)
		if err != nil || strictCandidateBytes(triageBytes, &triage) != nil {
			problem = "TRIAGE_READ_OR_SCHEMA_ERROR"
		}
	}
	r := assessSecurity(identity, modules, snapshot, triage, now)
	if problem != "" {
		r.Status = "ERROR"
		r.Quality = []string{problem}
	}
	if len(snapshotBytes) > 0 {
		r.SnapshotSHA256 = supplyDigest(snapshotBytes)
	}
	if len(triageBytes) > 0 {
		r.TriageSHA256 = supplyDigest(triageBytes)
	}
	if err := securityOutputPath(*output, *dir); err != nil {
		return err
	}
	if os.Mkdir(*output, 0o700) != nil {
		return errors.New("assessment output creation failed")
	}
	if err := writeJSON(filepath.Join(*output, "assessment.json"), r); err != nil {
		return errors.New("assessment report write failed")
	}
	if *online && len(snapshotBytes) > 0 {
		if err := writeReleaseNotes(filepath.Join(*output, "snapshot.json"), string(snapshotBytes)); err != nil {
			return errors.New("assessment snapshot write failed")
		}
	}
	summary := securitySummary(r)
	if err := writeReleaseNotes(filepath.Join(*output, "summary.md"), summary); err != nil {
		return errors.New("assessment summary write failed")
	}
	fmt.Fprint(os.Stdout, summary)
	if r.Status != "COMPLETE" {
		return errors.New("security assessment incomplete; see task-local report")
	}
	// Findings and documented exceptions never determine the CLI exit status.
	return nil
}

func securitySummary(r securityAssessment) string {
	return fmt.Sprintf("## Informative dependency security assessment\n\nStatus: %s; freshness: %s; modules: %d; findings: %d; quality findings: %d.\n\nFindings and assessment errors do not block commits, PRs, merges or releases. Existing integrity, build and test gates remain mandatory. Source Go modules only; runtime reachability and full platform composition are unproven.\n", r.Status, r.Freshness, len(r.Modules), len(r.Findings), len(r.Quality))
}
