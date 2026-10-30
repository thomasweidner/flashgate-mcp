package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type normalizationMock struct {
	*publicationMock
	deleteIDs                            []int64
	ambiguousUpload, ambiguousDelete     bool
	afterDelete                          func()
	failPublic                           bool
	publicFaultName, publicFaultMode     string
	publicReads, beforeFirstDeletePublic []string
}

func (m *normalizationMock) Upload(ctx context.Context, id int64, a publicationAsset, r io.Reader) error {
	err := m.publicationMock.Upload(ctx, id, a, r)
	if err == nil && m.ambiguousUpload {
		return errors.New("controlled ambiguous accepted upload")
	}
	return err
}
func (m *normalizationMock) DeleteAsset(_ context.Context, id int64) error {
	if len(m.deleteIDs) == 0 {
		m.beforeFirstDeletePublic = append([]string{}, m.publicReads...)
	}
	m.deleteIDs = append(m.deleteIDs, id)
	if err := m.write(); err != nil {
		return err
	}
	for i, a := range m.state.Release.Assets {
		if a.ID == id {
			m.state.Release.Assets = append(m.state.Release.Assets[:i], m.state.Release.Assets[i+1:]...)
			delete(m.data, id)
			if m.afterDelete != nil {
				m.afterDelete()
			}
			if m.ambiguousDelete {
				return errors.New("controlled ambiguous accepted deletion")
			}
			return nil
		}
	}
	return errors.New("asset absent")
}

func normalizationFixture(t *testing.T) (string, normalizationPlan, *normalizationMock) {
	t.Helper()
	dir, original := publicationFixture(t)
	legacy, err := publicationEvidenceAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	original.Assets = legacy
	assets, _ := supplyJSON(legacy)
	original.Authorization.AssetsSHA256 = supplyDigest(assets)
	auth, _ := supplyJSON(original.Authorization)
	original.AuthorizationSHA256 = supplyDigest(auth)
	manifest, _ := supplyJSON(original)
	m := &normalizationMock{publicationMock: newPublicationMock(original)}
	ctx := context.Background()
	_, _ = m.CreateDraft(ctx, original)
	for _, a := range legacy {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(a.Path)))
		if err != nil {
			t.Fatal(err)
		}
		if err := m.Upload(ctx, 789, a, bytes.NewReader(b)); err != nil {
			t.Fatal(err)
		}
	}
	a := publicationAsset{"publication-manifest.json", "", int64(len(manifest)), supplyDigest(manifest)}
	if err := m.Upload(ctx, 789, a, bytes.NewReader(manifest)); err != nil {
		t.Fatal(err)
	}
	if err := m.Publish(ctx, 789, original); err != nil {
		t.Fatal(err)
	}
	m.writes = 0
	plan, err := prepareNormalization(ctx, m, dir, original, supplyDigest(manifest), 789)
	if err != nil {
		t.Fatal(err)
	}
	return dir, plan, m
}

func normalizationApproval(t *testing.T, m publicationProvider, p normalizationPlan, action string) normalizationAuthorization {
	t.Helper()
	state, err := normalizationReadback(context.Background(), m, p, p.OriginalAssets, action == normalizationDelete)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := supplyJSON(p)
	snapshot, _ := supplyJSON(state)
	return normalizationAuthorization{"flashgate-publication-normalization-authorization/v1", action, publicationRepository, p.ReleaseID, supplyDigest(data), supplyDigest(snapshot), "fixture-separate-" + action}
}
func normalizationExecute(t *testing.T, m normalizationProvider, dir string, p normalizationPlan, a normalizationAuthorization, journal string) error {
	t.Helper()
	data, _ := supplyJSON(a)
	return executeNormalization(context.Background(), m, dir, p, a, supplyDigest(data), journal)
}

func TestPublicationNormalizationSeparateActionsAndReadback(t *testing.T) {
	dir, p, m := normalizationFixture(t)
	if len(p.OriginalAssets) != 40 || len(p.Retain) != 4 || len(p.Delete) != 36 {
		t.Fatal("historical partition")
	}
	upload := normalizationApproval(t, m, p, normalizationUpload)
	if err := normalizationExecute(t, m, dir, p, upload, filepath.Join(t.TempDir(), "upload.jsonl")); err != nil {
		t.Fatal(err)
	}
	if m.writes != 1 || len(m.state.Release.Assets) != 41 || len(m.deleteIDs) != 0 {
		t.Fatal("upload crossed deletion boundary")
	}
	remove := normalizationApproval(t, m, p, normalizationDelete)
	journal := filepath.Join(t.TempDir(), "delete.jsonl")
	if err := normalizationExecute(t, m, dir, p, remove, journal); err != nil {
		t.Fatal(err)
	}
	if m.writes != 37 || len(m.deleteIDs) != 36 || len(m.state.Release.Assets) != 5 {
		t.Fatal("normalization write/inventory mismatch")
	}
	for _, a := range p.Retain {
		for _, id := range m.deleteIDs {
			if id == a.ID {
				t.Fatal("retained archive deleted")
			}
		}
	}
	if _, err := reconcileNormalization(context.Background(), m, dir, p); err != nil {
		t.Fatal("final read-only reconciliation", err)
	}
	before := m.writes
	if err := normalizationExecute(t, m, dir, p, remove, journal); err == nil || m.writes != before {
		t.Fatal("consumed deletion repeated")
	}
	log, err := os.ReadFile(journal)
	if err != nil || !bytes.Contains(log, []byte("VERIFIED_FIVE_ASSET_RELEASE")) {
		t.Fatal("final intent/readback missing", err)
	}
}

func TestPublicationNormalizationAmbiguityIsReadOnly(t *testing.T) {
	for _, action := range []string{normalizationUpload, normalizationDelete} {
		t.Run(action, func(t *testing.T) {
			dir, p, m := normalizationFixture(t)
			if action == normalizationDelete {
				a := normalizationApproval(t, m, p, normalizationUpload)
				if err := normalizationExecute(t, m, dir, p, a, filepath.Join(t.TempDir(), "up.jsonl")); err != nil {
					t.Fatal(err)
				}
				m.ambiguousDelete = true
			} else {
				m.ambiguousUpload = true
			}
			a := normalizationApproval(t, m, p, action)
			before := m.writes
			journal := filepath.Join(t.TempDir(), "attempt.jsonl")
			if err := normalizationExecute(t, m, dir, p, a, journal); err == nil || m.writes != before+1 {
				t.Fatal("ambiguous write retried or accepted", err)
			}
			written := m.writes
			if _, err := reconcileNormalization(context.Background(), m, dir, p); err != nil {
				t.Fatal("actual accepted ambiguous state not reconciled", err)
			}
			if m.writes != written {
				t.Fatal("reconciliation wrote")
			}
			if err := normalizationExecute(t, m, dir, p, a, journal); err == nil || m.writes != written {
				t.Fatal("consumed ambiguous operation repeated")
			}
		})
	}
}

func TestPublicationNormalizationEveryDeleteFailure(t *testing.T) {
	dir, p, original := normalizationFixture(t)
	upload := normalizationApproval(t, original, p, normalizationUpload)
	if err := normalizationExecute(t, original, dir, p, upload, filepath.Join(t.TempDir(), "upload.jsonl")); err != nil {
		t.Fatal(err)
	}
	for fail := 1; fail <= 36; fail++ {
		m := &normalizationMock{publicationMock: &publicationMock{state: normalizationSnapshot(original.state), data: map[int64][]byte{}, failWrite: fail}}
		for id, b := range original.data {
			m.data[id] = b
		}
		a := normalizationApproval(t, m, p, normalizationDelete)
		if err := normalizationExecute(t, m, dir, p, a, filepath.Join(t.TempDir(), "delete.jsonl")); err == nil || m.writes != fail || len(m.deleteIDs) != fail {
			t.Fatalf("delete failure %d not consumed exactly once: %v", fail, err)
		}
		if _, err := reconcileNormalization(context.Background(), m, dir, p); err != nil {
			t.Fatal("failed-prefix readback", err)
		}
		if m.writes != fail {
			t.Fatal("readback retried write")
		}
	}
}

func TestPublicationNormalizationScopeAndPrestateNegatives(t *testing.T) {
	dir, p, m := normalizationFixture(t)
	for _, name := range []string{"missing-original", "duplicate-id", "zero-id", "archive-in-delete", "checksum", "old-auth", "wrong-action", "wrong-release", "stale-state", "version", "notes", "published-time", "missing-digest", "swapped-id"} {
		t.Run(name, func(t *testing.T) {
			data, _ := supplyJSON(p)
			var candidate normalizationPlan
			if err := json.Unmarshal(data, &candidate); err != nil {
				t.Fatal(err)
			}
			mock := &normalizationMock{publicationMock: &publicationMock{state: normalizationSnapshot(m.state), data: m.data}}
			a := normalizationApproval(t, m, p, normalizationUpload)
			switch name {
			case "missing-original":
				candidate.OriginalAssets = candidate.OriginalAssets[:39]
			case "duplicate-id":
				candidate.OriginalAssets[1].ID = candidate.OriginalAssets[0].ID
			case "zero-id":
				candidate.OriginalAssets[0].ID = 0
			case "archive-in-delete":
				candidate.Delete[0] = candidate.Retain[0]
			case "checksum":
				candidate.Checksum.SHA256 = strings.Repeat("0", 64)
			case "old-auth":
				a.OriginalApproval = p.Original.Authorization.OriginalApproval
			case "wrong-action":
				a.Action = "CREATE_DRAFT_UPLOAD_READBACK_PUBLISH_PRERELEASE"
			case "wrong-release":
				a.ReleaseID++
			case "stale-state":
				a.PrestateSHA256 = strings.Repeat("0", 64)
			case "version":
				candidate.Original.Authorization.Version = "0.4.0"
			case "notes":
				mock.state.Release.Body += "changed"
			case "published-time":
				mock.state.Release.PublishedAt = "2026-10-26T00:00:00Z"
			case "missing-digest":
				mock.state.Release.Assets[0].Digest = ""
			case "swapped-id":
				mock.state.Release.Assets[0].ID, mock.state.Release.Assets[1].ID = mock.state.Release.Assets[1].ID, mock.state.Release.Assets[0].ID
			}
			// Rebind a changed plan so semantic guards, not only the plan hash, are exercised.
			encoded, _ := supplyJSON(candidate)
			a.PlanSHA256 = supplyDigest(encoded)
			if err := normalizationExecute(t, mock, dir, candidate, a, filepath.Join(t.TempDir(), "hold.jsonl")); err == nil || mock.writes != 0 {
				t.Fatal("unsafe normalization accepted", err)
			}
		})
	}
	// Unavailable original evidence is never replaced by provider assertions.
	if err := os.Remove(filepath.Join(dir, "supply-chain-index.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := reconcileNormalization(context.Background(), m, dir, p); err == nil || m.writes != 0 {
		t.Fatal("missing evidence accepted")
	}
}

func TestPublicationNormalizationProviderDeleteOneShot(t *testing.T) {
	for _, status := range []int{204, 302, 403, 404, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "DELETE" || r.URL.Path != "/repos/"+publicationRepository+"/releases/assets/42" {
					t.Error("wrong delete target")
				}
				w.Header().Set("Location", "https://example.invalid")
				w.WriteHeader(status)
			}))
			defer server.Close()
			client := server.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			g := &githubPublication{client: client, token: "test-only", api: server.URL}
			err := g.DeleteAsset(context.Background(), 42)
			if (err == nil) != (status == 204) || calls != 1 {
				t.Fatal("delete status/retry boundary", err, calls)
			}
			if err := g.DeleteAsset(context.Background(), 0); err == nil || calls != 1 {
				t.Fatal("invalid ID dispatched")
			}
		})
	}
}

func (m *normalizationMock) ReadPublicAsset(ctx context.Context, plan publicationPlan, asset publicationAsset) (io.ReadCloser, error) {
	m.publicReads = append(m.publicReads, asset.Name)
	if m.failPublic || (m.publicFaultName == asset.Name && m.publicFaultMode == "unavailable") {
		return nil, errors.New("controlled public download failure")
	}
	stream, err := m.publicationMock.ReadPublicAsset(ctx, plan, asset)
	if err != nil || m.publicFaultName != asset.Name {
		return stream, err
	}
	data, readErr := io.ReadAll(stream)
	if err := errors.Join(readErr, stream.Close()); err != nil {
		return nil, err
	}
	switch m.publicFaultMode {
	case "tampered":
		data[0] ^= 1
	case "incomplete":
		data = data[:len(data)-1]
	default:
		return nil, errors.New("unknown public fault mode")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func TestPublicationNormalizationPostwriteAndPublicFailures(t *testing.T) {
	for _, kind := range []string{"checksum-id-drift", "public-download"} {
		t.Run(kind, func(t *testing.T) {
			dir, p, m := normalizationFixture(t)
			upload := normalizationApproval(t, m, p, normalizationUpload)
			if err := normalizationExecute(t, m, dir, p, upload, filepath.Join(t.TempDir(), "upload.jsonl")); err != nil {
				t.Fatal(err)
			}
			remove := normalizationApproval(t, m, p, normalizationDelete)
			m.writes = 0
			if kind == "checksum-id-drift" {
				m.afterDelete = func() {
					for i, a := range m.state.Release.Assets {
						if a.Name == publicationChecksumsName {
							m.state.Release.Assets[i].ID += 1000
							m.data[a.ID+1000] = m.data[a.ID]
							delete(m.data, a.ID)
							break
						}
					}
				}
			} else {
				// Availability can fail after a successful pre-delete gate.
				m.afterDelete = func() { m.failPublic = true }
			}
			if err := normalizationExecute(t, m, dir, p, remove, filepath.Join(t.TempDir(), "delete.jsonl")); err == nil {
				t.Fatal("postwrite failure accepted")
			}
			expected := 36
			if kind == "checksum-id-drift" {
				expected = 1
			}
			if m.writes != expected {
				t.Fatal("failure caused retry or extra mutation", m.writes)
			}
		})
	}
}

func TestPublicationNormalizationOfflineCLIAndTransport(t *testing.T) {
	dir, p, m := normalizationFixture(t)
	work := t.TempDir()
	original, _ := supplyJSON(p.Original)
	state, _ := supplyJSON(normalizationSnapshot(m.state))
	expected, _ := supplyJSON(p)
	originalPath, statePath, output := filepath.Join(work, "original.json"), filepath.Join(work, "state.json"), filepath.Join(work, "plan.json")
	for name, data := range map[string][]byte{originalPath: original, statePath: state} {
		if err := os.WriteFile(name, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	args := []string{"--dir", dir, "--publication-manifest", originalPath, "--publication-manifest-sha256", p.OriginalSHA256, "--state", statePath, "--release-id", "789", "--output", output}
	if err := runNormalization(args, "preflight"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(got, expected) || m.writes != 0 {
		t.Fatal("offline plan/side-effect mismatch", err)
	}
	if _, err := readNormalizationPlan(output, supplyDigest(got)); err != nil {
		t.Fatal(err)
	}
	if _, err := readNormalizationPlan(output, strings.Repeat("0", 64)); err == nil {
		t.Fatal("untrusted plan accepted")
	}
	if err := runNormalization(append(args, "--authorization", "unused.json"), "preflight"); err == nil {
		t.Fatal("execution authorization silently ignored")
	}
	if err := runNormalization(append(args, "unexpected"), "preflight"); err == nil {
		t.Fatal("positional argument accepted")
	}
	args[5] = strings.Repeat("0", 64)
	if err := runNormalization(args, "preflight"); err == nil {
		t.Fatal("untrusted original accepted")
	}
}

func TestPublicationNormalizationPublicGateBeforeDelete(t *testing.T) {
	dir, plan, original := normalizationFixture(t)
	upload := normalizationApproval(t, original, plan, normalizationUpload)
	if err := normalizationExecute(t, original, dir, plan, upload, filepath.Join(t.TempDir(), "upload.jsonl")); err != nil {
		t.Fatal(err)
	}
	names := []string{publicationChecksumsName}
	for _, a := range plan.Retain {
		names = append(names, a.Asset.Name)
	}
	for _, name := range names {
		for _, mode := range []string{"unavailable", "tampered", "incomplete"} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				m := &normalizationMock{publicationMock: &publicationMock{state: normalizationSnapshot(original.state), data: map[int64][]byte{}}, publicFaultName: name, publicFaultMode: mode}
				for id, b := range original.data {
					m.data[id] = b
				}
				a := normalizationApproval(t, m, plan, normalizationDelete)
				journal := filepath.Join(t.TempDir(), "delete.jsonl")
				err := normalizationExecute(t, m, dir, plan, a, journal)
				if err == nil || !strings.Contains(err.Error(), "HOLD before first asset deletion") || m.writes != 0 || len(m.deleteIDs) != 0 || len(m.state.Release.Assets) != 41 {
					t.Fatal("public failure did not HOLD before all deletes", err, m.writes, m.deleteIDs)
				}
				log, err := os.ReadFile(journal)
				if err != nil || bytes.Contains(log, []byte("NORMALIZATION_DELETE")) {
					t.Fatal("delete intent despite failed public gate", err)
				}
				if _, err := reconcileNormalization(context.Background(), m, dir, plan); err != nil || m.writes != 0 {
					t.Fatal("read-only reconciliation failed", err)
				}
				m.publicFaultName = ""
				if err := normalizationExecute(t, m, dir, plan, a, journal); err == nil || m.writes != 0 {
					t.Fatal("consumed journal retried after public recovery", err)
				}
			})
		}
	}
	t.Run("all-public-downloads-succeed", func(t *testing.T) {
		m := &normalizationMock{publicationMock: &publicationMock{state: normalizationSnapshot(original.state), data: map[int64][]byte{}}}
		for id, b := range original.data {
			m.data[id] = b
		}
		a := normalizationApproval(t, m, plan, normalizationDelete)
		journal := filepath.Join(t.TempDir(), "delete.jsonl")
		if err := normalizationExecute(t, m, dir, plan, a, journal); err != nil {
			t.Fatal(err)
		}
		if m.writes != 36 || len(m.deleteIDs) != 36 || len(m.state.Release.Assets) != 5 {
			t.Fatal("success did not perform exactly 36 deletes", m.writes)
		}
		if strings.Join(m.beforeFirstDeletePublic, "\n") != strings.Join(names, "\n") || len(m.publicReads) != 10 {
			t.Fatal("all five public reads must precede first delete and repeat at closure", m.beforeFirstDeletePublic, m.publicReads)
		}
		log, err := os.ReadFile(journal)
		before, firstDelete := bytes.Index(log, []byte("VERIFIED_FIVE_ASSETS_BEFORE_DELETE")), bytes.Index(log, []byte("NORMALIZATION_DELETE"))
		if err != nil || before < 0 || firstDelete <= before {
			t.Fatal("durable public verification order missing", err)
		}
	})
}
