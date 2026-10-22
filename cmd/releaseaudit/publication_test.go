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
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func publicationFixture(t *testing.T) (string, publicationPlan) {
	t.Helper()
	dir, mh, ih, m := supplyFixture(t)
	changelog := []byte("# Changelog\n\n## [Unreleased]\n\n## [" + m.Version + "] - 2026-10-20\n\n### Added\n\n- Release test fixture.\n")
	_, notes, err := auditChangelog(string(changelog), m.Version, true, false)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := publicationAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := supplyJSON(assets)
	a := publicationAuthorization{Schema: "flashgate-publication-authorization/v1", Action: "CREATE_DRAFT_UPLOAD_READBACK_PUBLISH_PRERELEASE", Repository: publicationRepository, Version: m.Version, SourceCommit: m.SourceCommitSHA, ManifestSHA256: mh, IndexSHA256: ih, ChangelogSHA256: supplyDigest(changelog), NotesSHA256: supplyDigest([]byte(notes)), AssetsSHA256: supplyDigest(data), CandidateRunID: 123, CandidateRunAttempt: 2, EvidenceArtifactID: 456, PreState: "RELEASE_ABSENT", OriginalApproval: "fixture-human-approval"}
	a.EvidenceOrigin = "ACTIONS_ARTIFACT"
	auth, _ := supplyJSON(a)
	return dir, publicationPlan{Schema: "flashgate-publication-manifest/v1", Authorization: a, AuthorizationSHA256: supplyDigest(auth), Tag: "v" + m.Version, Notes: notes, Assets: assets}
}

// Context and metadata unit tests need valid identities and digest bindings,
// not a verified four-target bundle. These synthetic bytes never authorize an
// actual publication; the end-to-end tests retain publicationFixture.
func publicationUnitPlan(t *testing.T) publicationPlan {
	t.Helper()
	encode := func(value any) []byte {
		data, err := supplyJSON(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	const value = "0.3.0"
	manifest := encode(map[string]string{"fixture": "publication unit candidate", "version": value, "sourceCommitSha": testCandidateCommit})
	index := encode(map[string]any{"fixture": "publication unit index", "version": value, "sourceCommitSha": testCandidateCommit,
		"candidateManifest": supplyFile{"candidate/candidate-manifest.json", supplyDigest(manifest)}})
	assets := []publicationAsset{
		{"candidate-manifest.json", "candidate/candidate-manifest.json", int64(len(manifest)), supplyDigest(manifest)},
		{supplyIndexName, supplyIndexName, int64(len(index)), supplyDigest(index)},
	}
	changelog := []byte("# Changelog\n\n## [Unreleased]\n\n## [" + value + "] - 2026-10-20\n\n### Added\n\n- Release test fixture.\n")
	_, notes, err := auditChangelog(string(changelog), value, true, false)
	if err != nil {
		t.Fatal(err)
	}
	a := publicationAuthorization{
		Schema: "flashgate-publication-authorization/v1", Action: "CREATE_DRAFT_UPLOAD_READBACK_PUBLISH_PRERELEASE",
		Repository: publicationRepository, Version: value, SourceCommit: testCandidateCommit,
		ManifestSHA256: supplyDigest(manifest), IndexSHA256: supplyDigest(index),
		ChangelogSHA256: supplyDigest(changelog), NotesSHA256: supplyDigest([]byte(notes)), AssetsSHA256: supplyDigest(encode(assets)),
		CandidateRunID: 123, CandidateRunAttempt: 2, EvidenceArtifactID: 456, EvidenceOrigin: "ACTIONS_ARTIFACT",
		PreState: "RELEASE_ABSENT", OriginalApproval: "fixture-human-approval",
	}
	if err := validatePublicationAuthorization(a); err != nil {
		t.Fatal("invalid unit authorization:", err)
	}
	return publicationPlan{Schema: "flashgate-publication-manifest/v1", Authorization: a,
		AuthorizationSHA256: supplyDigest(encode(a)), Tag: "v" + value, Notes: notes, Assets: assets}
}

func TestPublicationUnitPlan(t *testing.T) {
	plan := publicationUnitPlan(t)
	auth, err := supplyJSON(plan.Authorization)
	if err != nil || supplyDigest(auth) != plan.AuthorizationSHA256 {
		t.Fatal("unit plan authorization is not bound to its actual bytes", err)
	}
	assets, err := supplyJSON(plan.Assets)
	if err != nil || supplyDigest(assets) != plan.Authorization.AssetsSHA256 {
		t.Fatal("unit plan asset inventory is not bound", err)
	}
	// An authorization-shaped unit plan must still fail the real evidence gate
	// before any provider write. It cannot stand in for a verified bundle.
	mock := newPublicationMock(plan)
	if err := executePublication(context.Background(), mock, plan, t.TempDir(), filepath.Join(t.TempDir(), "attempt.jsonl")); err == nil || mock.writes != 0 {
		t.Fatal("synthetic unit metadata bypassed real evidence preflight", err)
	}
}

type publicationMock struct {
	state     remotePublicationState
	data      map[int64][]byte
	writes    int
	failWrite int
	failRead  bool
	corrupt   bool
	stable    bool
	finalFail bool
}

func newPublicationMock(plan publicationPlan) *publicationMock {
	return &publicationMock{state: remotePublicationState{TagCommit: plan.Authorization.SourceCommit}, data: map[int64][]byte{}}
}
func (m *publicationMock) Inspect(_ context.Context, _ publicationPlan) (remotePublicationState, error) {
	if m.failRead || (m.finalFail && m.state.Release != nil && !m.state.Release.Draft) {
		return remotePublicationState{}, errors.New("controlled read failure")
	}
	return m.state, nil
}
func (m *publicationMock) ReadAsset(_ context.Context, id int64) (io.ReadCloser, error) {
	data := m.data[id]
	if m.corrupt {
		data = append(append([]byte{}, data...), 1)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (m *publicationMock) write() error {
	m.writes++
	if m.writes == m.failWrite {
		return errors.New("controlled ambiguous write")
	}
	return nil
}
func (m *publicationMock) CreateDraft(_ context.Context, p publicationPlan) (int64, error) {
	if err := m.write(); err != nil {
		return 0, err
	}
	m.state.Release = &remotePublicationRelease{ID: 789, Tag: p.Tag, Target: p.Authorization.SourceCommit, Name: "FlashGate MCP " + p.Authorization.Version, Body: p.Notes, Draft: true, Prerelease: !m.stable}
	return 789, nil
}
func (m *publicationMock) Upload(_ context.Context, _ int64, a publicationAsset, r io.Reader) error {
	if err := m.write(); err != nil {
		return err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	id := int64(len(m.data) + 1)
	m.data[id] = data
	m.state.Release.Assets = append(m.state.Release.Assets, remotePublicationAsset{id, a.Name, int64(len(data)), "uploaded"})
	return nil
}
func (m *publicationMock) Publish(_ context.Context, _ int64, _ publicationPlan) error {
	if err := m.write(); err != nil {
		return err
	}
	m.state.Release.Draft = false
	return nil
}

func TestPublicationExactByteSequence(t *testing.T) {
	dir, plan := publicationFixture(t)
	mock := newPublicationMock(plan)
	journal := filepath.Join(t.TempDir(), "attempt.jsonl")
	if err := executePublication(context.Background(), mock, plan, dir, journal); err != nil {
		t.Fatal(err)
	}
	if mock.state.Release.Draft || !mock.state.Release.Prerelease || mock.writes != len(plan.Assets)+3 {
		t.Fatal("publication sequence mismatch")
	}
	for _, a := range plan.Assets {
		original, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(a.Path)))
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, r := range mock.state.Release.Assets {
			if r.Name == a.Name {
				found = true
				if !bytes.Equal(original, mock.data[r.ID]) {
					t.Fatal("asset bytes changed")
				}
			}
		}
		if !found {
			t.Fatal("asset missing")
		}
	}
	journalBytes, err := os.ReadFile(journal)
	if err != nil || !bytes.Contains(journalBytes, []byte("VERIFIED_PUBLIC_PRERELEASE")) {
		t.Fatal("final journal missing", err)
	}
	if bytes.Contains(journalBytes, []byte("fixture-human-approval")) {
		t.Fatal("journal copied approval content")
	}
	before := mock.writes
	if err := executePublication(context.Background(), mock, plan, dir, journal); err == nil || mock.writes != before {
		t.Fatal("consumed execution repeated")
	}
	// Even if the provider prestate is reset, the local journal is one-shot.
	mock.state.Release = nil
	if err := executePublication(context.Background(), mock, plan, dir, journal); err == nil || mock.writes != before {
		t.Fatal("consumed journal reused")
	}
}

func TestPublicationEveryWriteFailureHoldsWithoutRetry(t *testing.T) {
	dir, plan := publicationFixture(t)
	for fail := 1; fail <= len(plan.Assets)+3; fail++ {
		t.Run(strconv.Itoa(fail), func(t *testing.T) {
			mock := newPublicationMock(plan)
			mock.failWrite = fail
			err := executePublication(context.Background(), mock, plan, dir, filepath.Join(t.TempDir(), "attempt.jsonl"))
			if err == nil || mock.writes != fail {
				t.Fatalf("write %d retried/continued: %d %v", fail, mock.writes, err)
			}
			if mock.state.Release != nil && !mock.state.Release.Draft {
				t.Fatal("failed write published")
			}
		})
	}
}

func TestPublicationReadbackAndPrestateNegatives(t *testing.T) {
	dir, plan := publicationFixture(t)
	for _, name := range []string{"tag", "existing-release", "read-failure", "corrupt-upload", "stable", "final-read-failure"} {
		t.Run(name, func(t *testing.T) {
			m := newPublicationMock(plan)
			switch name {
			case "tag":
				m.state.TagCommit = strings.Repeat("a", 40)
			case "existing-release":
				m.state.Release = &remotePublicationRelease{ID: 1}
			case "read-failure":
				m.failRead = true
			case "corrupt-upload":
				m.corrupt = true
			case "stable":
				m.stable = true
			case "final-read-failure":
				m.finalFail = true
			}
			if err := executePublication(context.Background(), m, plan, dir, filepath.Join(t.TempDir(), "attempt.jsonl")); err == nil {
				t.Fatal("unsafe publication accepted")
			}
			if name == "tag" || name == "existing-release" || name == "read-failure" {
				if m.writes != 0 {
					t.Fatal("preflight failure wrote")
				}
			}
			if name == "final-read-failure" && m.state.Release.Draft {
				t.Fatal("fixture did not reach final readback boundary")
			}
		})
	}
	// Missing, extra, duplicate, wrong-size, replaced and same-size changed bytes
	// are checked from actual downloads, not provider digest assertions.
	m := newPublicationMock(plan)
	_, _ = m.CreateDraft(context.Background(), plan)
	a := plan.Assets[0]
	data, _ := os.ReadFile(filepath.Join(dir, filepath.FromSlash(a.Path)))
	if err := m.Upload(context.Background(), 789, a, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(){func() { m.state.Release.Assets = nil }, func() { m.state.Release.Assets = append(m.state.Release.Assets, m.state.Release.Assets[0]) }, func() { m.state.Release.Assets[0].Name = "other" }, func() { m.state.Release.Assets[0].Size++ }, func() { m.data[1][0] ^= 1 }, func() { m.state.LatestID = 789 }, func() { m.state.Release.Body += "changed" }, func() { m.state.Release.Target = "main" }} {
		saved := *m.state.Release
		saved.Assets = append([]remotePublicationAsset{}, saved.Assets...)
		savedData := append([]byte{}, m.data[1]...)
		mutate()
		if err := publicationReadback(context.Background(), m, plan, 789, true, []publicationAsset{a}); err == nil {
			t.Fatal("readback drift accepted")
		}
		m.state.Release = &saved
		m.data[1] = savedData
		m.state.LatestID = 0
	}
}

func TestPublicationAuthorizationAndNotes(t *testing.T) {
	p := publicationUnitPlan(t)
	a := p.Authorization
	data, _ := supplyJSON(a)
	file := filepath.Join(t.TempDir(), "authorization.json")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if actual, err := readPublicationAuthorization(file, supplyDigest(data)); err != nil || !reflect.DeepEqual(actual, a) {
		t.Fatal("valid external authorization rejected", err)
	}
	for _, anchor := range []string{"", strings.Repeat("0", 64)} {
		if _, err := readPublicationAuthorization(file, anchor); err == nil {
			t.Fatal("unbound authorization accepted")
		}
	}
	for _, mutate := range []func(*publicationAuthorization){func(a *publicationAuthorization) { a.Repository = "other/repo" }, func(a *publicationAuthorization) { a.PreState = "DRAFT_PRESENT" }, func(a *publicationAuthorization) { a.Version = "1.0.0" }, func(a *publicationAuthorization) { a.Version = "0.3.0-rc.01" }, func(a *publicationAuthorization) { a.ManifestSHA256 = "" }, func(a *publicationAuthorization) { a.OriginalApproval = "" }, func(a *publicationAuthorization) { a.CandidateRunAttempt = 0 }, func(a *publicationAuthorization) { a.Action = "REPLACE_ASSET" }} {
		bad := a
		mutate(&bad)
		if validatePublicationAuthorization(bad) == nil {
			t.Fatal("bad authorization accepted", bad)
		}
	}
	changelog := []byte("# Changelog\n\n## [Unreleased]\n\n## [" + a.Version + "] - 2026-10-20\n\n### Added\n\n- Release test fixture.\n")
	if notes, err := publicationNotes([]byte(a.Version+"\n"), changelog, a); err != nil || notes != p.Notes {
		t.Fatal("valid committed notes rejected", err)
	}
	for _, kind := range []string{"undated", "missing", "empty", "invalid-date", "changed", "suffix", "invalid-utf8"} {
		t.Run(kind, func(t *testing.T) {
			bad := append([]byte{}, changelog...)
			value := a.Version
			a2 := a
			switch kind {
			case "undated":
				bad = bytes.ReplaceAll(bad, []byte(" - 2026-10-20"), nil)
			case "missing":
				bad = []byte("## [Unreleased]\n")
			case "empty":
				bad = bytes.ReplaceAll(bad, []byte("- Release test fixture."), nil)
			case "invalid-date":
				bad = bytes.ReplaceAll(bad, []byte("2026-10-20"), []byte("2026-02-30"))
			case "changed":
				bad = append(bad, ' ')
			case "suffix":
				value += "-rc.1"
			case "invalid-utf8":
				bad = append(bad, 0xff)
			}
			if kind != "changed" {
				a2.ChangelogSHA256 = supplyDigest(bad)
			}
			if _, err := publicationNotes([]byte(value+"\n"), bad, a2); err == nil {
				t.Fatal("bad frozen notes accepted")
			}
		})
	}
}

func TestPublicationCommittedSourceBinding(t *testing.T) {
	p := publicationUnitPlan(t)
	a := p.Authorization
	dir := t.TempDir()
	changelog := []byte("# Changelog\n\n## [Unreleased]\n\n## [" + a.Version + "] - 2026-10-20\n\n### Added\n\n- Release test fixture.\n")
	for name, data := range map[string][]byte{"VERSION": []byte(a.Version + "\n"), "CHANGELOG.md": changelog} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git := func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "rev-parse":
			return []byte(a.SourceCommit + "\n"), nil
		case "status":
			return nil, nil
		case "remote":
			return []byte(projectURI + ".git\n"), nil
		case "cat-file", "show":
			name := strings.SplitN(args[len(args)-1], ":", 2)[1]
			data, err := os.ReadFile(filepath.Join(dir, name))
			if args[0] == "cat-file" {
				return []byte(strconv.Itoa(len(data))), err
			}
			return data, err
		}
		return nil, errors.New("unexpected git operation")
	}
	if _, err := publicationSourceWithGit(dir, a, git); err != nil {
		t.Fatal(err)
	}
	for _, failure := range []string{"rev-parse", "status", "remote", "cat-file", "show"} {
		t.Run(failure, func(t *testing.T) {
			bad := func(root string, args ...string) ([]byte, error) {
				if args[0] == failure {
					return []byte("drift"), nil
				}
				return git(root, args...)
			}
			if _, err := publicationSourceWithGit(dir, a, bad); err == nil {
				t.Fatal("source drift accepted")
			}
		})
	}
	badTag := func(root string, args ...string) ([]byte, error) {
		if len(args) > 1 && strings.HasPrefix(args[1], "refs/tags/") {
			return []byte(strings.Repeat("a", 40)), nil
		}
		return git(root, args...)
	}
	if _, err := publicationSourceWithGit(dir, a, badTag); err == nil {
		t.Fatal("wrong peeled tag accepted")
	}
	if _, err := preparePublication(dir, dir, a, p.AuthorizationSHA256); err == nil {
		t.Fatal("missing supply-chain bundle accepted")
	}
}

func TestPublicationEvidenceExtraction(t *testing.T) {
	for _, kind := range []string{"valid", "traversal", "case-collision", "duplicate", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			var b bytes.Buffer
			w := zip.NewWriter(&b)
			name := "candidate/test.json"
			if kind == "traversal" {
				name = "../escape"
			}
			header := &zip.FileHeader{Name: name}
			header.SetMode(0o600)
			if kind == "symlink" {
				header.SetMode(os.ModeSymlink | 0o777)
			}
			if kind == "directory" {
				header.Name = "candidate/"
				header.SetMode(os.ModeDir | 0o700)
			}
			entry, err := w.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = entry.Write([]byte("{}"))
			if kind == "case-collision" || kind == "duplicate" {
				if kind == "case-collision" {
					header.Name = strings.ToUpper(name)
				}
				copyHeader := *header
				entry, err = w.CreateHeader(&copyHeader)
				if err != nil {
					t.Fatal(err)
				}
				_, _ = entry.Write([]byte("{}"))
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			err = extractPublicationEvidence(bytes.NewReader(b.Bytes()), int64(b.Len()), filepath.Join(t.TempDir(), "evidence"))
			if (err == nil) != (kind == "valid") {
				t.Fatalf("extraction %s: %v", kind, err)
			}
		})
	}
}

func TestPublicationGitHubAdapter(t *testing.T) {
	dir, plan := publicationFixture(t)
	mock := newPublicationMock(plan)
	writeRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-only" {
			t.Error("missing job credential")
		}
		path := strings.TrimPrefix(r.URL.Path, "/repos/"+publicationRepository)
		var response any
		switch {
		case path == "/git/ref/tags/"+plan.Tag:
			response = map[string]any{"ref": "refs/tags/" + plan.Tag, "object": githubObject{strings.Repeat("b", 40), "tag"}}
		case path == "/git/tags/"+strings.Repeat("b", 40):
			response = map[string]any{"sha": strings.Repeat("b", 40), "object": githubObject{plan.Authorization.SourceCommit, "commit"}}
		case path == "/releases/latest":
			http.NotFound(w, r)
			return
		case r.Method == "GET" && path == "/releases":
			releases := []githubRelease{}
			if m := mock.state.Release; m != nil {
				releases = append(releases, githubRelease{ID: m.ID, Tag: m.Tag, Target: m.Target, Name: m.Name, Body: m.Body, Draft: m.Draft, Prerelease: m.Prerelease})
			}
			response = releases
		case r.Method == "GET" && path == "/releases/789/assets":
			assets := []githubAsset{}
			for _, a := range mock.state.Release.Assets {
				assets = append(assets, githubAsset(a))
			}
			response = assets
		case r.Method == "GET" && strings.HasPrefix(path, "/releases/assets/"):
			id, _ := strconv.ParseInt(strings.TrimPrefix(path, "/releases/assets/"), 10, 64)
			_, _ = w.Write(mock.data[id])
			return
		case r.Method == "POST" && path == "/releases":
			writeRequests++
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if payload["draft"] != true || payload["prerelease"] != true || payload["make_latest"] != "false" {
				t.Error("unsafe draft payload")
			}
			id, _ := mock.CreateDraft(r.Context(), plan)
			w.WriteHeader(201)
			response = map[string]any{"id": id}
		case r.Method == "POST" && path == "/releases/789/assets":
			writeRequests++
			name := r.URL.Query().Get("name")
			if err := mock.Upload(r.Context(), 789, publicationAsset{Name: name}, r.Body); err != nil {
				t.Error(err)
			}
			w.WriteHeader(201)
			response = map[string]any{"id": len(mock.data)}
		case r.Method == "PATCH" && path == "/releases/789":
			writeRequests++
			var payload map[string]any
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["draft"] != false || payload["prerelease"] != true || payload["make_latest"] != "false" {
				t.Error("unsafe publication payload")
			}
			_ = mock.Publish(r.Context(), 789, plan)
			response = map[string]any{"id": 789}
		default:
			t.Errorf("unexpected provider call: %s %s", r.Method, path)
			http.Error(w, "controlled", 500)
			return
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	g := &githubPublication{client: server.Client(), token: "test-only", api: server.URL, uploads: server.URL}
	if err := executePublication(context.Background(), g, plan, dir, filepath.Join(t.TempDir(), "attempt.jsonl")); err != nil {
		t.Fatal(err)
	}
	if writeRequests != len(plan.Assets)+3 {
		t.Fatal("provider write count mismatch")
	}
}

func TestPublicationGitHubNoRetryAndContext(t *testing.T) {
	plan := publicationUnitPlan(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Location", "/fallback")
		w.WriteHeader(307)
	}))
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	g := &githubPublication{client: client, token: "test-only", api: server.URL, uploads: server.URL}
	if _, err := g.CreateDraft(context.Background(), plan); err == nil || calls != 1 {
		t.Fatal("write redirect retried", calls, err)
	}
	if _, err := newGitHubPublication(plan.Authorization, plan.AuthorizationSHA256); err == nil {
		t.Fatal("local provider operation enabled")
	}
	env := map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_REPOSITORY": publicationRepository, "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_REF": "refs/heads/main", "GITHUB_RUN_ATTEMPT": "1", "GITHUB_RUN_ID": "999", "GH_TOKEN": "test-only", "FLASHGATE_AUTHORIZATION_SHA256": plan.AuthorizationSHA256, "GITHUB_WORKFLOW_REF": publicationRepository + "/.github/workflows/publish-prerelease.yml@refs/heads/main"}
	for k, v := range env {
		t.Setenv(k, v)
	}
	if _, err := newGitHubPublication(plan.Authorization, plan.AuthorizationSHA256); err != nil {
		t.Fatal(err)
	}
	for k := range env {
		before := os.Getenv(k)
		t.Setenv(k, "invalid")
		if k == "GH_TOKEN" {
			t.Setenv(k, "")
		}
		if _, err := newGitHubPublication(plan.Authorization, plan.AuthorizationSHA256); err == nil {
			t.Fatal("unsafe context accepted", k)
		}
		t.Setenv(k, before)
	}
}

func TestPublicationEvidenceOriginAndConsumedAuthorization(t *testing.T) {
	plan := publicationUnitPlan(t)
	a := plan.Authorization
	for _, kind := range []string{"valid", "unqualified", "wrong-workflow", "empty-qualifier", "expired", "wrong-run", "wrong-attempt", "wrong-subject", "failed-run", "consumed", "rerun", "missing-current"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var result any
				if strings.Contains(r.URL.Path, "/workflows/") {
					id := int64(999)
					attempt := 1
					if kind == "consumed" {
						id = 998
					}
					if kind == "rerun" {
						attempt = 2
					}
					runs := []any{}
					if kind != "missing-current" {
						runs = append(runs, map[string]any{"id": id, "run_attempt": attempt, "display_title": "BL-363 publication " + a.OriginalApproval, "event": "workflow_dispatch", "head_branch": "main"})
					}
					result = map[string]any{"workflow_runs": runs}
				} else if strings.Contains(r.URL.Path, "/attempts/") {
					commit := a.SourceCommit
					conclusion := "success"
					attempt := a.CandidateRunAttempt
					if kind == "wrong-subject" {
						commit = strings.Repeat("a", 40)
					}
					if kind == "failed-run" {
						conclusion = "failure"
					}
					if kind == "wrong-attempt" {
						attempt++
					}
					path := ".github/workflows/current-version-candidate.yml@main"
					if kind == "unqualified" {
						path = ".github/workflows/current-version-candidate.yml"
					}
					if kind == "wrong-workflow" {
						path = ".github/workflows/release-build.yml@main"
					}
					if kind == "empty-qualifier" {
						path = ".github/workflows/current-version-candidate.yml@"
					}
					result = map[string]any{"id": a.CandidateRunID, "run_attempt": attempt, "head_sha": commit, "path": path, "status": "completed", "conclusion": conclusion, "repository": map[string]any{"full_name": publicationRepository}}
				} else {
					run := a.CandidateRunID
					if kind == "wrong-run" {
						run++
					}
					result = map[string]any{"id": a.EvidenceArtifactID, "name": fmt.Sprintf("candidate-evidence-%s-%d-%d", a.SourceCommit, a.CandidateRunID, a.CandidateRunAttempt), "expired": kind == "expired", "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339), "workflow_run": map[string]any{"id": run, "head_sha": a.SourceCommit}}
				}
				_ = json.NewEncoder(w).Encode(result)
			}))
			defer server.Close()
			g := &githubPublication{client: server.Client(), token: "test-only", runID: 999, api: server.URL}
			err := g.checkAttempt(context.Background(), a.OriginalApproval)
			if kind == "consumed" || kind == "rerun" || kind == "missing-current" {
				if err == nil {
					t.Fatal("consumed/unknown attempt accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			err = g.verifyEvidenceOrigin(context.Background(), a)
			if (err == nil) != (kind == "valid" || kind == "unqualified") {
				t.Fatalf("origin %s: %v", kind, err)
			}
		})
	}
}

func TestPublicationAnchoredPreflightAndInventory(t *testing.T) {
	dir, plan := publicationFixture(t)
	source := func(_ string, a publicationAuthorization) (string, error) {
		if a.SourceCommit != plan.Authorization.SourceCommit {
			return "", errors.New("wrong source")
		}
		return plan.Notes, nil
	}
	actual, err := preparePublicationWithSource("source", dir, plan.Authorization, plan.AuthorizationSHA256, source)
	if err != nil || !reflect.DeepEqual(actual, plan) {
		t.Fatal("complete anchored candidate rejected", err)
	}
	bad := plan.Authorization
	bad.AssetsSHA256 = strings.Repeat("0", 64)
	if _, err := preparePublicationWithSource("source", dir, bad, plan.AuthorizationSHA256, source); err == nil {
		t.Fatal("unapproved inventory accepted")
	}
	output := filepath.Join(t.TempDir(), "assets.json")
	args := []string{"--dir", dir, "--version", plan.Authorization.Version, "--commit", plan.Authorization.SourceCommit, "--manifest-sha256", plan.Authorization.ManifestSHA256, "--index-sha256", plan.Authorization.IndexSHA256, "--output", output}
	if err := runPublicationInventory(args); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil || supplyDigest(data) != plan.Authorization.AssetsSHA256 {
		t.Fatal("inventory projection mismatch", err)
	}
	if runPublicationInventory(args) == nil {
		t.Fatal("inventory output replaced")
	}
	for _, args := range [][]string{nil, {"--unknown"}, {"--dir", dir, "extra"}} {
		if runPublicationInventory(args) == nil {
			t.Fatal("invalid inventory arguments accepted")
		}
		if runPublication(args, "preflight") == nil {
			t.Fatal("invalid publication arguments accepted")
		}
	}
	// The same strict parser as BL-255 rejects unknown, duplicate and null fields.
	auth, _ := supplyJSON(plan.Authorization)
	for _, data := range [][]byte{bytes.Replace(auth, []byte("{\n"), []byte("{\n\"unknown\":true,"), 1), bytes.Replace(auth, []byte("{\n"), []byte("{\n\"schema\":\"duplicate\","), 1), []byte("null"), append(auth, []byte("{}")...)} {
		file := filepath.Join(t.TempDir(), "authorization.json")
		if err := os.WriteFile(file, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readPublicationAuthorization(file, supplyDigest(data)); err == nil {
			t.Fatal("invalid authorization schema accepted")
		}
	}
}

type publicationTransport func(*http.Request) (*http.Response, error)

func (f publicationTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestPublicationReadRedirectDropsCredential(t *testing.T) {
	calls := 0
	g := &githubPublication{token: "test-only", api: githubAPI, uploads: githubUploads}
	g.client = &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }, Transport: publicationTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			if r.Header.Get("Authorization") != "Bearer test-only" {
				t.Fatal("fixture API credential missing")
			}
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://release-assets.githubusercontent.com/fixture"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		}
		if r.URL.Hostname() != "release-assets.githubusercontent.com" || r.Header.Get("Authorization") != "" {
			t.Fatal("credential crossed download boundary")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("exact bytes")), Request: r}, nil
	})}
	stream, err := g.ReadAsset(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	_ = stream.Close()
	if string(data) != "exact bytes" || calls != 2 {
		t.Fatal("download read mismatch")
	}
	for _, location := range []string{"http://release-assets.githubusercontent.com/fixture", "https://untrusted.example/fixture", "https://github.com:443/fixture", "https://user@github.com/fixture"} {
		g.client.Transport = publicationTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{location}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		})
		if _, err := g.ReadAsset(context.Background(), 1); err == nil {
			t.Fatal("unsafe read redirect accepted", location)
		}
	}
}

func TestPublicationExplicitRetainedCopy(t *testing.T) {
	plan := publicationUnitPlan(t)
	a := plan.Authorization
	a.EvidenceOrigin = "BOUND_LOCAL_COPY"
	for _, kind := range []string{"valid", "wrong-id", "wrong-name", "expired"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.Contains(r.URL.Path, "/actions/artifacts/") {
					t.Error("retained copy substituted an original run query")
				}
				id := a.EvidenceArtifactID
				name := "retained-candidate-evidence-" + a.IndexSHA256
				if kind == "wrong-id" {
					id++
				}
				if kind == "wrong-name" {
					name = "unbound"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "name": name, "expired": kind == "expired", "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
			}))
			defer server.Close()
			g := &githubPublication{client: server.Client(), token: "test-only", api: server.URL}
			err := g.verifyEvidenceOrigin(context.Background(), a)
			if (err == nil) != (kind == "valid") {
				t.Fatalf("retained copy %s: %v", kind, err)
			}
		})
	}
}

func TestPublicationDownloadCompleteOriginalBundle(t *testing.T) {
	dir, plan := publicationFixture(t)
	a := plan.Authorization
	a.EvidenceOrigin = "BOUND_LOCAL_COPY"
	var bundle bytes.Buffer
	writer := zip.NewWriter(&bundle)
	for _, asset := range plan.Assets {
		entry, err := writer.Create(asset.Path)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(asset.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/zip") {
			_, _ = w.Write(bundle.Bytes())
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": a.EvidenceArtifactID, "name": "retained-candidate-evidence-" + a.IndexSHA256, "expired": false, "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
	}))
	defer server.Close()
	g := &githubPublication{client: server.Client(), token: "test-only", api: server.URL}
	destination := filepath.Join(t.TempDir(), "evidence")
	if err := g.downloadEvidence(context.Background(), a, destination); err != nil {
		t.Fatal(err)
	}
	actual, err := publicationAssets(destination)
	if err != nil || !reflect.DeepEqual(actual, plan.Assets) {
		t.Fatal("download changed original bundle", err)
	}
	if err := g.downloadEvidence(context.Background(), a, destination); err == nil {
		t.Fatal("download overwrote existing evidence")
	}
}

func TestPublicationProviderInvalidStates(t *testing.T) {
	for _, body := range []string{"null", `{"id":1,"id":2}`, `{} {}`, `not json`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			g := &githubPublication{client: server.Client(), token: "test-only", api: server.URL}
			var state map[string]any
			if _, err := g.readJSON(context.Background(), "/releases", &state, false); err == nil {
				t.Fatal("invalid provider state accepted")
			}
		})
	}
	var output publicationGitOutput
	if _, err := output.Write(make([]byte, maximumChangelogSize+1)); err == nil {
		t.Fatal("unbounded Git stdout accepted")
	}
	p := publicationUnitPlan(t)
	changelog := []byte("## [Unreleased]\n\n## [" + p.Authorization.Version + "] - 2026-10-20\n\n- https://user:password@example.invalid/path\n")
	a := p.Authorization
	a.ChangelogSHA256 = supplyDigest(changelog)
	_, notes, err := auditChangelog(string(changelog), a.Version, true, false)
	if err != nil {
		t.Fatal(err)
	}
	a.NotesSHA256 = supplyDigest([]byte(notes))
	if _, err := publicationNotes([]byte(a.Version), changelog, a); err == nil {
		t.Fatal("credential-bearing release notes accepted")
	}
}
