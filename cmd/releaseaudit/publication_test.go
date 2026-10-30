package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"syscall"
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
func (m *publicationMock) InspectRelease(ctx context.Context, plan publicationPlan, _ int64) (remotePublicationState, error) {
	return m.Inspect(ctx, plan)
}
func (m *publicationMock) ReadAsset(_ context.Context, id int64) (io.ReadCloser, error) {
	data := m.data[id]
	if m.corrupt {
		data = append(append([]byte{}, data...), 1)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (m *publicationMock) ReadPublicAsset(ctx context.Context, _ publicationPlan, asset publicationAsset) (io.ReadCloser, error) {
	for _, remote := range m.state.Release.Assets {
		if remote.Name == asset.Name {
			return m.ReadAsset(ctx, remote.ID)
		}
	}
	return nil, errors.New("public asset absent")
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
	m.state.Release.Assets = append(m.state.Release.Assets, remotePublicationAsset{id, a.Name, int64(len(data)), "uploaded", "sha256:" + supplyDigest(data)})
	return nil
}
func (m *publicationMock) Publish(_ context.Context, _ int64, _ publicationPlan) error {
	if err := m.write(); err != nil {
		return err
	}
	m.state.Release.Draft = false
	m.state.Release.PublishedAt = "2026-10-25T00:00:00Z"
	return nil
}

func TestPublicationExactByteSequence(t *testing.T) {
	dir, plan := publicationFixture(t)
	mock := newPublicationMock(plan)
	journal := filepath.Join(t.TempDir(), "attempt.jsonl")
	if err := executePublication(context.Background(), mock, plan, dir, journal); err != nil {
		t.Fatal(err)
	}
	if mock.state.Release.Draft || !mock.state.Release.Prerelease || mock.writes != len(plan.Assets)+2 {
		t.Fatal("publication sequence mismatch")
	}
	for _, a := range plan.Assets {
		original, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(a.Path)))
		if a.Name == publicationChecksumsName {
			original, err = publicationGeneratedChecksums(plan.Assets)
		}
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
	for fail := 1; fail <= len(plan.Assets)+2; fail++ {
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
	a := plan.Assets[1]
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

func publicationParityDiagnosticJSON(t *testing.T, err error) (map[string]json.RawMessage, []string) {
	t.Helper()
	const prefix = "remote release parity failed: "
	if err == nil || !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("missing parity diagnostic: %v", err)
	}
	if len(err.Error()) > 4096 {
		t.Fatal("unbounded parity diagnostic")
	}
	var value map[string]json.RawMessage
	if e := json.Unmarshal([]byte(strings.TrimPrefix(err.Error(), prefix)), &value); e != nil {
		t.Fatal("invalid diagnostic JSON", e)
	}
	wantKeys := []string{"codes", "createDraftID", "expectedTagCommit", "observedTagCommit", "expectedRelease", "observedRelease", "latestID"}
	if len(value) != len(wantKeys) {
		t.Fatal("unexpected diagnostic fields", value)
	}
	for _, key := range wantKeys {
		if _, ok := value[key]; !ok {
			t.Fatal("missing diagnostic field", key)
		}
	}
	var codes []string
	if e := json.Unmarshal(value["codes"], &codes); e != nil || len(codes) == 0 {
		t.Fatal("missing diagnostic codes", e)
	}
	return value, codes
}

func TestPublicationParityDiagnostics(t *testing.T) {
	p := publicationUnitPlan(t)
	tests := []struct {
		code   string
		mutate func(*remotePublicationState)
	}{
		{"TAG_COMMIT", func(s *remotePublicationState) { s.TagCommit = "other" }},
		{"RELEASE_PRESENT", func(s *remotePublicationState) { s.Release = nil }},
		{"RELEASE_ID", func(s *remotePublicationState) { s.Release.ID = 790 }},
		{"RELEASE_TAG", func(s *remotePublicationState) { s.Release.Tag = "other" }},
		{"RELEASE_TARGET", func(s *remotePublicationState) { s.Release.Target = "main" }},
		{"RELEASE_NAME", func(s *remotePublicationState) { s.Release.Name = "other" }},
		{"RELEASE_BODY", func(s *remotePublicationState) { s.Release.Body += "other" }},
		{"RELEASE_DRAFT", func(s *remotePublicationState) { s.Release.Draft = false }},
		{"RELEASE_PRERELEASE", func(s *remotePublicationState) { s.Release.Prerelease = false }},
		{"RELEASE_NOT_LATEST", func(s *remotePublicationState) { s.LatestID = 789 }},
		{"ASSET_COUNT", func(s *remotePublicationState) { s.Release.Assets = []remotePublicationAsset{{ID: 1}} }},
	}
	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			m := newPublicationMock(p)
			_, _ = m.CreateDraft(context.Background(), p)
			tc.mutate(&m.state)
			err := publicationReadback(context.Background(), m, p, 789, true, nil)
			value, codes := publicationParityDiagnosticJSON(t, err)
			if !reflect.DeepEqual(codes, []string{tc.code}) || string(value["createDraftID"]) != "789" {
				t.Fatal("incorrect predicate or create ID", codes, string(value["createDraftID"]))
			}
			if tc.code == "RELEASE_PRESENT" && string(value["observedRelease"]) != "null" {
				t.Fatal("absent release represented as observed metadata")
			}
			if tc.code == "RELEASE_ID" {
				var observed struct{ ID int64 }
				if e := json.Unmarshal(value["observedRelease"], &observed); e != nil || observed.ID != 790 {
					t.Fatal("observed ID not preserved", e, observed.ID)
				}
			}
		})
	}
	m := newPublicationMock(p)
	_, _ = m.CreateDraft(context.Background(), p)
	for _, assets := range [][]remotePublicationAsset{nil, {}} {
		m.state.Release.Assets = assets
		if err := publicationReadback(context.Background(), m, p, 789, true, nil); err != nil {
			t.Fatal("unchanged positive/empty parity", err)
		}
	}
	m.state.Release.Draft = false
	m.state.Release.PublishedAt = "2026-10-25T00:00:00Z"
	if err := publicationReadback(context.Background(), m, p, 789, false, nil); err != nil {
		t.Fatal("published-state positive parity", err)
	}
	m.state.TagCommit = "other"
	m.state.Release.Name = "other"
	m.state.Release.Body = "other"
	m.state.Release.Prerelease = false
	m.state.LatestID = 789
	m.state.Release.Assets = []remotePublicationAsset{{ID: 1}}
	want := []string{"TAG_COMMIT", "RELEASE_NAME", "RELEASE_BODY", "RELEASE_DRAFT", "RELEASE_PRERELEASE", "RELEASE_NOT_LATEST", "ASSET_COUNT"}
	for i := 0; i < 2; i++ {
		_, codes := publicationParityDiagnosticJSON(t, publicationReadback(context.Background(), m, p, 789, true, nil))
		if !reflect.DeepEqual(codes, want) {
			t.Fatal("multiple failures lost or reordered", codes)
		}
	}
	m.state.Release = nil
	_, codes := publicationParityDiagnosticJSON(t, publicationReadback(context.Background(), m, p, 789, true, nil))
	if !reflect.DeepEqual(codes, []string{"TAG_COMMIT", "RELEASE_PRESENT", "RELEASE_NOT_LATEST"}) {
		t.Fatal("nil release lost independent predicates or invented dependent fields", codes)
	}
	for _, body := range []string{strings.ReplaceAll(p.Notes, "\n", "\r\n"), strings.TrimSuffix(p.Notes, "\n"), p.Notes + "\n"} {
		m = newPublicationMock(p)
		_, _ = m.CreateDraft(context.Background(), p)
		m.state.Release.Body = body
		_, codes := publicationParityDiagnosticJSON(t, publicationReadback(context.Background(), m, p, 789, true, nil))
		if !reflect.DeepEqual(codes, []string{"RELEASE_BODY"}) {
			t.Fatal("body normalization changed exact parity", codes)
		}
	}
}

func TestPublicationParityDiagnosticRedaction(t *testing.T) {
	p := publicationUnitPlan(t)
	m := newPublicationMock(p)
	_, _ = m.CreateDraft(context.Background(), p)
	const marker = "fixture-only-sensitive-marker\r\nAuthorization: Bearer fixture-only\nhttps://example.invalid/asset?sig=fixture-only&cookie=fixture-only\x00\x1b"
	large := strings.Repeat(marker, 20000)
	p.Tag, p.Notes, p.Authorization.SourceCommit, p.Authorization.Version = large, large, large, large
	m.state.TagCommit = large + "observed"
	m.state.Release.Tag, m.state.Release.Target, m.state.Release.Name, m.state.Release.Body = large+"tag", large+"target", large+"name", large+"body"
	err := publicationReadback(context.Background(), m, p, 789, true, nil)
	value, _ := publicationParityDiagnosticJSON(t, err)
	for _, forbidden := range []string{"sensitive-marker", "Authorization", "Bearer", "example.invalid", "sig=", "cookie=", "\r", "\n", "\x00", "\x1b"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatal("untrusted diagnostic text escaped redaction", forbidden)
		}
	}
	var fingerprint struct {
		Length int    `json:"length"`
		SHA256 string `json:"sha256"`
	}
	if e := json.Unmarshal(value["observedTagCommit"], &fingerprint); e != nil || fingerprint.Length != len(m.state.TagCommit) || fingerprint.SHA256 != supplyDigest([]byte(m.state.TagCommit)) {
		t.Fatal("fingerprint does not bind immediate bytes", e)
	}
	var observed struct {
		Name, Body struct {
			Length int    `json:"length"`
			SHA256 string `json:"sha256"`
		}
	}
	if e := json.Unmarshal(value["observedRelease"], &observed); e != nil || observed.Name.Length != len(m.state.Release.Name) || observed.Name.SHA256 != supplyDigest([]byte(m.state.Release.Name)) || observed.Body.Length != len(m.state.Release.Body) || observed.Body.SHA256 != supplyDigest([]byte(m.state.Release.Body)) {
		t.Fatal("name/body evidence does not bind immediate bytes", e)
	}
}

// These HTTP fixtures are independent JSON objects, not githubRelease values
// serialized from publicationMock or echoes of the create response/payload.
func TestPublicationParityProviderJSONFixtures(t *testing.T) {
	dir, p := publicationFixture(t)
	type fixture struct {
		name      string
		mutate    func(map[string]any)
		codes     []string
		readError bool
		empty     bool
		latest    bool
		assets    bool
	}
	tests := []fixture{
		{name: "missing-after-create", empty: true, readError: true},
		{name: "different-positive-id", mutate: func(r map[string]any) { r["id"] = 901 }, readError: true},
		{name: "latest-conflict", latest: true, codes: []string{"RELEASE_NOT_LATEST"}},
		{name: "unexpected-assets", assets: true, codes: []string{"ASSET_COUNT"}},
		{name: "name-and-body", mutate: func(r map[string]any) {
			r["name"], r["body"] = "fixture-only-private-name", "fixture-only-private-notes"
		}, codes: []string{"RELEASE_NAME", "RELEASE_BODY"}},
	}
	fields := []struct{ field, code string }{{"id", ""}, {"tag_name", "RELEASE_TAG"}, {"target_commitish", "RELEASE_TARGET"}, {"name", "RELEASE_NAME"}, {"body", "RELEASE_BODY"}, {"draft", "RELEASE_DRAFT"}, {"prerelease", "RELEASE_PRERELEASE"}}
	for _, field := range fields {
		for _, mode := range []string{"missing", "null", "wrong-type"} {
			f, mode := field, mode
			one := fixture{name: f.field + "-" + mode, readError: mode == "wrong-type" || f.field == "id" || f.field == "tag_name" || f.field == "draft" || f.field == "prerelease", mutate: func(r map[string]any) {
				switch mode {
				case "missing":
					delete(r, f.field)
				case "null":
					r[f.field] = nil
				case "wrong-type":
					r[f.field] = []string{"fixture-only-private-type"}
				}
			}}
			if !one.readError {
				one.codes = []string{f.code}
			}
			tests = append(tests, one)
		}
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			created, creates, uploads, publishes := false, 0, 0, 0
			requests := map[string]int{}
			raw := map[string]any{"id": 900, "tag_name": p.Tag, "target_commitish": p.Authorization.SourceCommit, "name": "FlashGate MCP " + p.Authorization.Version, "body": p.Notes, "draft": true, "prerelease": true}
			if tc.mutate != nil {
				tc.mutate(raw)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				endpoint := strings.TrimPrefix(r.URL.Path, "/repos/"+publicationRepository)
				requests[r.Method+" "+endpoint]++
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.Method == "POST" && endpoint == "/releases":
					creates++
					created = true
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{"id":900}`))
				case r.Method == "POST":
					uploads++
					w.WriteHeader(http.StatusInternalServerError)
				case r.Method == "PATCH":
					publishes++
					w.WriteHeader(http.StatusInternalServerError)
				case endpoint == "/git/ref/tags/"+p.Tag:
					_ = json.NewEncoder(w).Encode(map[string]any{"ref": "refs/tags/" + p.Tag, "object": map[string]string{"type": "commit", "sha": p.Authorization.SourceCommit}})
				case endpoint == "/releases":
					if !created || tc.empty {
						_, _ = w.Write([]byte("[]"))
					} else {
						_ = json.NewEncoder(w).Encode([]any{raw})
					}
				case endpoint == "/releases/latest":
					if created && tc.latest {
						_, _ = w.Write([]byte(`{"id":900}`))
					} else {
						http.NotFound(w, r)
					}
				case endpoint == "/releases/900":
					if tc.empty {
						http.NotFound(w, r)
					} else {
						_ = json.NewEncoder(w).Encode(raw)
					}
				case endpoint == "/releases/900/assets" || endpoint == "/releases/901/assets":
					if tc.assets {
						_, _ = w.Write([]byte(`[{"id":1,"name":"fixture-only-private-asset","size":1,"state":"uploaded"}]`))
					} else {
						_, _ = w.Write([]byte("[]"))
					}
				default:
					t.Errorf("unexpected provider route: %s %s", r.Method, endpoint)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
			defer server.Close()
			g := &githubPublication{client: server.Client(), token: "fixture-only", api: server.URL, uploads: server.URL}
			err := executePublication(context.Background(), g, p, dir, filepath.Join(t.TempDir(), "attempt.jsonl"))
			if err == nil || creates != 1 || uploads != 0 || publishes != 0 || requests["GET /releases"] != 2 || requests["GET /git/ref/tags/"+p.Tag] != 2 {
				t.Fatalf("failure did not stop at first readback: create=%d upload=%d publish=%d requests=%v err=%v", creates, uploads, publishes, requests, err)
			}
			for key, count := range requests {
				if count > 2 || strings.HasPrefix(key, "POST ") && count != 1 {
					t.Fatal("provider retry", requests)
				}
			}
			if tc.readError {
				if err.Error() != "HOLD before transfer: provider readback failed" {
					t.Fatal("provider parse error misclassified", err)
				}
			} else {
				if !strings.HasPrefix(err.Error(), "HOLD before transfer: ") {
					t.Fatal("wrong failure boundary", err)
				}
				value, codes := publicationParityDiagnosticJSON(t, errors.New(strings.TrimPrefix(err.Error(), "HOLD before transfer: ")))
				if !reflect.DeepEqual(codes, tc.codes) || string(value["createDraftID"]) != "900" {
					t.Fatal("provider mismatch not diagnosed", codes)
				}
			}
			for _, forbidden := range []string{"fixture-only-private", "Bearer", "sig=", "cookie="} {
				if strings.Contains(err.Error(), forbidden) {
					t.Fatal("provider text disclosed", err)
				}
			}
		})
	}
}

func TestPublicationParityProviderReadBoundaries(t *testing.T) {
	p := publicationUnitPlan(t)
	for _, name := range []string{"latest-404", "inspect-http-error", "inspect-malformed", "latest-null", "latest-wrong-type", "asset-read-error"} {
		t.Run(name, func(t *testing.T) {
			calls := map[string]int{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				endpoint := strings.TrimPrefix(r.URL.Path, "/repos/"+publicationRepository)
				calls[endpoint]++
				if r.Method != "GET" {
					t.Error("readback issued a write")
				}
				switch endpoint {
				case "/git/ref/tags/" + p.Tag:
					_ = json.NewEncoder(w).Encode(map[string]any{"ref": "refs/tags/" + p.Tag, "object": map[string]string{"type": "commit", "sha": p.Authorization.SourceCommit}})
				case "/releases":
					if name == "inspect-http-error" {
						w.WriteHeader(500)
						_, _ = w.Write([]byte("fixture-only-private-provider-error?sig=fixture-only"))
					} else if name == "inspect-malformed" {
						_, _ = w.Write([]byte("fixture-only-private-not-json"))
					} else {
						_ = json.NewEncoder(w).Encode([]any{map[string]any{"id": 900, "tag_name": p.Tag, "target_commitish": p.Authorization.SourceCommit, "name": "FlashGate MCP " + p.Authorization.Version, "body": p.Notes, "draft": true, "prerelease": true}})
					}
				case "/releases/900":
					_ = json.NewEncoder(w).Encode(map[string]any{"id": 900, "tag_name": p.Tag, "target_commitish": p.Authorization.SourceCommit, "name": "FlashGate MCP " + p.Authorization.Version, "body": p.Notes, "draft": true, "prerelease": true})
				case "/releases/latest":
					switch name {
					case "latest-null":
						_, _ = w.Write([]byte("null"))
					case "latest-wrong-type":
						_, _ = w.Write([]byte(`{"id":"fixture-only-private-id"}`))
					default:
						http.NotFound(w, r)
					}
				case "/releases/900/assets":
					if name == "asset-read-error" {
						_, _ = w.Write([]byte(`[{"id":"fixture-only-private-id"}]`))
					} else {
						_, _ = w.Write([]byte("[]"))
					}
				default:
					t.Error("unexpected read route", endpoint)
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			g := &githubPublication{client: server.Client(), token: "fixture-only", api: server.URL, uploads: server.URL}
			err := publicationReadback(context.Background(), g, p, 900, true, nil)
			if name == "latest-404" {
				if err != nil || len(calls) != 5 {
					t.Fatal("valid empty assets/latest absence rejected", err, calls)
				}
			} else if err == nil || err.Error() != "provider readback failed" {
				t.Fatal("read error changed class or disclosed provider data", err)
			}
			for _, n := range calls {
				if n != 1 {
					t.Fatal("Inspect retried", calls)
				}
			}
		})
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
		if strings.HasPrefix(r.URL.Path, "/"+publicationRepository+"/releases/download/") {
			if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
				t.Error("credential leaked to public download")
			}
			name := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			for _, asset := range mock.state.Release.Assets {
				if asset.Name == name {
					_, _ = w.Write(mock.data[asset.ID])
					return
				}
			}
			http.NotFound(w, r)
			return
		}
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
				releases = append(releases, githubRelease{ID: m.ID, Tag: m.Tag, Target: m.Target, Name: m.Name, Body: m.Body, Draft: m.Draft, Prerelease: m.Prerelease, PublishedAt: m.PublishedAt})
			}
			response = releases
		case r.Method == "GET" && path == "/releases/789":
			m := mock.state.Release
			response = githubRelease{ID: m.ID, Tag: m.Tag, Target: m.Target, Name: m.Name, Body: m.Body, Draft: m.Draft, Prerelease: m.Prerelease, PublishedAt: m.PublishedAt}
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
			if r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("Content-Type") != "application/octet-stream" {
				t.Error("invalid release upload media negotiation")
			}
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
	client := &http.Client{Transport: publicationLocalDownloadTransport(t, server, func(*http.Request) {})}
	g := &githubPublication{client: client, token: "test-only", api: server.URL, uploads: server.URL}
	if err := executePublication(context.Background(), g, plan, dir, filepath.Join(t.TempDir(), "attempt.jsonl")); err != nil {
		t.Fatal(err)
	}
	if writeRequests != len(plan.Assets)+2 {
		t.Fatal("provider write count mismatch")
	}
}

func TestPublicationGitHubUploadMediaContractAndFailureBoundary(t *testing.T) {
	payload := []byte("fixture asset payload")
	for _, scenario := range []struct {
		name   string
		status int
	}{
		{"created", http.StatusCreated},
		{"unsupported-media", http.StatusUnsupportedMediaType},
		{"duplicate-asset", http.StatusUnprocessableEntity},
		{"upstream-failure", http.StatusBadGateway},
		{"redirect-rejected", http.StatusTemporaryRedirect},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/repos/"+publicationRepository+"/releases/123/assets" || r.URL.Query().Get("name") != "candidate-manifest.json" {
					t.Error("upload target or method mismatch")
				}
				if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
					t.Errorf("incorrect upload Accept: %q", got)
				}
				if got := r.Header.Get("Content-Type"); got != "application/octet-stream" {
					t.Errorf("incorrect upload Content-Type: %q", got)
				}
				if got := r.ContentLength; got != int64(len(payload)) {
					t.Errorf("incorrect content length: %d", got)
				}
				if got, err := io.ReadAll(r.Body); err != nil || !bytes.Equal(got, payload) {
					t.Error("upload payload modified")
				}
				if scenario.status == http.StatusTemporaryRedirect {
					w.Header().Set("Location", "/another-upload")
				}
				w.WriteHeader(scenario.status)
				// Provider content must never be reflected by the returned error.
				_, _ = w.Write([]byte("sensitive-provider-error-content"))
			}))
			defer server.Close()
			client := server.Client()
			client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
			g := &githubPublication{client: client, token: "test-only", api: server.URL, uploads: server.URL}
			err := g.Upload(context.Background(), 123, publicationAsset{Name: "candidate-manifest.json", Size: int64(len(payload))}, bytes.NewReader(payload))
			if calls != 1 {
				t.Fatalf("upload must be attempted once, got %d", calls)
			}
			if scenario.status == http.StatusCreated {
				if err != nil {
					t.Fatalf("correct upload rejected: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", scenario.status)) || strings.Contains(err.Error(), "sensitive-provider-error-content") {
				t.Fatalf("unsafe upload failure classification: %v", err)
			}
		})
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
	evidenceAssets, err := publicationEvidenceAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range evidenceAssets {
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
	// Reuse these real four-target bytes for the Actions storage boundary; no
	// additional producer fixture and no weakening of anchored consumer checks.
	for _, kind := range []string{"azure", "invalid-evidence", "truncated", "stream-failure", "stream-close", "transport-failure"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			g := &githubPublication{token: "fixture-only", api: githubAPI}
			g.client = &http.Client{Transport: publicationTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host == "api.github.com" {
					if r.Header.Get("Authorization") != "Bearer fixture-only" {
						t.Fatal("API authorization missing")
					}
					if strings.HasSuffix(r.URL.Path, "/zip") {
						return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://fixture.blob.core.windows.net/bundle?sig=fixture%2Bonly%3D"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
					}
					body, err := json.Marshal(map[string]any{"id": a.EvidenceArtifactID, "name": "retained-candidate-evidence-" + a.IndexSHA256, "expired": false, "expires_at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
					if err != nil {
						t.Fatal(err)
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Request: r}, nil
				}
				if r.URL.Host != "fixture.blob.core.windows.net" || len(r.Header) != 0 || r.URL.RawQuery != "sig=fixture%2Bonly%3D" {
					t.Fatal("storage identity, signed query or credential isolation failed")
				}
				if kind == "transport-failure" {
					return nil, errors.New("controlled unavailable download")
				}
				data := bundle.Bytes()
				if kind == "invalid-evidence" {
					data = publicationTestZip(t, []string{"untrusted"}, 0o600)
				} else if kind == "truncated" {
					data = data[:len(data)/2]
				}
				body := &publicationFaultStream{Reader: bytes.NewReader(data)}
				if kind == "stream-failure" {
					body.readErr = io.ErrUnexpectedEOF
				} else if kind == "stream-close" {
					body.closeErr = errors.New("controlled close failure")
				}
				return &http.Response{StatusCode: 200, Body: body, Request: r}, nil
			})}
			dest := filepath.Join(t.TempDir(), "evidence")
			err := g.downloadEvidence(context.Background(), a, dest)
			if kind == "azure" {
				if err != nil {
					t.Fatal(err)
				}
				actual, err := publicationAssets(dest)
				if err != nil || !reflect.DeepEqual(actual, plan.Assets) {
					t.Fatal("redirect changed anchored four-target evidence", err)
				}
			} else if err == nil {
				t.Fatal("redirect bypassed anchored verification/transport failure")
			} else if strings.Contains(err.Error(), "sig=") || strings.Contains(err.Error(), "fixture%2B") {
				t.Fatal("signed query leaked")
			}
			if calls != 3 {
				t.Fatal("download retried or fell back", calls)
			}
			if kind != "transport-failure" {
				if _, err := os.Stat(dest + ".transport.zip"); err != nil {
					t.Fatal("transport recovery bytes not retained", err)
				}
			}
		})
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

func TestPublicationDownloadClasses(t *testing.T) {
	for _, tc := range []struct {
		name, location string
		class          publicationDownloadClass
		status         int
		valid          bool
	}{
		{"actions-azure", "https://fixture.blob.core.windows.net/container/blob?sig=fixture%2Bonly%3D&se=bound", actionsArtifactDownload, 302, true},
		{"actions-github", "https://release-assets.githubusercontent.com/blob", actionsArtifactDownload, 307, true},
		{"asset-github", "https://objects.githubusercontent.com/blob", releaseAssetDownload, 301, true},
		{"asset-azure", "https://fixture.blob.core.windows.net/blob", releaseAssetDownload, 302, false},
		{"suffix", "https://evilblob.core.windows.net/blob", actionsArtifactDownload, 302, false},
		{"suffix-tail", "https://fixture.blob.core.windows.net.evil.example/blob", actionsArtifactDownload, 302, false},
		{"bare-suffix", "https://blob.core.windows.net/blob", actionsArtifactDownload, 302, false},
		{"http", "http://fixture.blob.core.windows.net/blob", actionsArtifactDownload, 302, false},
		{"userinfo", "https://user:fixture@fixture.blob.core.windows.net/blob", actionsArtifactDownload, 302, false},
		{"port", "https://fixture.blob.core.windows.net:443/blob", actionsArtifactDownload, 302, false},
		{"empty-port", "https://fixture.blob.core.windows.net:/blob", actionsArtifactDownload, 302, false},
		{"ip", "https://127.0.0.1/blob", actionsArtifactDownload, 302, false},
		{"opaque", "https:fixture.blob.core.windows.net/blob", actionsArtifactDownload, 302, false},
		{"relative", "/fixture", actionsArtifactDownload, 302, false},
		{"invalid-location", "https://%zz/", actionsArtifactDownload, 302, false},
		{"fragment", "https://fixture.blob.core.windows.net/blob#hidden", actionsArtifactDownload, 302, false},
		{"bad-label", "https://-fixture.blob.core.windows.net/blob", actionsArtifactDownload, 302, false},
		{"empty-label", "https://fixture..blob.core.windows.net/blob", actionsArtifactDownload, 302, false},
		{"unsupported-303", "https://fixture.blob.core.windows.net/blob", actionsArtifactDownload, 303, false},
		{"unsupported-308", "https://fixture.blob.core.windows.net/blob", actionsArtifactDownload, 308, false},
		{"unavailable", "", actionsArtifactDownload, 404, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			u, _ := url.Parse("https://fixture.blob.core.windows.net")
			jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "fixture-only"}})
			g := &githubPublication{token: "fixture-only", api: githubAPI}
			g.client = &http.Client{Timeout: time.Second, Jar: jar, Transport: publicationTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					if r.URL.Host != "api.github.com" || r.Header.Get("Authorization") != "Bearer fixture-only" {
						t.Fatal("initial request lost its bound API identity")
					}
					return &http.Response{StatusCode: tc.status, Header: http.Header{"Location": {tc.location}, "Set-Cookie": {"provider=fixture-only"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
				}
				if !tc.valid || calls != 2 || r.URL.String() != tc.location || r.Method != "GET" {
					t.Fatal("unexpected redirect request or changed signed query")
				}
				for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "GH_TOKEN", "X-GitHub-Api-Version", "Accept", "Referer"} {
					if r.Header.Get(header) != "" {
						t.Fatal("sensitive or unvetted redirect header", header)
					}
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("exact bytes")), Request: r}, nil
			})}
			endpoint := "/actions/artifacts/456/zip"
			if tc.class == releaseAssetDownload {
				endpoint = "/releases/assets/456"
			}
			stream, err := g.download(context.Background(), endpoint, tc.class)
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				data, readErr := io.ReadAll(stream)
				if err := errors.Join(readErr, stream.Close()); err != nil || string(data) != "exact bytes" || calls != 2 {
					t.Fatal("download bytes/close mismatch", err)
				}
			} else if err == nil || calls != 1 {
				t.Fatal("unsafe redirect did not fail before external request", calls, err)
			} else if strings.Contains(err.Error(), tc.location) && tc.location != "" {
				t.Fatal("redirect URL leaked in diagnostics")
			}
		})
	}
	for _, endpoint := range []string{"/actions/artifacts/456", "/releases/assets/456", "/actions/artifacts/0456/zip", "/actions/artifacts/456/zip?other=1", "/actions/artifacts/-1/zip"} {
		g := &githubPublication{}
		if _, err := g.download(context.Background(), endpoint, actionsArtifactDownload); err == nil {
			t.Fatal("broadened an unrelated endpoint", endpoint)
		}
	}
	for _, class := range []publicationDownloadClass{releaseAssetDownload, actionsArtifactDownload, 255} {
		calls := 0
		g := &githubPublication{api: githubAPI, client: &http.Client{Transport: publicationTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://objects.githubusercontent.com/loop"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
		})}}
		endpoint := "/releases/assets/1"
		if class == actionsArtifactDownload {
			endpoint = "/actions/artifacts/1/zip"
		}
		if _, err := g.download(context.Background(), endpoint, class); err == nil || class != 255 && calls != 4 || class == 255 && calls != 0 {
			t.Fatal("redirect/class budget not enforced", calls, err)
		}
	}
}

func publicationTestZip(t *testing.T, names []string, mode os.FileMode) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, name := range names {
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		h.SetMode(mode)
		out, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Write([]byte("fixture")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestPublicationPortableEvidencePaths(t *testing.T) {
	// Bound prefix/alias bookkeeping as well as expanded file bytes. A ZIP can
	// carry long names without consuming the file-expansion budget.
	if publicationEvidencePath(strings.Repeat("a/", 512)+"file") == nil {
		t.Fatal("unbounded evidence name accepted")
	}
	for _, name := range []string{"", "../escape", "a/../../escape", "/absolute", "C:/drive", "C:drive", `\\server\share`, `a\b`, "a/../b", "a//b", "a/./b", "a\x00b", "a\x01b", "a\x7fb", "a:stream", "NUL", "con.txt", "a/COM1.json", "lPt9", "CONIN$", "a/CONOUT$", "a.", "a /b", "a/b ", "a/<b", "a/?b", "a/é", "a/COM¹.txt"} {
		t.Run(fmt.Sprintf("%q", name), func(t *testing.T) {
			root := t.TempDir()
			sentinel := filepath.Join(root, "sentinel")
			if err := os.WriteFile(sentinel, []byte("unchanged"), 0o600); err != nil {
				t.Fatal(err)
			}
			data := publicationTestZip(t, []string{name}, 0o600)
			if extractPublicationEvidence(bytes.NewReader(data), int64(len(data)), filepath.Join(root, "evidence")) == nil {
				t.Fatal("unsafe portable name accepted")
			}
			if got, err := os.ReadFile(sentinel); err != nil || string(got) != "unchanged" {
				t.Fatal("outside sentinel changed", err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 2 {
				t.Fatal("unexpected file outside evidence root", err)
			}
		})
	}
	for _, names := range [][]string{{"a", "a"}, {"a", "A"}, {"a", "a/b"}, {"a/b", "a"}, {"A/b", "a/c"}} {
		data := publicationTestZip(t, names, 0o600)
		if extractPublicationEvidence(bytes.NewReader(data), int64(len(data)), filepath.Join(t.TempDir(), "evidence")) == nil {
			t.Fatal("alias/prefix collision accepted", names)
		}
	}
	for _, mode := range []os.FileMode{os.ModeSymlink | 0o777, os.ModeDir | 0o700, os.ModeNamedPipe | 0o600, os.ModeDevice | 0o600, os.ModeSocket | 0o600} {
		data := publicationTestZip(t, []string{"entry"}, mode)
		if extractPublicationEvidence(bytes.NewReader(data), int64(len(data)), filepath.Join(t.TempDir(), "evidence")) == nil {
			t.Fatal("special entry accepted", mode)
		}
	}
	data := publicationTestZip(t, []string{"candidate/a/b.json", "candidate/a/c.json", "source/COPYING"}, 0o600)
	dest := filepath.Join(t.TempDir(), "evidence")
	if err := extractPublicationEvidence(bytes.NewReader(data), int64(len(data)), dest); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"candidate/a/b.json", "candidate/a/c.json", "source/COPYING"} {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil || string(got) != "fixture" {
			t.Fatal("nested regular evidence changed", err)
		}
	}
	if extractPublicationEvidence(bytes.NewReader(data), int64(len(data)), dest) == nil {
		t.Fatal("existing root adopted")
	}
}

type publicationFaultStream struct {
	io.Reader
	readErr, closeErr error
	closed            bool
}

func (s *publicationFaultStream) Read(p []byte) (int, error) {
	if s.readErr != nil {
		return 0, s.readErr
	}
	return s.Reader.Read(p)
}
func (s *publicationFaultStream) Close() error { s.closed = true; return s.closeErr }

type publicationFaultOutput struct {
	buffer             bytes.Buffer
	writeErr, closeErr error
	closed             bool
}

func (s *publicationFaultOutput) Write(p []byte) (int, error) {
	if s.writeErr != nil {
		return 0, s.writeErr
	}
	return s.buffer.Write(p)
}
func (s *publicationFaultOutput) Close() error { s.closed = true; return s.closeErr }

func TestPublicationEvidenceBoundsAndIO(t *testing.T) {
	for _, kind := range []string{"short", "long", "read", "write", "input-close", "output-close"} {
		t.Run(kind, func(t *testing.T) {
			in := &publicationFaultStream{Reader: strings.NewReader("fixture")}
			out := &publicationFaultOutput{}
			size := uint64(7)
			failure := errors.New("controlled I/O failure")
			switch kind {
			case "short":
				size++
			case "long":
				size--
			case "read":
				in.readErr = failure
			case "write":
				out.writeErr = failure
			case "input-close":
				in.closeErr = failure
			case "output-close":
				out.closeErr = failure
			}
			if err := copyPublicationEvidence(in, out, size); err == nil || !in.closed || !out.closed {
				t.Fatal("I/O failure/close contract lost", err)
			}
			if out.buffer.Len() > int(size)+1 {
				t.Fatal("streaming bound exceeded")
			}
		})
	}
	names := make([]string, 129)
	for i := range names {
		names[i] = fmt.Sprintf("file-%d", i)
	}
	for _, kind := range []string{"entries", "single-size", "total-size", "declared-size", "crc", "truncated"} {
		t.Run(kind, func(t *testing.T) {
			selected := names[:1]
			if kind == "entries" {
				selected = names
			} else if kind == "total-size" {
				selected = names[:6]
			}
			data := publicationTestZip(t, selected, 0o600)
			for pos := 0; ; {
				at := bytes.Index(data[pos:], []byte{'P', 'K', 1, 2})
				if at < 0 {
					break
				}
				at += pos
				switch kind {
				case "single-size":
					binary.LittleEndian.PutUint32(data[at+24:], maximumSupplyArchiveSize+1)
				case "total-size":
					binary.LittleEndian.PutUint32(data[at+24:], maximumSupplyArchiveSize)
				case "declared-size":
					binary.LittleEndian.PutUint32(data[at+24:], 8)
				case "crc":
					data[at+16] ^= 1
				}
				pos = at + 46
			}
			if kind == "truncated" {
				data = data[:len(data)-24]
			}
			if extractPublicationEvidence(bytes.NewReader(data), int64(len(data)), filepath.Join(t.TempDir(), "evidence")) == nil {
				t.Fatal("unsafe ZIP resource/integrity condition accepted")
			}
		})
	}
}

func TestPublicationEvidenceRootConfinement(t *testing.T) {
	for _, kind := range []string{"existing-file", "hardlink", "symlink", "root-link", "parent-link", "root-exchange", "parent-exchange", "directory-exchange"} {
		t.Run(kind, func(t *testing.T) {
			base := t.TempDir()
			outside := filepath.Join(base, "outside")
			parent := filepath.Join(base, "parent")
			for _, dir := range []string{outside, parent} {
				if err := os.Mkdir(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			sentinel := filepath.Join(outside, "sentinel")
			if err := os.WriteFile(sentinel, []byte("unchanged"), 0o600); err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(parent, "evidence")
			if kind == "root-link" || kind == "parent-link" {
				link := dest
				if kind == "parent-link" {
					link = filepath.Join(base, "parent-link")
					dest = filepath.Join(link, "evidence")
				}
				if err := os.Symlink(outside, link); err != nil {
					t.Fatal("required symlink fixture unavailable", err)
				}
				data := publicationTestZip(t, []string{"sentinel"}, 0o600)
				if extractPublicationEvidence(bytes.NewReader(data), int64(len(data)), dest) == nil {
					t.Fatal("linked root/parent accepted")
				}
			} else {
				bound, err := newPublicationEvidenceRoot(dest)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := bound.close(); err != nil {
						t.Error(err)
					}
				}()
				switch kind {
				case "existing-file", "hardlink":
					original := "owned-original"
					var err error
					if kind == "hardlink" {
						original = "unchanged"
						err = os.Link(sentinel, filepath.Join(dest, "sentinel"))
					} else {
						err = os.WriteFile(filepath.Join(dest, "sentinel"), []byte(original), 0o600)
					}
					if err != nil {
						t.Fatal(err)
					}
					if f, err := bound.root.OpenFile("sentinel", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); err == nil {
						_ = f.Close()
						t.Fatal("existing file adopted")
					}
					if data, err := os.ReadFile(filepath.Join(dest, "sentinel")); err != nil || string(data) != original {
						t.Fatal("existing file overwritten", err)
					}
				case "symlink":
					if err := os.Symlink(outside, filepath.Join(dest, "redirect")); err != nil {
						t.Fatal(err)
					}
					if bound.mkdirParents("redirect/new") == nil {
						t.Fatal("unowned linked parent adopted")
					}
					if f, err := bound.root.OpenFile("redirect/new", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); err == nil {
						_ = f.Close()
						t.Fatal("descriptor root escaped")
					}
				default:
					target := dest
					if kind == "parent-exchange" {
						target = parent
					} else if kind == "directory-exchange" {
						if err := bound.mkdirParents("nested/file"); err != nil {
							t.Fatal(err)
						}
						target = filepath.Join(dest, "nested")
					}
					if err := os.Rename(target, target+"-moved"); err != nil {
						var code syscall.Errno
						if runtime.GOOS != "windows" || !errors.As(err, &code) || code != 5 && code != 32 {
							t.Fatal(err)
						}
						// Windows may prevent exchange while Root holds its handle.
						if err := bound.check(); err != nil {
							t.Fatal("blocked exchange changed the binding", err)
						}
						t.Log("descriptor handle prevented the requested exchange")
					} else {
						if err := os.Symlink(outside, target); err != nil {
							t.Fatal(err)
						}
						if bound.check() == nil {
							t.Fatal("identity/reparse drift accepted")
						}
					}
				}
			}
			if data, err := os.ReadFile(sentinel); err != nil || string(data) != "unchanged" {
				t.Fatal("outside sentinel modified", err)
			}
			if entries, err := os.ReadDir(outside); err != nil || len(entries) != 1 {
				t.Fatal("outside file created", err)
			}
		})
	}
}
