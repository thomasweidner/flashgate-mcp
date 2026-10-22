package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const githubAPI = "https://api.github.com"
const githubUploads = "https://uploads.github.com"

// Only this adapter knows the provider or reads a job credential. It has no
// retry, delete, asset replacement, tag creation or provider-fallback method.
type githubPublication struct {
	client       *http.Client
	token        string
	runID        int64
	api, uploads string
}

func newGitHubPublication(a publicationAuthorization, anchor string) (*githubPublication, error) {
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("GITHUB_REPOSITORY") != a.Repository || os.Getenv("GITHUB_EVENT_NAME") != "workflow_dispatch" || os.Getenv("GITHUB_REF") != "refs/heads/main" || os.Getenv("GITHUB_RUN_ATTEMPT") != "1" || os.Getenv("FLASHGATE_AUTHORIZATION_SHA256") != anchor || os.Getenv("GITHUB_WORKFLOW_REF") != a.Repository+"/.github/workflows/publish-prerelease.yml@refs/heads/main" {
		return nil, errors.New("publication requires the explicit main Actions context, bound authorization and first attempt")
	}
	run, err := strconv.ParseInt(os.Getenv("GITHUB_RUN_ID"), 10, 64)
	if err != nil || run < 1 || os.Getenv("GH_TOKEN") == "" {
		return nil, errors.New("short-lived publication job credential/context unavailable")
	}
	return &githubPublication{client: &http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, token: os.Getenv("GH_TOKEN"), runID: run, api: githubAPI, uploads: githubUploads}, nil
}

func (g *githubPublication) request(ctx context.Context, method, endpoint, accept string, body io.Reader, size int64) (*http.Response, error) {
	// Endpoints are constructed from fixed repository paths and validated IDs.
	if !strings.HasPrefix(endpoint, g.api+"/") && !strings.HasPrefix(endpoint, g.uploads+"/") {
		return nil, errors.New("provider endpoint rejected")
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, errors.New("provider request invalid")
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "flashgate-releaseaudit")
	if body != nil {
		req.ContentLength = size
		req.Header.Set("Content-Type", "application/json")
		if accept == "application/octet-stream" {
			req.Header.Set("Content-Type", accept)
		}
	}
	res, err := g.client.Do(req)
	if err != nil {
		return nil, errors.New("provider transport failed or ambiguous")
	}
	return res, nil
}

func (g *githubPublication) endpoint(path string) string {
	return g.api + "/repos/" + publicationRepository + path
}

func (g *githubPublication) readJSON(ctx context.Context, path string, value any, absentOK bool) (bool, error) {
	res, err := g.request(ctx, "GET", g.endpoint(path), "application/vnd.github+json", nil, 0)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if absentOK && res.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if res.StatusCode != http.StatusOK {
		return false, errors.New("provider read unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maximumSupplyMetadataSize+1))
	if err != nil || len(data) > maximumSupplyMetadataSize {
		return false, errors.New("provider response exceeds bound")
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return false, errors.New("provider returned no state")
	}
	// Provider objects are open schemas; reject duplicate keys and trailing JSON.
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return false, errors.New("invalid provider JSON")
	}
	if err := json.Unmarshal(data, value); err != nil {
		return false, errors.New("invalid provider JSON")
	}
	return true, nil
}

type githubObject struct {
	SHA  string `json:"sha"`
	Type string `json:"type"`
}
type githubRelease struct {
	ID         int64  `json:"id"`
	Tag        string `json:"tag_name"`
	Target     string `json:"target_commitish"`
	Name       string `json:"name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}
type githubAsset struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	State string `json:"state"`
}

func (g *githubPublication) tagCommit(ctx context.Context, tag string) (string, error) {
	var ref struct {
		Ref    string       `json:"ref"`
		Object githubObject `json:"object"`
	}
	if _, err := g.readJSON(ctx, "/git/ref/tags/"+url.PathEscape(tag), &ref, false); err != nil {
		return "", err
	}
	if ref.Ref != "refs/tags/"+tag {
		return "", errors.New("remote tag identity mismatch")
	}
	object := ref.Object
	seen := map[string]bool{}
	for depth := 0; depth < 8; depth++ {
		if !commitPattern.MatchString(object.SHA) || seen[object.SHA] {
			return "", errors.New("invalid remote tag object")
		}
		seen[object.SHA] = true
		if object.Type == "commit" {
			return object.SHA, nil
		}
		if object.Type != "tag" {
			return "", errors.New("remote tag does not resolve to commit")
		}
		var annotated struct {
			SHA    string       `json:"sha"`
			Object githubObject `json:"object"`
		}
		if _, err := g.readJSON(ctx, "/git/tags/"+object.SHA, &annotated, false); err != nil {
			return "", err
		}
		if annotated.SHA != object.SHA {
			return "", errors.New("annotated tag object mismatch")
		}
		object = annotated.Object
	}
	return "", errors.New("remote tag depth exceeded")
}

func (g *githubPublication) Inspect(ctx context.Context, plan publicationPlan) (remotePublicationState, error) {
	state := remotePublicationState{}
	commit, err := g.tagCommit(ctx, plan.Tag)
	if err != nil {
		return state, err
	}
	state.TagCommit = commit
	for page := 1; page <= 100; page++ {
		var releases []githubRelease
		if _, err := g.readJSON(ctx, fmt.Sprintf("/releases?per_page=100&page=%d", page), &releases, false); err != nil {
			return state, err
		}
		for _, r := range releases {
			if r.Tag != plan.Tag {
				continue
			}
			if state.Release != nil || r.ID < 1 {
				return state, errors.New("release collision")
			}
			state.Release = &remotePublicationRelease{ID: r.ID, Tag: r.Tag, Target: r.Target, Name: r.Name, Body: r.Body, Draft: r.Draft, Prerelease: r.Prerelease}
		}
		if len(releases) < 100 {
			break
		}
		if page == 100 {
			return state, errors.New("release pagination bound exceeded")
		}
	}
	var latest githubRelease
	if found, err := g.readJSON(ctx, "/releases/latest", &latest, true); err != nil {
		return state, err
	} else if found {
		state.LatestID = latest.ID
	}
	if state.Release != nil {
		for page := 1; page <= 100; page++ {
			var assets []githubAsset
			if _, err := g.readJSON(ctx, fmt.Sprintf("/releases/%d/assets?per_page=100&page=%d", state.Release.ID, page), &assets, false); err != nil {
				return state, err
			}
			for _, a := range assets {
				state.Release.Assets = append(state.Release.Assets, remotePublicationAsset(a))
			}
			if len(assets) < 100 {
				break
			}
			if page == 100 {
				return state, errors.New("asset pagination bound exceeded")
			}
		}
	}
	return state, nil
}

func (g *githubPublication) mutate(ctx context.Context, method, path string, payload any) (int64, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	res, err := g.request(ctx, method, g.endpoint(path), "application/vnd.github+json", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	expected := http.StatusCreated
	if method == "PATCH" {
		expected = http.StatusOK
	}
	if res.StatusCode != expected {
		return 0, errors.New("provider write failed or ambiguous; consumed")
	}
	var r struct {
		ID int64 `json:"id"`
	}
	data, err = io.ReadAll(io.LimitReader(res.Body, maximumSupplyMetadataSize+1))
	if err != nil || len(data) > maximumSupplyMetadataSize || rejectDuplicateJSONKeys(data) != nil || json.Unmarshal(data, &r) != nil || r.ID < 1 {
		return 0, errors.New("provider write response ambiguous; consumed")
	}
	return r.ID, nil
}

func publicationReleasePayload(plan publicationPlan, draft bool) map[string]any {
	payload := map[string]any{"tag_name": plan.Tag, "target_commitish": plan.Authorization.SourceCommit, "name": "FlashGate MCP " + plan.Authorization.Version, "body": plan.Notes, "draft": draft, "prerelease": true, "make_latest": "false"}
	if draft {
		payload["generate_release_notes"] = false
	}
	return payload
}
func (g *githubPublication) CreateDraft(ctx context.Context, plan publicationPlan) (int64, error) {
	return g.mutate(ctx, "POST", "/releases", publicationReleasePayload(plan, true))
}
func (g *githubPublication) Publish(ctx context.Context, id int64, plan publicationPlan) error {
	actual, err := g.mutate(ctx, "PATCH", fmt.Sprintf("/releases/%d", id), publicationReleasePayload(plan, false))
	if err == nil && actual != id {
		return errors.New("publication response identity mismatch")
	}
	return err
}
func (g *githubPublication) Upload(ctx context.Context, id int64, asset publicationAsset, reader io.Reader) error {
	endpoint := fmt.Sprintf("%s/repos/%s/releases/%d/assets?name=%s", g.uploads, publicationRepository, id, url.QueryEscape(asset.Name))
	res, err := g.request(ctx, "POST", endpoint, "application/octet-stream", reader, asset.Size)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return errors.New("asset transfer failed or ambiguous; consumed")
	}
	return nil
}

// Redirects are read-only and credentials never cross the API host boundary.
// Returned upload URLs and provider error bodies are never trusted or logged.
func (g *githubPublication) download(ctx context.Context, path string) (io.ReadCloser, error) {
	res, err := g.request(ctx, "GET", g.endpoint(path), "application/octet-stream", nil, 0)
	if err != nil {
		return nil, err
	}
	for redirects := 0; res.StatusCode == 302 || res.StatusCode == 301 || res.StatusCode == 307; redirects++ {
		location := res.Header.Get("Location")
		_ = res.Body.Close()
		u, err := url.Parse(location)
		if err != nil || redirects >= 3 || u.Scheme != "https" || u.User != nil || u.Port() != "" || !(u.Hostname() == "github.com" || strings.HasSuffix(u.Hostname(), ".githubusercontent.com") || strings.HasSuffix(u.Hostname(), ".github.com")) {
			return nil, errors.New("unsafe download redirect")
		}
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, errors.New("invalid download redirect")
		}
		res, err = g.client.Do(req)
		if err != nil {
			return nil, errors.New("download read failed")
		}
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, errors.New("download unavailable")
	}
	return res.Body, nil
}
func (g *githubPublication) ReadAsset(ctx context.Context, id int64) (io.ReadCloser, error) {
	return g.download(ctx, fmt.Sprintf("/releases/assets/%d", id))
}

// A workflow rerun is denied before any write. A new run with the same
// original approval is also denied, including reserialized authorization JSON.
func (g *githubPublication) checkAttempt(ctx context.Context, approval string) error {
	current := false
	for page := 1; page <= 100; page++ {
		var result struct {
			Runs []struct {
				ID      int64  `json:"id"`
				Attempt int64  `json:"run_attempt"`
				Title   string `json:"display_title"`
				Event   string `json:"event"`
				Branch  string `json:"head_branch"`
			} `json:"workflow_runs"`
		}
		if _, err := g.readJSON(ctx, fmt.Sprintf("/actions/workflows/publish-prerelease.yml/runs?per_page=100&page=%d", page), &result, false); err != nil {
			return err
		}
		for _, r := range result.Runs {
			if r.Title != "BL-363 publication "+approval {
				continue
			}
			if r.ID != g.runID {
				return errors.New("HOLD: authorization already dispatched; reconciliation and fresh authorization required")
			}
			if r.Attempt != 1 || r.Event != "workflow_dispatch" || r.Branch != "main" {
				return errors.New("invalid publication run identity")
			}
			current = true
		}
		if len(result.Runs) < 100 {
			if !current {
				return errors.New("publication run not independently confirmed")
			}
			return nil
		}
	}
	return errors.New("publication history bound exceeded")
}

func (g *githubPublication) verifyEvidenceOrigin(ctx context.Context, a publicationAuthorization) error {
	if a.EvidenceOrigin == "BOUND_LOCAL_COPY" {
		return g.verifyRetainedCopy(ctx, a)
	}
	var run struct {
		ID         int64  `json:"id"`
		Attempt    int64  `json:"run_attempt"`
		Head       string `json:"head_sha"`
		Path       string `json:"path"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if _, err := g.readJSON(ctx, fmt.Sprintf("/actions/runs/%d/attempts/%d", a.CandidateRunID, a.CandidateRunAttempt), &run, false); err != nil {
		return err
	}
	workflowPath, qualifier, qualified := strings.Cut(run.Path, "@")
	// The API may qualify the workflow path with @ref. The path identifies the
	// existing workflow; the separately checked full head SHA binds source bytes.
	if run.ID != a.CandidateRunID || run.Attempt != a.CandidateRunAttempt || run.Head != a.SourceCommit || workflowPath != ".github/workflows/current-version-candidate.yml" || (qualified && qualifier == "") || run.Status != "completed" || run.Conclusion != "success" || run.Repository.FullName != a.Repository {
		return errors.New("candidate run identity/status mismatch")
	}
	var artifact struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		Expired     bool   `json:"expired"`
		Expires     string `json:"expires_at"`
		WorkflowRun struct {
			ID   int64  `json:"id"`
			Head string `json:"head_sha"`
		} `json:"workflow_run"`
	}
	if _, err := g.readJSON(ctx, fmt.Sprintf("/actions/artifacts/%d", a.EvidenceArtifactID), &artifact, false); err != nil {
		return err
	}
	expires, err := time.Parse(time.RFC3339, artifact.Expires)
	expected := fmt.Sprintf("candidate-evidence-%s-%d-%d", a.SourceCommit, a.CandidateRunID, a.CandidateRunAttempt)
	if err != nil || !expires.After(time.Now()) || artifact.Expired || artifact.ID != a.EvidenceArtifactID || artifact.Name != expected || artifact.WorkflowRun.ID != a.CandidateRunID || artifact.WorkflowRun.Head != a.SourceCommit {
		return errors.New("candidate artifact unavailable/expired/unbound; no rebuild fallback")
	}
	return nil
}

// A retained copy is an explicitly authorized transport of the original bundle,
// not a replacement build or a claim that expired hosted evidence is available.
// The authorization pins this transport's artifact ID and original run/attempt;
// verifySupply separately checks the independently anchored original bytes.
func (g *githubPublication) verifyRetainedCopy(ctx context.Context, a publicationAuthorization) error {
	var artifact struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Expired bool   `json:"expired"`
		Expires string `json:"expires_at"`
	}
	if _, err := g.readJSON(ctx, fmt.Sprintf("/actions/artifacts/%d", a.EvidenceArtifactID), &artifact, false); err != nil {
		return err
	}
	expires, err := time.Parse(time.RFC3339, artifact.Expires)
	if err != nil || !expires.After(time.Now()) || artifact.Expired || artifact.ID != a.EvidenceArtifactID || artifact.Name != "retained-candidate-evidence-"+a.IndexSHA256 {
		return errors.New("explicitly bound retained copy unavailable; no fallback")
	}
	return nil
}

// Extraction creates only exclusive regular files in a new caller-owned root.
// Explicit directories, aliases, case collisions and extra inputs fail closed.
func extractPublicationEvidence(reader io.ReaderAt, size int64, destination string) error {
	if err := rejectSymlinkParents(filepath.Dir(destination)); err != nil {
		return err
	}
	if err := os.Mkdir(destination, 0o700); err != nil {
		return err
	}
	archive, err := zip.NewReader(reader, size)
	if err != nil {
		return err
	}
	if len(archive.File) > 128 {
		return errors.New("evidence entry count exceeded")
	}
	seen := map[string]bool{}
	var total uint64
	for _, entry := range archive.File {
		name, err := normalizeArchivePath(entry.Name)
		if err != nil || name != entry.Name || entry.FileInfo().IsDir() || !entry.Mode().IsRegular() || seen[strings.ToLower(name)] || entry.UncompressedSize64 > maximumSupplyArchiveSize {
			return errors.New("unsafe evidence archive entry")
		}
		seen[strings.ToLower(name)] = true
		total += entry.UncompressedSize64
		if total > 5*maximumSupplyArchiveSize {
			return errors.New("evidence expansion bound exceeded")
		}
		target := filepath.Join(destination, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		in, err := entry.Open()
		if err != nil {
			return errors.Join(err, out.Close())
		}
		n, copyErr := io.Copy(out, io.LimitReader(in, int64(entry.UncompressedSize64)+1))
		if err := errors.Join(copyErr, in.Close(), out.Close()); err != nil {
			return err
		}
		if n != int64(entry.UncompressedSize64) {
			return errors.New("evidence entry size mismatch")
		}
	}
	return nil
}

func (g *githubPublication) downloadEvidence(ctx context.Context, a publicationAuthorization, destination string) error {
	if err := g.verifyEvidenceOrigin(ctx, a); err != nil {
		return err
	}
	if err := rejectSymlinkParents(filepath.Dir(destination)); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return errors.New("new evidence directory required")
	}
	stream, err := g.download(ctx, fmt.Sprintf("/actions/artifacts/%d/zip", a.EvidenceArtifactID))
	if err != nil {
		return err
	}
	// A private, exclusive transport file outside the evidence set is retained on
	// failure for reconciliation; no automatic cleanup or alternate build.
	file, err := os.OpenFile(destination+".transport.zip", os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.Join(err, stream.Close())
	}
	defer file.Close()
	n, copyErr := io.Copy(file, io.LimitReader(stream, 5*maximumSupplyArchiveSize+1))
	if err := errors.Join(copyErr, stream.Close()); err != nil {
		return err
	}
	if n > 5*maximumSupplyArchiveSize {
		return errors.New("evidence transport exceeds bound")
	}
	if err := extractPublicationEvidence(file, n, destination); err != nil {
		return err
	}
	return verifySupply(destination, a.Version, a.SourceCommit, a.ManifestSHA256, a.IndexSHA256, nil)
}
