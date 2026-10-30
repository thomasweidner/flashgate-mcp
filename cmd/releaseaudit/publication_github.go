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
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const githubAPI = "https://api.github.com"
const githubUploads = "https://uploads.github.com"

// Only this adapter knows the provider or reads a job credential. It has no
// retry, asset replacement, tag creation or provider-fallback method.
// Asset deletion is only consumed by the separately authorized normalization path.
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

func (g *githubPublication) request(ctx context.Context, method, endpoint, accept, contentType string, body io.Reader, size int64) (*http.Response, error) {
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
		if contentType == "" {
			return nil, errors.New("provider content type missing")
		}
		req.Header.Set("Content-Type", contentType)
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
	res, err := g.request(ctx, "GET", g.endpoint(path), "application/vnd.github+json", "", nil, 0)
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
	ID          int64  `json:"id"`
	Tag         string `json:"tag_name"`
	Target      string `json:"target_commitish"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
}
type githubAsset struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	State  string `json:"state"`
	Digest string `json:"digest"`
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
	return g.inspect(ctx, plan, 0)
}

func (g *githubPublication) InspectRelease(ctx context.Context, plan publicationPlan, id int64) (remotePublicationState, error) {
	if id < 1 {
		return remotePublicationState{}, errors.New("positive release ID required")
	}
	return g.inspect(ctx, plan, id)
}

// The list still detects visible conflicting releases. It is not the authority
// for the existence of a draft whose immutable ID is already known.
func (g *githubPublication) inspect(ctx context.Context, plan publicationPlan, id int64) (remotePublicationState, error) {
	state := remotePublicationState{}
	commit, err := g.tagCommit(ctx, plan.Tag)
	if err != nil {
		return state, err
	}
	state.TagCommit = commit
	var listed *githubRelease
	for page := 1; page <= 100; page++ {
		var releases []githubRelease
		if _, err := g.readJSON(ctx, fmt.Sprintf("/releases?per_page=100&page=%d", page), &releases, false); err != nil {
			return state, err
		}
		for _, r := range releases {
			if r.Tag != plan.Tag {
				if id > 0 && r.ID == id {
					return state, errors.New("bound release has conflicting listed tag")
				}
				continue
			}
			if state.Release != nil || r.ID < 1 {
				return state, errors.New("release collision")
			}
			state.Release = &remotePublicationRelease{ID: r.ID, Tag: r.Tag, Target: r.Target, Name: r.Name, Body: r.Body, Draft: r.Draft, Prerelease: r.Prerelease, PublishedAt: r.PublishedAt}
			copy := r
			listed = &copy
		}
		if len(releases) < 100 {
			break
		}
		if page == 100 {
			return state, errors.New("release pagination bound exceeded")
		}
	}
	if id > 0 {
		var raw json.RawMessage
		var direct githubRelease
		if _, err := g.readJSON(ctx, fmt.Sprintf("/releases/%d", id), &raw, false); err != nil {
			return state, err // No fallback, create, or write follows an unavailable ID.
		}
		var flags struct {
			Draft      *bool `json:"draft"`
			Prerelease *bool `json:"prerelease"`
		}
		if json.Unmarshal(raw, &flags) != nil || flags.Draft == nil || flags.Prerelease == nil || json.Unmarshal(raw, &direct) != nil {
			return state, errors.New("direct release flags or response unavailable")
		}
		if direct.ID != id || listed != nil && *listed != direct {
			return state, errors.New("direct release identity conflicts with request or list")
		}
		state.Release = &remotePublicationRelease{ID: direct.ID, Tag: direct.Tag, Target: direct.Target, Name: direct.Name, Body: direct.Body, Draft: direct.Draft, Prerelease: direct.Prerelease, PublishedAt: direct.PublishedAt}
	}
	var latest githubRelease
	if found, err := g.readJSON(ctx, "/releases/latest", &latest, true); err != nil {
		return state, err
	} else if found {
		if latest.ID < 1 {
			return state, errors.New("latest release identity unavailable")
		}
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
	res, err := g.request(ctx, method, g.endpoint(path), "application/vnd.github+json", "application/json", bytes.NewReader(data), int64(len(data)))
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
	res, err := g.request(ctx, "POST", endpoint, "application/vnd.github+json", "application/octet-stream", reader, asset.Size)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return &publicationUploadHTTPError{statusCode: res.StatusCode}
	}
	return nil
}

// The only delete consumer is the separately authorized historical path.
// A failed/redirected/ambiguous request is consumed and is never retried here.
func (g *githubPublication) DeleteAsset(ctx context.Context, id int64) error {
	if id < 1 {
		return errors.New("positive asset ID required")
	}
	res, err := g.request(ctx, "DELETE", g.endpoint(fmt.Sprintf("/releases/assets/%d", id)), "application/vnd.github+json", "", nil, 0)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		return errors.New("provider asset delete failed or ambiguous; consumed")
	}
	return nil
}

// Redirects are read-only and credentials never cross the API host boundary.
// Returned upload URLs and provider error bodies are never trusted or logged.
type publicationDownloadClass uint8

const (
	releaseAssetDownload publicationDownloadClass = iota
	actionsArtifactDownload
)

func publicationDownloadHost(u *url.URL, class publicationDownloadClass) bool {
	// Compare the complete authority too: an empty explicit port, escaped host,
	// trailing dot, opaque URL or fragment must not acquire an allowlisted identity.
	if u.Scheme != "https" || u.Opaque != "" || u.User != nil || u.Host != u.Hostname() || u.Fragment != "" {
		return false
	}
	host := u.Hostname()
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return false
			}
		}
	}
	return host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com") || strings.HasSuffix(host, ".github.com") ||
		class == actionsArtifactDownload && strings.HasSuffix(host, ".blob.core.windows.net")
}

func (g *githubPublication) download(ctx context.Context, endpoint string, class publicationDownloadClass) (io.ReadCloser, error) {
	// A download class is not permission to broaden arbitrary API requests.
	prefix, suffix := "/releases/assets/", ""
	if class == actionsArtifactDownload {
		prefix, suffix = "/actions/artifacts/", "/zip"
	} else if class != releaseAssetDownload {
		return nil, errors.New("invalid download class")
	}
	id := strings.TrimSuffix(strings.TrimPrefix(endpoint, prefix), suffix)
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n < 1 || endpoint != prefix+strconv.FormatInt(n, 10)+suffix {
		return nil, errors.New("download endpoint/class mismatch")
	}
	// A copied client retains transport and timeout bounds but disables both
	// implicit redirects and CookieJar injection, including on API requests.
	client := *g.client
	client.Jar = nil
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	download := *g
	download.client = &client
	accept := "application/octet-stream"
	if class == actionsArtifactDownload {
		accept = "application/vnd.github+json"
	}
	res, err := download.request(ctx, "GET", g.endpoint(endpoint), accept, "", nil, 0)
	if err != nil {
		return nil, err
	}
	return publicationDownloadResponse(ctx, &client, res, class)
}

// Both initial public reads and every redirect are credential- and cookie-free.
// The URL is constructed from validated identity, never a provider-supplied URL.
func (g *githubPublication) ReadPublicAsset(ctx context.Context, plan publicationPlan, asset publicationAsset) (io.ReadCloser, error) {
	if plan.Tag != "v"+plan.Authorization.Version || validatePublicationAuthorization(plan.Authorization) != nil ||
		publicationEvidencePath(asset.Name) != nil || strings.Contains(asset.Name, "/") {
		return nil, errors.New("invalid public asset identity")
	}
	client := *g.client
	client.Jar = nil
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	endpoint := "https://github.com/" + publicationRepository + "/releases/download/" + url.PathEscape(plan.Tag) + "/" + url.PathEscape(asset.Name)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, errors.New("invalid public download request")
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New("public download transport failed")
	}
	return publicationDownloadResponse(ctx, &client, res, releaseAssetDownload)
}

func publicationDownloadResponse(ctx context.Context, client *http.Client, res *http.Response, class publicationDownloadClass) (io.ReadCloser, error) {
	redirected := false
	for redirects := 0; res.StatusCode == 302 || res.StatusCode == 301 || res.StatusCode == 307; redirects++ {
		location := res.Header.Get("Location")
		if err := res.Body.Close(); err != nil {
			return nil, errors.New("download redirect close failed")
		}
		u, err := url.Parse(location)
		if err != nil || redirects >= 3 || !publicationDownloadHost(u, class) {
			return nil, errors.New("unsafe download redirect")
		}
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, errors.New("invalid download redirect")
		}
		res, err = client.Do(req)
		if err != nil {
			return nil, errors.New("download read failed")
		}
		redirected = true
	}
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, &publicationDownloadHTTPError{statusCode: res.StatusCode, redirected: redirected}
	}
	return res.Body, nil
}
func (g *githubPublication) ReadAsset(ctx context.Context, id int64) (io.ReadCloser, error) {
	return g.download(ctx, fmt.Sprintf("/releases/assets/%d", id), releaseAssetDownload)
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

// Publication evidence has a portable ASCII inventory. Reject aliases on both
// hosts, before passing archive names to any filesystem operation. The shared
// archive normalizer remains unchanged for other consumers.
func publicationEvidencePath(name string) error {
	normal, err := normalizeArchivePath(name)
	if err != nil || normal != name || name == "" || len(name) > 1024 {
		return errors.New("noncanonical evidence path")
	}
	for _, ch := range name {
		if ch < 0x21 || ch > 0x7e || strings.ContainsRune(`\:<>"|?*`, ch) {
			return errors.New("nonportable evidence path")
		}
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasSuffix(part, ".") {
			return errors.New("ambiguous evidence path")
		}
		base, _, _ := strings.Cut(strings.ToUpper(part), ".")
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || base == "CONIN$" || base == "CONOUT$" ||
			len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
			return errors.New("reserved evidence path")
		}
	}
	return nil
}

type publicationEvidenceRoot struct {
	root, parent *os.Root
	path         string
	identity     os.FileInfo
	parents      []stageParent
	directories  map[string]os.FileInfo
}

func publicationRootDirectory(root *os.Root, name string) (os.FileInfo, error) {
	link, err := root.Lstat(name)
	if err != nil || link.Mode().Type() != os.ModeDir {
		return nil, errors.New("evidence directory is not regular")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	info, statErr := file.Stat() // freezes Windows file identity from the handle
	if err := errors.Join(statErr, file.Close()); err != nil {
		return nil, err
	}
	if !info.IsDir() || !os.SameFile(link, info) {
		return nil, errors.New("evidence directory identity changed")
	}
	return info, nil
}

func newPublicationEvidenceRoot(destination string) (_ *publicationEvidenceRoot, result error) {
	abs, err := filepath.Abs(destination)
	if err != nil {
		return nil, err
	}
	b := &publicationEvidenceRoot{path: abs, directories: map[string]os.FileInfo{}}
	for current := filepath.Dir(abs); ; current = filepath.Dir(current) {
		info, err := stageDirectory(current) // reuse the existing no-reparse identity check
		if err != nil {
			return nil, err
		}
		b.parents = append(b.parents, stageParent{current, info})
		if filepath.Dir(current) == current {
			break
		}
	}
	b.parent, err = os.OpenRoot(filepath.Dir(abs))
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, b.close())
		}
	}()
	info, err := publicationRootDirectory(b.parent, ".")
	if err != nil || !os.SameFile(b.parents[0].info, info) {
		return nil, errors.New("evidence parent binding changed")
	}
	if err := b.parent.Mkdir(filepath.Base(abs), 0o700); err != nil {
		return nil, err // existing targets, including links, are never adopted
	}
	b.identity, err = stageDirectory(abs)
	if err != nil {
		return nil, err
	}
	b.root, err = b.parent.OpenRoot(filepath.Base(abs))
	if err != nil {
		return nil, err
	}
	if err := b.check(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *publicationEvidenceRoot) close() error {
	var err error
	if b.root != nil {
		err = b.root.Close()
	}
	if b.parent != nil {
		err = errors.Join(err, b.parent.Close())
	}
	return err
}

func (b *publicationEvidenceRoot) check() error {
	for _, p := range append(append([]stageParent{}, b.parents...), stageParent{b.path, b.identity}) {
		info, err := stageDirectory(p.path)
		if err != nil || !os.SameFile(p.info, info) {
			return errors.New("evidence root/parent identity changed; recovery retained")
		}
	}
	info, err := publicationRootDirectory(b.root, ".")
	if err != nil || !os.SameFile(b.identity, info) {
		return errors.New("evidence root handle identity changed")
	}
	for name, bound := range b.directories {
		info, err := publicationRootDirectory(b.root, name)
		if err != nil || !os.SameFile(bound, info) {
			return errors.New("evidence subdirectory identity changed")
		}
	}
	return nil
}

func (b *publicationEvidenceRoot) mkdirParents(name string) error {
	parts := strings.Split(path.Dir(name), "/")
	for i := range parts {
		dir := strings.Join(parts[:i+1], "/")
		if dir == "." || b.directories[dir] != nil {
			continue
		}
		if err := b.check(); err != nil {
			return err
		}
		if err := b.root.Mkdir(filepath.FromSlash(dir), 0o700); err != nil {
			return err
		}
		info, err := publicationRootDirectory(b.root, filepath.FromSlash(dir))
		if err != nil {
			return err
		}
		b.directories[dir] = info
	}
	return b.check()
}

func copyPublicationEvidence(in io.ReadCloser, out io.WriteCloser, size uint64) error {
	n, copyErr := io.Copy(out, io.LimitReader(in, int64(size)+1))
	if err := errors.Join(copyErr, in.Close(), out.Close()); err != nil {
		return err
	}
	if n != int64(size) {
		return errors.New("evidence entry size mismatch")
	}
	return nil
}

// Descriptor-relative creation confines every write even during path exchange.
// Identity checks additionally reject drift instead of silently accepting a
// moved root or an in-root alias. Failed extractions retain their recovery data.
func extractPublicationEvidence(reader io.ReaderAt, size int64, destination string) (result error) {
	bound, err := newPublicationEvidenceRoot(destination)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, bound.close()) }()
	archive, err := zip.NewReader(reader, size)
	if err != nil {
		return err
	}
	if len(archive.File) > 128 {
		return errors.New("evidence entry count exceeded")
	}
	seen := map[string]string{}
	files := map[string]bool{}
	var total uint64
	for _, entry := range archive.File {
		name := entry.Name
		if publicationEvidencePath(name) != nil || !entry.Mode().IsRegular() || entry.UncompressedSize64 > maximumSupplyArchiveSize {
			return errors.New("unsafe evidence archive entry")
		}
		key := strings.ToLower(name)
		if seen[key] != "" {
			return errors.New("duplicate or file/directory evidence collision")
		}
		seen[key], files[key] = name, true
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			key := strings.ToLower(dir)
			if files[key] || seen[key] != "" && seen[key] != dir {
				return errors.New("aliased evidence directory")
			}
			seen[key] = dir
		}
		total += entry.UncompressedSize64
		if total > 5*maximumSupplyArchiveSize {
			return errors.New("evidence expansion bound exceeded")
		}
	}
	for _, entry := range archive.File {
		if err := bound.mkdirParents(entry.Name); err != nil {
			return err
		}
		out, err := bound.root.OpenFile(filepath.FromSlash(entry.Name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		in, err := entry.Open()
		if err != nil {
			return errors.Join(err, out.Close())
		}
		info, statErr := out.Stat()
		if statErr != nil || !info.Mode().IsRegular() {
			return errors.Join(errors.New("nonregular evidence output"), statErr, in.Close(), out.Close())
		}
		if err := copyPublicationEvidence(in, out, entry.UncompressedSize64); err != nil {
			return err
		}
		if err := bound.check(); err != nil {
			return err
		}
	}
	return nil
}

func (g *githubPublication) downloadEvidence(ctx context.Context, a publicationAuthorization, destination string) (result error) {
	if err := g.verifyEvidenceOrigin(ctx, a); err != nil {
		return err
	}
	if err := rejectSymlinkParents(filepath.Dir(destination)); err != nil {
		return err
	}
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return errors.New("new evidence directory required")
	}
	stream, err := g.download(ctx, fmt.Sprintf("/actions/artifacts/%d/zip", a.EvidenceArtifactID), actionsArtifactDownload)
	if err != nil {
		return err
	}
	// A private, exclusive transport file outside the evidence set is retained on
	// failure for reconciliation; no automatic cleanup or alternate build.
	file, err := os.OpenFile(destination+".transport.zip", os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.Join(err, stream.Close())
	}
	defer func() { result = errors.Join(result, file.Close()) }()
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
