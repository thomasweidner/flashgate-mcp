package main

// Historical normalization never creates or publishes a release. The old
// publication manifest is read-only evidence, not a renewed write approval.
import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"sort"
	"time"
)

const normalizationSchema = "flashgate-publication-normalization/v1"
const normalizationUpload = "UPLOAD_SHA256SUMS"
const normalizationDelete = "DELETE_ADDITIONAL_ASSETS"

type normalizationAsset struct {
	ID    int64            `json:"id"`
	Asset publicationAsset `json:"asset"`
}

type normalizationPlan struct {
	Schema         string               `json:"schema"`
	Original       publicationPlan      `json:"originalPublication"`
	OriginalSHA256 string               `json:"originalPublicationSha256"`
	ReleaseID      int64                `json:"releaseId"`
	PublishedAt    string               `json:"publishedAt"`
	OriginalAssets []normalizationAsset `json:"originalAssets"`
	Retain         []normalizationAsset `json:"retain"`
	Delete         []normalizationAsset `json:"delete"`
	Checksum       publicationAsset     `json:"checksum"`
}

type normalizationAuthorization struct {
	Schema           string `json:"schema"`
	Action           string `json:"action"`
	Repository       string `json:"repository"`
	ReleaseID        int64  `json:"releaseId"`
	PlanSHA256       string `json:"planSha256"`
	PrestateSHA256   string `json:"prestateSha256"`
	OriginalApproval string `json:"originalApproval"`
}

type normalizationProvider interface {
	publicationProvider
	DeleteAsset(context.Context, int64) error
}

func normalizationOriginal(dir string, original publicationPlan, anchor string) ([]publicationAsset, []publicationAsset, error) {
	data, err := supplyJSON(original)
	if err != nil || !hashPattern.MatchString(anchor) || supplyDigest(data) != anchor || original.Schema != "flashgate-publication-manifest/v1" || original.Authorization.Version != "0.3.0" || original.Tag != "v0.3.0" || original.Tag != "v"+original.Authorization.Version || validatePublicationAuthorization(original.Authorization) != nil || !hashPattern.MatchString(original.AuthorizationSHA256) || supplyDigest([]byte(original.Notes)) != original.Authorization.NotesSHA256 {
		return nil, nil, errors.New("original v0.3.0 publication binding required")
	}
	a := original.Authorization
	if err := verifySupply(dir, a.Version, a.SourceCommit, a.ManifestSHA256, a.IndexSHA256, nil); err != nil {
		return nil, nil, err
	}
	legacy, err := publicationEvidenceAssets(dir)
	if err != nil {
		return nil, nil, err
	}
	encoded, err := supplyJSON(legacy)
	if err != nil || len(legacy) != 39 || !reflect.DeepEqual(legacy, original.Assets) || supplyDigest(encoded) != a.AssetsSHA256 {
		return nil, nil, errors.New("original complete 40-asset evidence unavailable or changed")
	}
	legacy = append(legacy, publicationAsset{"publication-manifest.json", "", int64(len(data)), anchor})
	projected, err := publicationAssets(dir)
	return legacy, projected, err
}

func normalizationSnapshot(state remotePublicationState) remotePublicationState {
	if state.Release != nil {
		r := *state.Release
		r.Assets = append([]remotePublicationAsset{}, r.Assets...)
		sort.Slice(r.Assets, func(i, j int) bool { return r.Assets[i].Name < r.Assets[j].Name })
		state.Release = &r
	}
	return state
}

func prepareNormalization(ctx context.Context, p publicationProvider, dir string, original publicationPlan, originalHash string, releaseID int64) (normalizationPlan, error) {
	plan := normalizationPlan{Schema: normalizationSchema, Original: original, OriginalSHA256: originalHash, ReleaseID: releaseID}
	legacy, projected, err := normalizationOriginal(dir, original, originalHash)
	if err != nil {
		return plan, err
	}
	state, err := publicationReadbackState(ctx, p, original, releaseID, false, legacy)
	if err != nil {
		return plan, err
	}
	state = normalizationSnapshot(state)
	plan.PublishedAt = state.Release.PublishedAt
	byName := map[string]publicationAsset{}
	for _, a := range legacy {
		byName[a.Name] = a
	}
	keep := map[string]bool{}
	for _, a := range projected {
		if a.Name == publicationChecksumsName {
			plan.Checksum = a
		} else {
			keep[a.Name] = true
		}
	}
	for _, actual := range state.Release.Assets {
		asset := normalizationAsset{actual.ID, byName[actual.Name]}
		plan.OriginalAssets = append(plan.OriginalAssets, asset)
		if keep[actual.Name] {
			plan.Retain = append(plan.Retain, asset)
		} else {
			plan.Delete = append(plan.Delete, asset)
		}
	}
	if len(plan.Retain) != 4 || len(plan.Delete) != 36 {
		return plan, errors.New("exact four-retain/36-delete partition required")
	}
	return plan, nil
}

func validateNormalization(dir string, plan normalizationPlan) error {
	legacy, projected, err := normalizationOriginal(dir, plan.Original, plan.OriginalSHA256)
	if err != nil {
		return err
	}
	if plan.Schema != normalizationSchema || plan.ReleaseID < 1 || len(plan.OriginalAssets) != 40 || len(plan.Retain) != 4 || len(plan.Delete) != 36 {
		return errors.New("invalid normalization scope")
	}
	byName := map[string]normalizationAsset{}
	ids := map[int64]bool{}
	previous := ""
	for _, a := range plan.OriginalAssets {
		if a.ID < 1 || ids[a.ID] || a.Asset.Name <= previous {
			return errors.New("duplicate, unsafe or unordered original identity")
		}
		byName[a.Asset.Name] = a
		ids[a.ID] = true
		previous = a.Asset.Name
	}
	keep := map[string]bool{}
	var checksum publicationAsset
	for _, a := range projected {
		if a.Name == publicationChecksumsName {
			checksum = a
		} else {
			keep[a.Name] = true
		}
	}
	if plan.Checksum != checksum {
		return errors.New("normalization checksum binding changed")
	}
	var retain, remove []normalizationAsset
	for _, expected := range legacy {
		actual, ok := byName[expected.Name]
		if !ok || actual.Asset != expected {
			return errors.New("original asset hash/size/path identity changed")
		}
		if keep[expected.Name] {
			retain = append(retain, actual)
		} else {
			remove = append(remove, actual)
		}
	}
	sort.Slice(retain, func(i, j int) bool { return retain[i].Asset.Name < retain[j].Asset.Name })
	sort.Slice(remove, func(i, j int) bool { return remove[i].Asset.Name < remove[j].Asset.Name })
	if !reflect.DeepEqual(retain, plan.Retain) || !reflect.DeepEqual(remove, plan.Delete) {
		return errors.New("retain/delete partition changed")
	}
	return nil
}

// Returning a canonical complete snapshot lets a later human approval bind
// exactly the observed IDs, names, sizes, hashes, release and retained set.
func normalizationReadback(ctx context.Context, p publicationProvider, plan normalizationPlan, expected []normalizationAsset, checksum bool) (remotePublicationState, error) {
	assets := make([]publicationAsset, 0, len(expected)+1)
	for _, a := range expected {
		assets = append(assets, a.Asset)
	}
	if checksum {
		assets = append(assets, plan.Checksum)
	}
	state, err := publicationReadbackState(ctx, p, plan.Original, plan.ReleaseID, false, assets)
	if err != nil {
		return state, err
	}
	if state.Release.PublishedAt != plan.PublishedAt {
		return state, errors.New("original publication identity changed")
	}
	byName := map[string]int64{}
	for _, a := range state.Release.Assets {
		byName[a.Name] = a.ID
	}
	for _, a := range expected {
		if byName[a.Asset.Name] != a.ID {
			return state, errors.New("original asset ID changed")
		}
	}
	return normalizationSnapshot(state), nil
}

func validateNormalizationAuthorization(a normalizationAuthorization, plan normalizationPlan) error {
	data, err := supplyJSON(plan)
	if err != nil || a.Schema != "flashgate-publication-normalization-authorization/v1" || a.Repository != publicationRepository || a.ReleaseID != plan.ReleaseID || (a.Action != normalizationUpload && a.Action != normalizationDelete) || !hashPattern.MatchString(a.PlanSHA256) || a.PlanSHA256 != supplyDigest(data) || !hashPattern.MatchString(a.PrestateSHA256) || !publicationApprovalPattern.MatchString(a.OriginalApproval) || a.OriginalApproval == plan.Original.Authorization.OriginalApproval {
		return errors.New("separate exact normalization authorization required")
	}
	leaks := leakReport{}
	scanBytes("normalization approval", []byte(a.OriginalApproval), nil, &leaks)
	if len(leaks.Findings) > 0 {
		return errors.New("unsafe approval reference")
	}
	return nil
}

func executeNormalization(ctx context.Context, p normalizationProvider, dir string, plan normalizationPlan, a normalizationAuthorization, anchor, journal string) (result error) {
	if err := validateNormalizationAuthorization(a, plan); err != nil {
		return err
	}
	if !hashPattern.MatchString(anchor) {
		return errors.New("external authorization digest required")
	}
	if err := validateNormalization(dir, plan); err != nil {
		return err
	}
	withChecksum := a.Action == normalizationDelete
	state, err := normalizationReadback(ctx, p, plan, plan.OriginalAssets, withChecksum)
	if err != nil {
		return fmt.Errorf("PREFLIGHT HOLD: %w", err)
	}
	data, err := supplyJSON(state)
	if err != nil || supplyDigest(data) != a.PrestateSHA256 {
		return errors.New("PREFLIGHT HOLD: authorized provider prestate changed")
	}
	log, intent, err := publicationJournal(journal, a.PlanSHA256, anchor)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, log.Close()) }()
	if a.Action == normalizationUpload {
		root, err := supplyOpen(dir)
		if err != nil {
			return err
		}
		defer root.Close()
		assets, err := publicationAssets(dir)
		if err != nil {
			return err
		}
		reader, err := publicationAssetReader(root, plan.Checksum, assets)
		if err != nil {
			return err
		}
		if err := intent("NORMALIZATION_UPLOAD", normalizationUpload, plan.ReleaseID); err != nil {
			return errors.Join(err, reader.Close())
		}
		err = errors.Join(p.Upload(ctx, plan.ReleaseID, plan.Checksum, reader), reader.Close())
		if err != nil {
			return errors.New("HOLD: checksum upload failed or ambiguous; read-only reconciliation required")
		}
		if _, err := normalizationReadback(ctx, p, plan, plan.OriginalAssets, true); err != nil {
			return fmt.Errorf("HOLD after checksum upload: %w", err)
		}
		return intent("NORMALIZATION_READBACK", "VERIFIED_CHECKSUM_UPLOAD", plan.ReleaseID)
	}
	// Retain the newly approved checksum ID as well as every original retained ID.
	checksumID := int64(0)
	for _, asset := range state.Release.Assets {
		if asset.Name == publicationChecksumsName {
			checksumID = asset.ID
		}
	}
	// Establish public availability and byte parity before any irreversible delete.
	// Keep the final public readback too: either phase can fail independently.
	if err := normalizationPublicReadback(ctx, p, dir, plan); err != nil {
		return fmt.Errorf("HOLD before first asset deletion: %w", err)
	}
	if err := intent("NORMALIZATION_PUBLIC_READBACK", "VERIFIED_FIVE_ASSETS_BEFORE_DELETE", plan.ReleaseID); err != nil {
		return err
	}
	remaining := append([]normalizationAsset{}, plan.OriginalAssets...)
	for _, remove := range plan.Delete {
		current, err := normalizationReadback(ctx, p, plan, remaining, true)
		if err != nil {
			return fmt.Errorf("HOLD before asset deletion: %w", err)
		}
		for _, asset := range current.Release.Assets {
			if asset.Name == publicationChecksumsName && asset.ID != checksumID {
				return errors.New("HOLD: checksum asset ID changed")
			}
		}
		if err := intent("NORMALIZATION_DELETE", fmt.Sprintf("DELETE_ASSET_%d", remove.ID), plan.ReleaseID); err != nil {
			return err
		}
		if err := p.DeleteAsset(ctx, remove.ID); err != nil {
			return errors.New("HOLD: asset delete failed or ambiguous; read-only reconciliation required")
		}
		for i, asset := range remaining {
			if asset.ID == remove.ID {
				remaining = append(remaining[:i], remaining[i+1:]...)
				break
			}
		}
		post, err := normalizationReadback(ctx, p, plan, remaining, true)
		if err != nil {
			return fmt.Errorf("HOLD after asset deletion: %w", err)
		}
		for _, asset := range post.Release.Assets {
			if asset.Name == publicationChecksumsName && asset.ID != checksumID {
				return errors.New("HOLD: checksum asset ID changed after deletion")
			}
		}
	}
	if err := normalizationPublicReadback(ctx, p, dir, plan); err != nil {
		return fmt.Errorf("HOLD final public download: %w", err)
	}
	final, err := normalizationReadback(ctx, p, plan, plan.Retain, true)
	if err != nil {
		return fmt.Errorf("HOLD final normalization readback: %w", err)
	}
	for _, asset := range final.Release.Assets {
		if asset.Name == publicationChecksumsName && asset.ID != checksumID {
			return errors.New("HOLD: final checksum ID changed")
		}
	}
	return intent("NORMALIZATION_READBACK", "VERIFIED_FIVE_ASSET_RELEASE", plan.ReleaseID)
}

// Public reads use the existing credential-free provider route and independently
// verified original archive/checksum hashes, both before deletion and at closure.
func normalizationPublicReadback(ctx context.Context, p publicationProvider, dir string, plan normalizationPlan) error {
	projected, err := publicationAssets(dir)
	if err != nil {
		return err
	}
	for _, asset := range projected {
		stream, err := p.ReadPublicAsset(ctx, plan.Original, asset)
		if err != nil {
			return errors.New("normalized public download unavailable")
		}
		digest, size, err := publicationHash(stream, asset.Size)
		if err != nil || size != asset.Size || digest != asset.SHA256 {
			return errors.New("normalized public bytes differ")
		}
	}
	return nil
}

// Reconciliation is read-only, including after a consumed/ambiguous mutation.
// It recognizes only the original set, checksum upload, or a deletion prefix.
func reconcileNormalization(ctx context.Context, p publicationProvider, dir string, plan normalizationPlan) (remotePublicationState, error) {
	if err := validateNormalization(dir, plan); err != nil {
		return remotePublicationState{}, err
	}
	state, err := p.InspectRelease(ctx, plan.Original, plan.ReleaseID)
	if err != nil || state.Release == nil {
		return state, errors.New("normalization provider state unavailable")
	}
	names := map[string]bool{}
	for _, a := range state.Release.Assets {
		names[a.Name] = true
	}
	withChecksum := names[publicationChecksumsName]
	remaining := append([]normalizationAsset{}, plan.OriginalAssets...)
	for _, a := range plan.Delete {
		if names[a.Asset.Name] {
			break
		}
		if !withChecksum {
			return state, errors.New("deletion without verified checksum upload")
		}
		for i, b := range remaining {
			if a.ID == b.ID {
				remaining = append(remaining[:i], remaining[i+1:]...)
				break
			}
		}
	}
	return normalizationReadback(ctx, p, plan, remaining, withChecksum)
}

func readNormalizationPlan(name, anchor string) (normalizationPlan, error) {
	var plan normalizationPlan
	data, err := publicationReadFile(name)
	if err != nil || !hashPattern.MatchString(anchor) || supplyDigest(data) != anchor {
		return plan, errors.New("normalization plan digest mismatch")
	}
	if err := strictCandidateBytes(data, &plan); err != nil {
		return plan, err
	}
	return plan, nil
}

func runNormalization(arguments []string, mode string) error {
	flags := flag.NewFlagSet("publication-normalization-"+mode, flag.ContinueOnError)
	dir := flags.String("dir", "", "complete original verified evidence bundle")
	originalPath := flags.String("publication-manifest", "", "original 40-asset publication manifest")
	originalHash := flags.String("publication-manifest-sha256", "", "independent original manifest digest")
	statePath := flags.String("state", "", "complete provider snapshot for offline preflight")
	id := flags.Int64("release-id", 0, "exact original release ID")
	planPath := flags.String("plan", "", "reviewed normalization plan")
	planHash := flags.String("plan-sha256", "", "independent normalization plan digest")
	authPath := flags.String("authorization", "", "separate upload or delete authorization")
	anchor := flags.String("authorization-sha256", "", "independent authorization digest")
	output := flags.String("output", "", "new plan, readback or journal path")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 || *dir == "" || *output == "" {
		return errors.New("normalization requires dir, new output and no positional arguments")
	}
	if err := rejectSymlinkParents(filepath.Dir(*output)); err != nil {
		return err
	}
	if mode == "preflight" {
		if *planPath != "" || *planHash != "" || *authPath != "" || *anchor != "" {
			return errors.New("offline normalization preflight cannot consume execution authorization")
		}
		data, err := publicationReadFile(*originalPath)
		if err != nil {
			return err
		}
		var original publicationPlan
		if strictCandidateBytes(data, &original) != nil {
			return errors.New("invalid original publication manifest")
		}
		canonical, err := supplyJSON(original)
		if err != nil || !bytes.Equal(canonical, data) {
			return errors.New("original publication bytes not canonical")
		}
		data, err = publicationReadFile(*statePath)
		if err != nil {
			return err
		}
		var state remotePublicationState
		if strictCandidateBytes(data, &state) != nil {
			return errors.New("invalid provider snapshot")
		}
		legacy, _, err := normalizationOriginal(*dir, original, *originalHash)
		if err != nil {
			return err
		}
		plan, err := prepareNormalization(context.Background(), normalizationSnapshotProvider{state, *dir, legacy, canonical}, *dir, original, *originalHash, *id)
		if err != nil {
			return err
		}
		encoded, err := supplyJSON(plan)
		if err != nil {
			return err
		}
		return writeReleaseNotes(*output, string(encoded))
	}
	if (mode != "execute" && mode != "reconcile") || *originalPath != "" || *originalHash != "" || *statePath != "" || *id != 0 {
		return errors.New("normalization execution/readback requires only its bound plan and authorization")
	}
	plan, err := readNormalizationPlan(*planPath, *planHash)
	if err != nil {
		return err
	}
	var authorization normalizationAuthorization
	data, err := publicationReadFile(*authPath)
	if err != nil || !hashPattern.MatchString(*anchor) || supplyDigest(data) != *anchor || strictCandidateBytes(data, &authorization) != nil {
		return errors.New("normalization authorization bytes invalid")
	}
	if err := validateNormalizationAuthorization(authorization, plan); err != nil {
		return err
	}
	p, err := newGitHubPublication(plan.Original.Authorization, *anchor)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	if mode == "reconcile" {
		state, err := reconcileNormalization(ctx, p, *dir, plan)
		if err != nil {
			return err
		}
		encoded, err := supplyJSON(state)
		if err != nil {
			return err
		}
		return writeReleaseNotes(*output, string(encoded))
	}
	if mode != "execute" {
		return errors.New("unknown normalization operation")
	}
	if err := p.checkAttempt(ctx, authorization.OriginalApproval); err != nil {
		return err
	}
	return executeNormalization(ctx, p, *dir, plan, authorization, *anchor, *output)
}

// Offline preflight has no write capability. Snapshot IDs/digests are metadata;
// original bytes are still checked through the verified local evidence bundle.
type normalizationSnapshotProvider struct {
	state    remotePublicationState
	dir      string
	assets   []publicationAsset
	manifest []byte
}

func (p normalizationSnapshotProvider) InspectRelease(context.Context, publicationPlan, int64) (remotePublicationState, error) {
	return p.state, nil
}
func (p normalizationSnapshotProvider) Inspect(context.Context, publicationPlan) (remotePublicationState, error) {
	return p.state, nil
}
func (p normalizationSnapshotProvider) ReadAsset(_ context.Context, id int64) (io.ReadCloser, error) {
	name := ""
	for _, a := range p.state.Release.Assets {
		if a.ID == id {
			name = a.Name
		}
	}
	for _, a := range p.assets {
		if a.Name == name {
			if a.Name == "publication-manifest.json" {
				return io.NopCloser(bytes.NewReader(p.manifest)), nil
			}
			root, err := supplyOpen(p.dir)
			if err != nil {
				return nil, err
			}
			defer root.Close()
			return supplyRegularOpen(root, a.Path, a.Size)
		}
	}
	return nil, errors.New("offline snapshot contains an unbound asset")
}
func (p normalizationSnapshotProvider) ReadPublicAsset(context.Context, publicationPlan, publicationAsset) (io.ReadCloser, error) {
	return nil, errors.New("offline snapshot cannot download")
}
func (p normalizationSnapshotProvider) CreateDraft(context.Context, publicationPlan) (int64, error) {
	return 0, errors.New("offline preflight is read-only")
}
func (p normalizationSnapshotProvider) Upload(context.Context, int64, publicationAsset, io.Reader) error {
	return errors.New("offline preflight is read-only")
}
func (p normalizationSnapshotProvider) Publish(context.Context, int64, publicationPlan) error {
	return errors.New("offline preflight is read-only")
}
