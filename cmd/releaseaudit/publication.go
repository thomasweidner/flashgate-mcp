package main

// Publication consumes the existing candidate and supply-chain authorities.
// It never builds, repacks, repairs evidence, creates tags or replaces assets.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const publicationRepository = "thomasweidner/flashgate-mcp"

var publicationApprovalPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,95}$`)

// This document is an external human authorization assertion, not a credential
// or a machine attestation. Its digest must arrive through a trusted channel.
type publicationAuthorization struct {
	Schema              string `json:"schema"`
	Action              string `json:"action"`
	Repository          string `json:"repository"`
	Version             string `json:"version"`
	SourceCommit        string `json:"sourceCommit"`
	ManifestSHA256      string `json:"manifestSha256"`
	IndexSHA256         string `json:"indexSha256"`
	ChangelogSHA256     string `json:"changelogSha256"`
	NotesSHA256         string `json:"notesSha256"`
	AssetsSHA256        string `json:"assetsSha256"`
	CandidateRunID      int64  `json:"candidateRunId"`
	CandidateRunAttempt int64  `json:"candidateRunAttempt"`
	EvidenceArtifactID  int64  `json:"evidenceArtifactId"`
	EvidenceOrigin      string `json:"evidenceOrigin"`
	PreState            string `json:"preState"`
	OriginalApproval    string `json:"originalApproval"`
}

type publicationAsset struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type publicationPlan struct {
	Schema              string                   `json:"schema"`
	Authorization       publicationAuthorization `json:"authorization"`
	AuthorizationSHA256 string                   `json:"authorizationSha256"`
	Tag                 string                   `json:"tag"`
	Notes               string                   `json:"notes"`
	Assets              []publicationAsset       `json:"assets"`
}

func readPublicationAuthorization(name, anchor string) (publicationAuthorization, error) {
	var auth publicationAuthorization
	root, err := supplyOpen(filepath.Dir(name))
	if err != nil {
		return auth, err
	}
	defer root.Close()
	data, err := supplyRead(root, filepath.Base(name))
	if err != nil {
		return auth, err
	}
	if !hashPattern.MatchString(anchor) || supplyDigest(data) != anchor {
		return auth, errors.New("external authorization digest mismatch")
	}
	if err := strictCandidateBytes(data, &auth); err != nil {
		return auth, err
	}
	return auth, validatePublicationAuthorization(auth)
}

func validatePublicationAuthorization(a publicationAuthorization) error {
	if a.Schema != "flashgate-publication-authorization/v1" || a.Action != "CREATE_DRAFT_UPLOAD_READBACK_PUBLISH_PRERELEASE" || a.Repository != publicationRepository || a.PreState != "RELEASE_ABSENT" {
		return errors.New("publication authorization action/repository/prestate mismatch")
	}
	if err := validateCandidateIdentity(a.Version, a.SourceCommit); err != nil {
		return err
	}
	if !strings.HasPrefix(a.Version, "0.") && !strings.HasPrefix(a.Version, "1.0.0-") {
		return errors.New("only pre-1.0 publication is supported; BL-263 owns stable releases")
	}
	for _, h := range []string{a.ManifestSHA256, a.IndexSHA256, a.ChangelogSHA256, a.NotesSHA256, a.AssetsSHA256} {
		if !hashPattern.MatchString(h) {
			return errors.New("external publication digest anchors required")
		}
	}
	if (a.EvidenceOrigin != "ACTIONS_ARTIFACT" && a.EvidenceOrigin != "BOUND_LOCAL_COPY") || a.CandidateRunID < 1 || a.CandidateRunAttempt < 1 || a.EvidenceArtifactID < 1 || !publicationApprovalPattern.MatchString(a.OriginalApproval) {
		return errors.New("bound Actions evidence and original approval reference required")
	}
	r := leakReport{}
	scanBytes("approval reference", []byte(a.OriginalApproval), nil, &r)
	if len(r.Findings) != 0 {
		return errors.New("unsafe approval reference")
	}
	return nil
}

func publicationGit(root string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	var output publicationGitOutput
	cmd.Stdout = &output
	err := cmd.Run()
	if err != nil {
		return nil, errors.New("publication source Git read failed")
	}
	return output.buffer.Bytes(), nil
}

type publicationGitOutput struct{ buffer bytes.Buffer }

func (b *publicationGitOutput) Write(data []byte) (int, error) {
	if len(data) > maximumChangelogSize-b.buffer.Len() {
		return 0, errors.New("Git source read exceeds bound")
	}
	return b.buffer.Write(data)
}

func publicationSource(root string, a publicationAuthorization) (string, error) {
	return publicationSourceWithGit(root, a, publicationGit)
}

func publicationSourceWithGit(root string, a publicationAuthorization, git func(string, ...string) ([]byte, error)) (string, error) {
	for _, check := range []struct {
		args     []string
		expected string
	}{
		{[]string{"rev-parse", "HEAD"}, a.SourceCommit},
		{[]string{"rev-parse", "refs/tags/v" + a.Version + "^{commit}"}, a.SourceCommit},
		{[]string{"status", "--porcelain=v1", "--untracked-files=all"}, ""},
		{[]string{"remote", "get-url", "origin"}, projectURI},
	} {
		data, err := git(root, check.args...)
		observed := strings.TrimSpace(string(data))
		if check.args[0] == "remote" {
			observed = strings.TrimSuffix(observed, ".git")
		}
		if err != nil || observed != check.expected {
			return "", errors.New("clean source/commit/tag/origin parity required")
		}
	}
	source, err := supplyOpen(root)
	if err != nil {
		return "", err
	}
	defer source.Close()
	inputs := map[string][]byte{}
	for _, name := range []string{"VERSION", "CHANGELOG.md"} {
		size, err := git(root, "cat-file", "-s", a.SourceCommit+":"+name)
		if err != nil {
			return "", err
		}
		n, err := strconv.ParseInt(strings.TrimSpace(string(size)), 10, 64)
		if err != nil || n < 1 || n > maximumChangelogSize {
			return "", errors.New("committed release source size invalid")
		}
		committed, err := git(root, "show", a.SourceCommit+":"+name)
		if err != nil {
			return "", err
		}
		actual, err := supplyRead(source, name)
		if err != nil || !bytes.Equal(bytes.ReplaceAll(actual, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(committed, []byte("\r\n"), []byte("\n"))) {
			return "", errors.New("release source changed after freeze")
		}
		inputs[name] = committed
	}
	return publicationNotes(inputs["VERSION"], inputs["CHANGELOG.md"], a)
}

func publicationNotes(value, changelog []byte, a publicationAuthorization) (string, error) {
	if string(value) != a.Version+"\n" && string(value) != a.Version {
		return "", errors.New("committed VERSION mismatch")
	}
	if supplyDigest(changelog) != a.ChangelogSHA256 {
		return "", errors.New("frozen CHANGELOG digest mismatch")
	}
	if !utf8.Valid(changelog) || bytes.HasPrefix(changelog, []byte{0xef, 0xbb, 0xbf}) || len(changelog) > maximumChangelogSize {
		return "", errors.New("invalid changelog encoding/size")
	}
	_, notes, err := auditChangelog(string(changelog), a.Version, true, false)
	if err != nil {
		return "", err
	}
	if supplyDigest([]byte(notes)) != a.NotesSHA256 {
		return "", errors.New("frozen release notes digest mismatch")
	}
	leaks := leakReport{}
	scanBytes("release notes", []byte(notes), nil, &leaks)
	if len(leaks.Findings) != 0 {
		return "", errors.New("release notes leak detected")
	}
	return notes, nil
}

func publicationAssets(dir string) ([]publicationAsset, error) {
	root, err := supplyOpen(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var assets []publicationAsset
	names := map[string]bool{}
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		publicName := name
		if strings.HasPrefix(name, "candidate/") {
			publicName = strings.TrimPrefix(name, "candidate/")
		}
		if strings.Contains(publicName, "/") {
			publicName = "evidence--" + strings.ReplaceAll(publicName, "/", "--")
		}
		key := strings.ToLower(publicName)
		if names[key] || key == "publication-manifest.json" {
			return errors.New("publication asset name collision")
		}
		names[key] = true
		file, err := supplyRegularOpen(root, name, maximumSupplyArchiveSize)
		if err != nil {
			return err
		}
		info, err := file.Stat()
		if err != nil {
			return errors.Join(err, file.Close())
		}
		digest, n, err := publicationHash(file, maximumSupplyArchiveSize)
		if err != nil || n != info.Size() {
			return errors.Join(errors.New("publication asset read failed"), err)
		}
		assets = append(assets, publicationAsset{publicName, name, n, digest})
		return nil
	})
	sort.Slice(assets, func(i, j int) bool { return assets[i].Name < assets[j].Name })
	return assets, err
}

// Own and close every read stream, including failed remote readbacks.
func publicationHash(reader io.ReadCloser, maximum int64) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(reader, maximum+1))
	err = errors.Join(err, reader.Close())
	if n > maximum {
		err = errors.Join(err, errors.New("publication stream exceeds bound"))
	}
	return fmt.Sprintf("%x", h.Sum(nil)), n, err
}

func preparePublication(root, dir string, a publicationAuthorization, anchor string) (publicationPlan, error) {
	return preparePublicationWithSource(root, dir, a, anchor, publicationSource)
}

func preparePublicationWithSource(root, dir string, a publicationAuthorization, anchor string, source func(string, publicationAuthorization) (string, error)) (publicationPlan, error) {
	plan := publicationPlan{Schema: "flashgate-publication-manifest/v1", Authorization: a, AuthorizationSHA256: anchor, Tag: "v" + a.Version}
	if err := validatePublicationAuthorization(a); err != nil {
		return plan, err
	}
	if !hashPattern.MatchString(anchor) {
		return plan, errors.New("external authorization anchor required")
	}
	if err := verifySupply(dir, a.Version, a.SourceCommit, a.ManifestSHA256, a.IndexSHA256, nil); err != nil {
		return plan, err
	}
	notes, err := source(root, a)
	if err != nil {
		return plan, err
	}
	plan.Notes = notes
	plan.Assets, err = publicationAssets(dir)
	if err != nil {
		return plan, err
	}
	data, err := supplyJSON(plan.Assets)
	if err != nil || supplyDigest(data) != a.AssetsSHA256 {
		return plan, errors.New("externally authorized asset inventory mismatch")
	}
	return plan, nil
}

type remotePublicationAsset struct {
	ID    int64
	Name  string
	Size  int64
	State string
}
type remotePublicationRelease struct {
	ID                      int64
	Tag, Target, Name, Body string
	Draft, Prerelease       bool
	PublishedAt             string
	Assets                  []remotePublicationAsset
}
type remotePublicationState struct {
	TagCommit string
	Release   *remotePublicationRelease
	LatestID  int64
}

type publicationProvider interface {
	Inspect(context.Context, publicationPlan) (remotePublicationState, error)
	InspectRelease(context.Context, publicationPlan, int64) (remotePublicationState, error)
	ReadAsset(context.Context, int64) (io.ReadCloser, error)
	ReadPublicAsset(context.Context, publicationPlan, publicationAsset) (io.ReadCloser, error)
	CreateDraft(context.Context, publicationPlan) (int64, error)
	Upload(context.Context, int64, publicationAsset, io.Reader) error
	Publish(context.Context, int64, publicationPlan) error
}

// Only these scalar values may cross the download diagnostic boundary.
// Never retain a response, URL, header, body or wrapped transport error here.
type publicationDownloadHTTPError struct {
	statusCode int
	redirected bool
}

func (e *publicationDownloadHTTPError) Error() string {
	phase := "api_response"
	if e.redirected {
		phase = "redirect_response"
	}
	return fmt.Sprintf("download unavailable (HTTP %d, phase=%s)", e.statusCode, phase)
}

// Parity diagnostics carry only fixed codes, numeric/boolean values and string
// fingerprints. Neither approved nor provider-controlled text crosses stderr.
type publicationParityString struct {
	Length int    `json:"length"`
	SHA256 string `json:"sha256"`
}

func publicationParityFingerprint(value string) publicationParityString {
	return publicationParityString{len(value), supplyDigest([]byte(value))}
}

type publicationParityRelease struct {
	ID         int64                   `json:"id"`
	Tag        publicationParityString `json:"tag"`
	Target     publicationParityString `json:"target"`
	Name       publicationParityString `json:"name"`
	Body       publicationParityString `json:"body"`
	Draft      bool                    `json:"draft"`
	Prerelease bool                    `json:"prerelease"`
	AssetCount int                     `json:"assetCount"`
}

func publicationParityReleaseEvidence(r *remotePublicationRelease) *publicationParityRelease {
	if r == nil {
		return nil
	}
	return &publicationParityRelease{r.ID, publicationParityFingerprint(r.Tag), publicationParityFingerprint(r.Target),
		publicationParityFingerprint(r.Name), publicationParityFingerprint(r.Body), r.Draft, r.Prerelease, len(r.Assets)}
}

func publicationParityError(codes []string, state remotePublicationState, plan publicationPlan, id int64, draft bool, assetCount int) error {
	expected := remotePublicationRelease{ID: id, Tag: plan.Tag, Target: plan.Authorization.SourceCommit,
		Name: "FlashGate MCP " + plan.Authorization.Version, Body: plan.Notes, Draft: draft, Prerelease: true}
	expectedEvidence := publicationParityReleaseEvidence(&expected)
	expectedEvidence.AssetCount = assetCount
	// This closed scalar structure cannot fail JSON marshaling. Its encoded size
	// is bounded independently of all input strings and asset inventories.
	data, _ := json.Marshal(struct {
		Codes             []string                  `json:"codes"`
		CreateDraftID     int64                     `json:"createDraftID"`
		ExpectedTagCommit publicationParityString   `json:"expectedTagCommit"`
		ObservedTagCommit publicationParityString   `json:"observedTagCommit"`
		ExpectedRelease   *publicationParityRelease `json:"expectedRelease"`
		ObservedRelease   *publicationParityRelease `json:"observedRelease"`
		LatestID          int64                     `json:"latestID"`
	}{codes, id, publicationParityFingerprint(plan.Authorization.SourceCommit), publicationParityFingerprint(state.TagCommit),
		expectedEvidence, publicationParityReleaseEvidence(state.Release), state.LatestID})
	return fmt.Errorf("remote release parity failed: %s", data)
}

func publicationReadback(ctx context.Context, p publicationProvider, plan publicationPlan, id int64, draft bool, assets []publicationAsset) error {
	state, err := p.InspectRelease(ctx, plan, id)
	if err != nil {
		return errors.New("provider readback failed")
	}
	r := state.Release
	var codes []string
	check := func(failed bool, code string) {
		if failed {
			codes = append(codes, code)
		}
	}
	check(state.TagCommit != plan.Authorization.SourceCommit, "TAG_COMMIT")
	check(r == nil, "RELEASE_PRESENT")
	if r != nil {
		check(r.ID != id, "RELEASE_ID")
		check(r.Tag != plan.Tag, "RELEASE_TAG")
		check(r.Target != plan.Authorization.SourceCommit, "RELEASE_TARGET")
		check(r.Name != "FlashGate MCP "+plan.Authorization.Version, "RELEASE_NAME")
		check(r.Body != plan.Notes, "RELEASE_BODY")
		check(r.Draft != draft, "RELEASE_DRAFT")
		check(!r.Prerelease, "RELEASE_PRERELEASE")
	}
	check(state.LatestID == id, "RELEASE_NOT_LATEST")
	if r != nil {
		check(len(r.Assets) != len(assets), "ASSET_COUNT")
	}
	if len(codes) != 0 {
		return publicationParityError(codes, state, plan, id, draft, len(assets))
	}
	if draft && r.PublishedAt != "" {
		return errors.New("draft has contradictory publication timestamp")
	}
	if !draft {
		if _, err := time.Parse(time.RFC3339, r.PublishedAt); err != nil {
			return errors.New("remote publication timestamp unavailable")
		}
	}
	seen := map[int64]bool{}
	byName := map[string]remotePublicationAsset{}
	for _, asset := range r.Assets {
		if asset.ID < 1 || seen[asset.ID] || byName[asset.Name].ID != 0 || asset.State != "uploaded" {
			return errors.New("remote asset conflict")
		}
		seen[asset.ID] = true
		byName[asset.Name] = asset
	}
	for _, expected := range assets {
		actual, ok := byName[expected.Name]
		if !ok || actual.Size != expected.Size {
			return errors.New("remote asset inventory mismatch")
		}
		stream, err := p.ReadAsset(ctx, actual.ID)
		if err != nil {
			var status *publicationDownloadHTTPError
			if errors.As(err, &status) {
				return fmt.Errorf("remote asset readback unavailable: %w", status)
			}
			return errors.New("remote asset readback unavailable")
		}
		digest, n, err := publicationHash(stream, expected.Size)
		if err != nil || n != expected.Size || digest != expected.SHA256 {
			return errors.New("remote asset byte parity failed")
		}
	}
	return nil
}

// A journal is exclusively created before a write. Each intent is fsynced
// before dispatch. No error path retries, rolls back or changes provider.
func executePublication(ctx context.Context, p publicationProvider, plan publicationPlan, dir, journal string) (result error) {
	return executePublicationTarget(ctx, p, plan, dir, journal, 0, plan.AuthorizationSHA256)
}

// A positive target is supplied only by a separately bound continuation. The
// original publication manifest is identical in the create and continue paths.
func executePublicationTarget(ctx context.Context, p publicationProvider, plan publicationPlan, dir, journal string, target int64, attemptAnchor string) (result error) {
	if err := validatePublicationAuthorization(plan.Authorization); err != nil {
		return err
	}
	if !hashPattern.MatchString(attemptAnchor) || plan.Schema != "flashgate-publication-manifest/v1" || plan.Tag != "v"+plan.Authorization.Version || supplyDigest([]byte(plan.Notes)) != plan.Authorization.NotesSHA256 {
		return errors.New("PREFLIGHT HOLD: publication plan binding mismatch")
	}
	a := plan.Authorization
	if err := verifySupply(dir, a.Version, a.SourceCommit, a.ManifestSHA256, a.IndexSHA256, nil); err != nil {
		return fmt.Errorf("PREFLIGHT HOLD: %w", err)
	}
	assets, err := publicationAssets(dir)
	if err != nil || !reflect.DeepEqual(assets, plan.Assets) {
		return errors.New("PREFLIGHT HOLD: frozen asset inventory changed")
	}
	root, err := supplyOpen(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	if target == 0 {
		state, err := p.Inspect(ctx, plan)
		if err != nil || state.TagCommit != plan.Authorization.SourceCommit || state.Release != nil {
			return errors.New("PREFLIGHT HOLD: tag/release prestate conflict or unavailable")
		}
	} else {
		if target < 1 {
			return errors.New("PREFLIGHT HOLD: positive continuation release ID required")
		}
		if err := publicationReadback(ctx, p, plan, target, true, nil); err != nil {
			return fmt.Errorf("PREFLIGHT HOLD empty draft: %w", err)
		}
	}
	manifest, err := supplyJSON(plan)
	if err != nil {
		return err
	}
	planHash := supplyDigest(manifest)
	if err := rejectSymlinkParents(filepath.Dir(journal)); err != nil {
		return err
	}
	log, err := os.OpenFile(journal, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.New("HOLD: publication attempt already consumed or journal unavailable")
	}
	defer func() { result = errors.Join(result, log.Close()) }()
	intent := func(phase, action string, id int64) error {
		data, err := json.Marshal(struct {
			Plan, Phase, Action string
			Authorization       string
			ReleaseID           int64
		}{planHash, phase, action, attemptAnchor, id})
		if err != nil {
			return err
		}
		if _, err := log.Write(append(data, '\n')); err != nil {
			return err
		}
		return log.Sync()
	}
	id := target
	if target == 0 {
		if err := intent("DRAFT_STAGING", "CREATE_ONCE", 0); err != nil {
			return err
		}
		id, err = p.CreateDraft(ctx, plan)
		if err != nil || id < 1 {
			return errors.New("HOLD: draft write failed or ambiguous; read-only reconciliation required")
		}
	} else if err := intent("EMPTY_DRAFT_CONTINUATION", "CONTINUE_ONCE", id); err != nil {
		return err
	}
	transferred := []publicationAsset{}
	all := append(append([]publicationAsset{}, plan.Assets...), publicationAsset{"publication-manifest.json", "", int64(len(manifest)), planHash})
	for _, asset := range all {
		if err := publicationReadback(ctx, p, plan, id, true, transferred); err != nil {
			return fmt.Errorf("HOLD before transfer: %w", err)
		}
		var reader io.Reader = bytes.NewReader(manifest)
		var file *os.File
		if asset.Path != "" {
			file, err = supplyRegularOpen(root, asset.Path, asset.Size)
			if err != nil {
				return errors.New("HOLD: frozen asset unavailable")
			}
			reader = io.LimitReader(file, asset.Size)
		}
		if err := intent("EXACT_ASSET_TRANSFER", asset.Name, id); err != nil {
			if file != nil {
				_ = file.Close()
			}
			return err
		}
		err = p.Upload(ctx, id, asset, reader)
		if file != nil {
			err = errors.Join(err, file.Close())
		}
		if err != nil {
			return errors.New("HOLD: asset write failed or ambiguous; no retry")
		}
		transferred = append(transferred, asset)
	}
	if err := publicationReadback(ctx, p, plan, id, true, all); err != nil {
		return fmt.Errorf("HOLD independent readback: %w", err)
	}
	if err := intent("PUBLICATION", "PUBLISH_ONCE", id); err != nil {
		return err
	}
	if err := p.Publish(ctx, id, plan); err != nil {
		return errors.New("HOLD: publication write failed or ambiguous; no retry")
	}
	if err := publicationReadback(ctx, p, plan, id, false, all); err != nil {
		return fmt.Errorf("HOLD final readback: %w", err)
	}
	for _, asset := range all {
		stream, err := p.ReadPublicAsset(ctx, plan, asset)
		if err != nil {
			return errors.New("HOLD public download unavailable")
		}
		digest, size, err := publicationHash(stream, asset.Size)
		if err != nil || size != asset.Size || digest != asset.SHA256 {
			return errors.New("HOLD public download byte parity failed")
		}
	}
	if err := publicationReadback(ctx, p, plan, id, false, all); err != nil {
		return fmt.Errorf("HOLD after public download: %w", err)
	}
	return intent("FINAL_READBACK", "VERIFIED_PUBLIC_PRERELEASE", id)
}

func runPublication(arguments []string, mode string) error {
	flags := flag.NewFlagSet("publication-"+mode, flag.ContinueOnError)
	root := flags.String("root", "", "exact clean source checkout")
	dir := flags.String("dir", "", "verified evidence directory (new directory for download)")
	authPath := flags.String("authorization", "", "externally approved authorization JSON")
	anchor := flags.String("authorization-sha256", "", "external authorization SHA-256")
	output := flags.String("output", "", "new plan output or attempt journal")
	manifestPath := flags.String("publication-manifest", "", "unchanged original plan for an authorized empty-draft continuation")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || *dir == "" || *authPath == "" {
		return errors.New("publication requires dir, authorization and no positional arguments")
	}
	a, continuation, original, err := readPublicationRequest(*authPath, *anchor, *manifestPath)
	if err != nil {
		return err
	}
	approval := a.OriginalApproval
	if continuation != nil {
		approval = continuation.OriginalApproval
	}
	if mode == "download" || mode == "publish" {
		provider, err := newGitHubPublication(a, *anchor)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		if err := provider.checkAttempt(ctx, approval); err != nil {
			return err
		}
		if mode == "download" {
			if *root != "" || *output != "" {
				return errors.New("download accepts no source/output")
			}
			return provider.downloadEvidence(ctx, a, *dir)
		}
		if *root == "" || *output == "" {
			return errors.New("publish requires source and new journal")
		}
		plan, err := preparePublicationRequest(*root, *dir, a, *anchor, continuation, original)
		if err != nil {
			return err
		}
		if err := provider.verifyEvidenceOrigin(ctx, a); err != nil {
			return err
		}
		if continuation != nil {
			return executePublicationTarget(ctx, provider, plan, *dir, *output, continuation.ReleaseID, *anchor)
		}
		return executePublication(ctx, provider, plan, *dir, *output)
	}
	if mode != "preflight" || *root == "" || *output == "" {
		return errors.New("preflight requires source and new plan output")
	}
	plan, err := preparePublicationRequest(*root, *dir, a, *anchor, continuation, original)
	if err != nil {
		return err
	}
	data, err := supplyJSON(plan)
	if continuation != nil {
		data, err = supplyJSON(publicationContinuationPlan{"flashgate-publication-continuation-plan/v1", *continuation, *anchor})
	}
	if err != nil {
		return err
	}
	if err := rejectSymlinkParents(filepath.Dir(*output)); err != nil {
		return err
	}
	return writeReleaseNotes(*output, string(data))
}

// Read-only preparation for human review. These observed values do not grant
// authority: manifest/index anchors must already be independently trusted.
func runPublicationInventory(arguments []string) error {
	flags := flag.NewFlagSet("publication-inventory", flag.ContinueOnError)
	dir := flags.String("dir", "", "complete evidence bundle")
	value := flags.String("version", "", "expected VERSION")
	commit := flags.String("commit", "", "expected source commit")
	mh := flags.String("manifest-sha256", "", "external candidate manifest anchor")
	ih := flags.String("index-sha256", "", "external evidence index anchor")
	output := flags.String("output", "", "new asset inventory JSON output")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || *dir == "" || *output == "" {
		return errors.New("publication inventory requires dir, new output and no positional arguments")
	}
	if err := verifySupply(*dir, *value, *commit, *mh, *ih, nil); err != nil {
		return err
	}
	assets, err := publicationAssets(*dir)
	if err != nil {
		return err
	}
	data, err := supplyJSON(assets)
	if err != nil {
		return err
	}
	if err := rejectSymlinkParents(filepath.Dir(*output)); err != nil {
		return err
	}
	return writeReleaseNotes(*output, string(data))
}
