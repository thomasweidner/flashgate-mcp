package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicationContinuationAuthorization(t *testing.T) {
	plan := publicationUnitPlan(t)
	plan.Authorization.OriginalApproval = "BL363-v0.3.0-39936762949-20261025-fa4082ce-A04"
	original, _ := supplyJSON(plan)
	valid := publicationContinuationAuthorization{"flashgate-publication-continuation-authorization/v1", "COMPLETE_EMPTY_DRAFT_UPLOAD_READBACK_PUBLISH_PRERELEASE", publicationRepository, 423367074, "EXACT_EMPTY_DRAFT", "fixture-new-human-approval", supplyDigest(original)}
	for _, name := range []string{"valid", "consumed-A04", "create-schema", "create-action", "wrong-repo", "zero-id", "negative-id", "partial-draft", "bad-plan-hash", "bad-approval", "unknown-field", "duplicate", "trailing", "transport-hash", "plan-bytes"} {
		t.Run(name, func(t *testing.T) {
			a := valid
			switch name {
			case "consumed-A04":
				a.OriginalApproval = plan.Authorization.OriginalApproval
			case "create-schema":
				a.Schema = plan.Authorization.Schema
			case "create-action":
				a.Action = plan.Authorization.Action
			case "wrong-repo":
				a.Repository = "other/repository"
			case "zero-id":
				a.ReleaseID = 0
			case "negative-id":
				a.ReleaseID = -1
			case "partial-draft":
				a.PreState = "PARTIAL_DRAFT"
			case "bad-plan-hash":
				a.PlanSHA256 = strings.Repeat("0", 64)
			case "bad-approval":
				a.OriginalApproval = "unsafe approval"
			}
			data, _ := supplyJSON(a)
			switch name {
			case "unknown-field":
				data = bytes.Replace(data, []byte("{\n"), []byte("{\n\"extra\":true,"), 1)
			case "duplicate":
				data = bytes.Replace(data, []byte("{\n"), []byte("{\n\"releaseId\":423367074,"), 1)
			case "trailing":
				data = append(data, []byte("{}")...)
			}
			root := t.TempDir()
			authPath, manifestPath := filepath.Join(root, "authorization.json"), filepath.Join(root, "original-plan.json")
			manifest := append([]byte{}, original...)
			if name == "plan-bytes" {
				manifest = append(manifest, '\n')
			}
			if err := os.WriteFile(authPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifestPath, manifest, 0o600); err != nil {
				t.Fatal(err)
			}
			anchor := supplyDigest(data)
			if name == "transport-hash" {
				anchor = strings.Repeat("0", 64)
			}
			_, c, p, err := readPublicationRequest(authPath, anchor, manifestPath)
			if name == "valid" {
				if err != nil || c.ReleaseID != 423367074 || p.Authorization.OriginalApproval != plan.Authorization.OriginalApproval {
					t.Fatal("valid exact binding rejected", err)
				}
				if _, _, _, err := readPublicationRequest(authPath, anchor, ""); err == nil {
					t.Fatal("continuation accepted without original manifest")
				}
			} else if err == nil {
				t.Fatal("invalid continuation accepted")
			}
		})
	}
}

type publicationContinuationMock struct {
	*publicationMock
	creates, publicReads                    int
	publicError, publicCorrupt, noTimestamp bool
}

func (m *publicationContinuationMock) CreateDraft(context.Context, publicationPlan) (int64, error) {
	m.creates++
	return 0, errors.New("continuation must never create")
}
func (m *publicationContinuationMock) Publish(ctx context.Context, id int64, p publicationPlan) error {
	if err := m.publicationMock.Publish(ctx, id, p); err != nil {
		return err
	}
	if m.noTimestamp {
		m.state.Release.PublishedAt = ""
	}
	return nil
}
func (m *publicationContinuationMock) ReadPublicAsset(ctx context.Context, p publicationPlan, a publicationAsset) (io.ReadCloser, error) {
	m.publicReads++
	if m.publicError {
		return nil, errors.New("public 404")
	}
	if m.publicCorrupt {
		return io.NopCloser(strings.NewReader("altered")), nil
	}
	return m.publicationMock.ReadPublicAsset(ctx, p, a)
}
func publicationEmptyDraft(t *testing.T, plan publicationPlan) *publicationContinuationMock {
	t.Helper()
	m := newPublicationMock(plan)
	if _, err := m.CreateDraft(context.Background(), plan); err != nil {
		t.Fatal(err)
	}
	m.writes = 0
	return &publicationContinuationMock{publicationMock: m}
}

func TestPublicationContinuationExactOriginalManifest(t *testing.T) {
	dir, plan := publicationFixture(t)
	if len(plan.Assets)+1 != 40 {
		t.Fatal("fixture must cover the full 40-asset contract")
	}
	original, _ := supplyJSON(plan)
	m := publicationEmptyDraft(t, plan)
	journal := filepath.Join(t.TempDir(), "new-attempt.jsonl")
	newAnchor := supplyDigest([]byte("new independently approved subject"))
	if err := executePublicationTarget(context.Background(), m, plan, dir, journal, 789, newAnchor); err != nil {
		t.Fatal(err)
	}
	if m.creates != 0 || m.writes != 40+1 || m.publicReads != 40 || m.state.Release.Draft {
		t.Fatal("continuation sequence drift", m.creates, m.writes, m.publicReads)
	}
	for _, a := range m.state.Release.Assets {
		if a.Name == "publication-manifest.json" && !bytes.Equal(m.data[a.ID], original) {
			t.Fatal("original manifest changed")
		}
	}
	data, err := os.ReadFile(journal)
	if err != nil || !bytes.Contains(data, []byte(newAnchor)) || !bytes.Contains(data, []byte(supplyDigest(original))) || bytes.Contains(data, []byte("CREATE_ONCE")) {
		t.Fatal("journal binding mismatch", err)
	}
	// Resetting the provider does not reset the consumed exclusive journal.
	m.state.Release.Draft = true
	m.state.Release.Assets = nil
	m.state.Release.PublishedAt = ""
	before := m.writes
	if err := executePublicationTarget(context.Background(), m, plan, dir, journal, 789, newAnchor); err == nil || m.writes != before {
		t.Fatal("consumed continuation repeated")
	}
}

func TestPublicationContinuationPrestateAndFailures(t *testing.T) {
	dir, plan := publicationFixture(t)
	for _, name := range []string{"absent", "wrong-id", "wrong-tag", "wrong-source", "wrong-notes", "wrong-name", "published", "stable", "nonempty", "tag-drift", "read-denied", "corrupt-upload", "final-read", "public-404", "public-corrupt", "missing-timestamp"} {
		t.Run(name, func(t *testing.T) {
			m := publicationEmptyDraft(t, plan)
			switch name {
			case "absent":
				m.state.Release = nil
			case "wrong-id":
				m.state.Release.ID++
			case "wrong-tag":
				m.state.Release.Tag = "v0.0.1"
			case "wrong-source":
				m.state.Release.Target = strings.Repeat("a", 40)
			case "wrong-notes":
				m.state.Release.Body += "changed"
			case "wrong-name":
				m.state.Release.Name = "other"
			case "published":
				m.state.Release.Draft = false
			case "stable":
				m.state.Release.Prerelease = false
			case "nonempty":
				m.state.Release.Assets = []remotePublicationAsset{{ID: 1, Name: "foreign", State: "uploaded"}}
			case "tag-drift":
				m.state.TagCommit = strings.Repeat("a", 40)
			case "read-denied":
				m.failRead = true
			case "corrupt-upload":
				m.corrupt = true
			case "final-read":
				m.finalFail = true
			case "public-404":
				m.publicError = true
			case "public-corrupt":
				m.publicCorrupt = true
			case "missing-timestamp":
				m.noTimestamp = true
			}
			err := executePublicationTarget(context.Background(), m, plan, dir, filepath.Join(t.TempDir(), "attempt.jsonl"), 789, supplyDigest([]byte("new-approval")))
			if err == nil || m.creates != 0 {
				t.Fatal("unsafe continuation accepted/recreated", err)
			}
			late := name == "corrupt-upload" || name == "final-read" || name == "public-404" || name == "public-corrupt" || name == "missing-timestamp"
			if !late && m.writes != 0 {
				t.Fatal("invalid prestate reached upload")
			}
			if name == "corrupt-upload" && (m.writes != 1 || !m.state.Release.Draft) {
				t.Fatal("corrupt upload continued/published")
			}
			if (name == "public-404" || name == "public-corrupt") && (m.publicReads != 1 || m.state.Release.Draft) {
				t.Fatal("public read failure retried or rolled back")
			}
		})
	}
	for fail := 1; fail <= len(plan.Assets)+2; fail++ {
		m := publicationEmptyDraft(t, plan)
		m.failWrite = fail
		if err := executePublicationTarget(context.Background(), m, plan, dir, filepath.Join(t.TempDir(), "attempt.jsonl"), 789, supplyDigest([]byte("new-approval"))); err == nil || m.writes != fail || m.creates != 0 {
			t.Fatal("mutating failure retried or fell back", fail, err)
		}
	}
}

func TestPublicationDirectIDVisibilityAndErrors(t *testing.T) {
	plan := publicationUnitPlan(t)
	for _, name := range []string{"list-hidden", "list-visible", "list-conflict", "list-lag", "id-404", "id-403", "id-500", "id-null", "id-invalid", "id-duplicate", "id-wrong", "timeout"} {
		t.Run(name, func(t *testing.T) {
			direct := githubRelease{ID: 423367074, Tag: plan.Tag, Target: plan.Authorization.SourceCommit, Name: "FlashGate MCP " + plan.Authorization.Version, Body: plan.Notes, Draft: true, Prerelease: true}
			calls := map[string]int{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				endpoint := strings.TrimPrefix(r.URL.Path, "/repos/"+publicationRepository)
				calls[endpoint]++
				if r.Method != "GET" {
					t.Error("read-only fixture received mutation")
				}
				switch endpoint {
				case "/git/ref/tags/" + plan.Tag:
					_ = json.NewEncoder(w).Encode(map[string]any{"ref": "refs/tags/" + plan.Tag, "object": githubObject{plan.Authorization.SourceCommit, "commit"}})
				case "/releases":
					list := []githubRelease{}
					if name == "list-visible" {
						list = append(list, direct)
					}
					if name == "list-conflict" {
						other := direct
						other.ID++
						list = append(list, other)
					}
					_ = json.NewEncoder(w).Encode(list)
				case "/releases/423367074":
					switch name {
					case "id-404":
						w.WriteHeader(404)
					case "id-403":
						w.WriteHeader(403)
					case "id-500":
						w.WriteHeader(500)
					case "id-null":
						_, _ = io.WriteString(w, "null")
					case "id-invalid":
						_, _ = io.WriteString(w, "invalid")
					case "id-duplicate":
						_, _ = io.WriteString(w, `{"id":423367074,"id":423367074}`)
					case "id-wrong":
						direct.ID++
						_ = json.NewEncoder(w).Encode(direct)
					default:
						_ = json.NewEncoder(w).Encode(direct)
					}
				case "/releases/latest":
					w.WriteHeader(404)
				case "/releases/423367074/assets":
					_, _ = io.WriteString(w, "[]")
				default:
					t.Error("unexpected endpoint", endpoint)
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			g := &githubPublication{client: server.Client(), api: server.URL, uploads: server.URL, token: "fixture-only"}
			ctx := context.Background()
			if name == "timeout" {
				base := g.client.Transport
				g.client = &http.Client{Transport: publicationTransport(func(r *http.Request) (*http.Response, error) {
					if strings.HasSuffix(r.URL.Path, "/releases/423367074") {
						calls["id-timeout"]++
						return nil, context.DeadlineExceeded
					}
					return base.RoundTrip(r)
				})}
			}
			err := publicationReadback(ctx, g, plan, 423367074, true, nil)
			positive := name == "list-hidden" || name == "list-visible" || name == "list-lag"
			if (err == nil) != positive {
				t.Fatal("direct-ID outcome mismatch", err)
			}
			for endpoint, count := range calls {
				if count != 1 {
					t.Fatal("retry", endpoint, count)
				}
			}
			if positive && calls["/releases/423367074"] != 1 {
				t.Fatal("direct endpoint not read")
			}
		})
	}
}

func TestPublicationPublicDownloadHasNoCredentialOrFallback(t *testing.T) {
	plan := publicationUnitPlan(t)
	for _, code := range []int{200, 403, 404, 500, 302} {
		calls := 0
		jar, _ := cookiejar.New(nil)
		u, _ := url.Parse("https://github.com")
		jar.SetCookies(u, []*http.Cookie{{Name: "secret", Value: "fixture"}})
		client := &http.Client{Jar: jar, Transport: publicationTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.URL.Host != "github.com" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("X-GitHub-Api-Version") != "" {
				t.Fatal("public request borrowed API identity")
			}
			return &http.Response{StatusCode: code, Header: http.Header{"Location": []string{"https://untrusted.invalid/private"}}, Body: io.NopCloser(strings.NewReader("public bytes"))}, nil
		})}
		g := &githubPublication{client: client, token: "fixture-secret", api: githubAPI, uploads: githubUploads}
		stream, err := g.ReadPublicAsset(context.Background(), plan, publicationAsset{Name: "publication-manifest.json"})
		if stream != nil {
			_ = stream.Close()
		}
		if (err == nil) != (code == 200) || calls != 1 {
			t.Fatal("public failure retried/fell back", code, calls, err)
		}
	}
}

func TestPublicationDirectIDRequiresExplicitFinalFlags(t *testing.T) {
	plan := publicationUnitPlan(t)
	for _, field := range []string{"draft", "prerelease"} {
		for _, null := range []bool{false, true} {
			raw := map[string]any{"id": 789, "tag_name": plan.Tag, "target_commitish": plan.Authorization.SourceCommit,
				"name": "FlashGate MCP " + plan.Authorization.Version, "body": plan.Notes,
				"draft": false, "prerelease": true, "published_at": "2026-10-25T00:00:00Z"}
			delete(raw, field)
			if null {
				raw[field] = nil
			}
			g := &githubPublication{api: githubAPI, uploads: githubUploads, token: "fixture-only"}
			g.client = &http.Client{Transport: publicationTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" {
					t.Fatal("flag validation wrote")
				}
				var value any
				switch {
				case strings.Contains(r.URL.Path, "/git/ref/tags/"):
					value = map[string]any{"ref": "refs/tags/" + plan.Tag, "object": githubObject{plan.Authorization.SourceCommit, "commit"}}
				case strings.HasSuffix(r.URL.Path, "/releases"):
					value = []any{}
				case strings.HasSuffix(r.URL.Path, "/releases/789"):
					value = raw
				default:
					t.Fatal("invalid flags reached further readback")
				}
				data, _ := json.Marshal(value)
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(data))}, nil
			})}
			if err := publicationReadback(context.Background(), g, plan, 789, false, nil); err == nil || err.Error() != "provider readback failed" {
				t.Fatal("absent/null flags were interpreted as a final published state", field, null, err)
			}
		}
	}
}
