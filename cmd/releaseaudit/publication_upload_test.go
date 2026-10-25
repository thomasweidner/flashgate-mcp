package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type publicationUploadFailureProvider struct {
	*publicationContinuationMock
	upload func(context.Context, int64, publicationAsset, io.Reader) error
	calls  int
}

func (p *publicationUploadFailureProvider) Upload(ctx context.Context, id int64, asset publicationAsset, reader io.Reader) error {
	p.calls++
	return p.upload(ctx, id, asset, reader)
}

func TestPublicationUploadStatusReachesContinuationHoldWithoutPrivateErrors(t *testing.T) {
	dir, plan := publicationFixture(t)
	private := "private-provider-body-header-query-and-file-close-details"
	for _, scenario := range []struct {
		name   string
		status int
		err    error
	}{
		{name: "unsupported-media", status: http.StatusUnsupportedMediaType},
		{name: "duplicate", status: http.StatusUnprocessableEntity},
		{name: "upstream", status: http.StatusBadGateway},
		{name: "redirect", status: http.StatusTemporaryRedirect},
		{name: "untyped-provider-error", err: errors.New(private + " HTTP 415")},
		{name: "joined-close-error", status: http.StatusUnsupportedMediaType, err: errors.Join(&publicationUploadHTTPError{statusCode: http.StatusUnsupportedMediaType}, errors.New(private))},
		{name: "invalid-low-status", err: &publicationUploadHTTPError{statusCode: -1}},
		{name: "invalid-high-status", err: &publicationUploadHTTPError{statusCode: 1000}},
		{name: "typed-nil", err: (*publicationUploadHTTPError)(nil)},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != http.MethodPost || r.Header.Get("Accept") != "application/vnd.github+json" || r.Header.Get("Content-Type") != "application/octet-stream" {
					t.Error("continuation upload contract changed")
				}
				w.Header().Set("X-Private-Detail", private)
				if scenario.status == http.StatusTemporaryRedirect {
					w.Header().Set("Location", "/another-upload?"+private)
				}
				w.WriteHeader(scenario.status)
				_, _ = io.WriteString(w, private)
			}))
			defer server.Close()
			client := server.Client()
			client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
			g := &githubPublication{client: client, token: "test-only", api: server.URL, uploads: server.URL}
			m := &publicationUploadFailureProvider{publicationContinuationMock: publicationEmptyDraft(t, plan)}
			m.upload = func(ctx context.Context, id int64, asset publicationAsset, reader io.Reader) error {
				if scenario.err != nil {
					return scenario.err
				}
				return g.Upload(ctx, id, asset, reader)
			}
			journal := filepath.Join(t.TempDir(), "attempt.jsonl")
			err := executePublicationTarget(context.Background(), m, plan, dir, journal, 789, supplyDigest([]byte("fixture-new-approval")))
			want := "HOLD: asset write failed or ambiguous; no retry"
			if scenario.status != 0 {
				want = fmt.Sprintf("HOLD: asset write failed or ambiguous (HTTP %d); no retry", scenario.status)
			}
			if err == nil || err.Error() != want || strings.Contains(err.Error(), private) {
				t.Fatalf("unsafe or missing full-path upload diagnostic: %v", err)
			}
			wantRequests := 1
			if scenario.err != nil {
				wantRequests = 0
			}
			if requests != wantRequests || m.calls != 1 || m.creates != 0 || m.writes != 0 || m.publicReads != 0 || !m.state.Release.Draft {
				t.Fatal("failed upload retried, redirected, recreated or continued")
			}
			data, readErr := os.ReadFile(journal)
			if readErr != nil {
				t.Fatal(readErr)
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) != 2 {
				t.Fatal("unexpected post-failure intent")
			}
			var last struct{ Phase, Action string }
			if json.Unmarshal([]byte(lines[1]), &last) != nil || last.Phase != "EXACT_ASSET_TRANSFER" || last.Action != plan.Assets[0].Name {
				t.Fatal("failed upload journal boundary changed")
			}
		})
	}
}

func TestPublicationRequestRequiresBodyContentTypeBeforeDispatch(t *testing.T) {
	calls := 0
	g := &githubPublication{api: githubAPI, uploads: githubUploads, client: &http.Client{Transport: publicationTransport(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("must not dispatch")
	})}}
	_, err := g.request(context.Background(), http.MethodPost, g.endpoint("/releases"), "application/vnd.github+json", "", strings.NewReader("body"), 4)
	if err == nil || err.Error() != "provider content type missing" || calls != 0 {
		t.Fatal("untyped request body reached provider")
	}
}
