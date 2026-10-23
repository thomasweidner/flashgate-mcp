package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Map the permitted synthetic redirect host to a local TLS server. No test
// request reaches a provider, including when a redirect is unexpectedly sent.
func publicationLocalDownloadTransport(t *testing.T, server *httptest.Server, observe func(*http.Request)) http.RoundTripper {
	t.Helper()
	local, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return publicationTransport(func(r *http.Request) (*http.Response, error) {
		observe(r)
		clone := r.Clone(r.Context())
		u := *r.URL
		u.Scheme, u.Host = local.Scheme, local.Host
		clone.URL = &u
		return server.Client().Transport.RoundTrip(clone)
	})
}

func TestPublicationDownloadHTTPDiagnostics(t *testing.T) {
	const signed = "https://fixture.blob.core.windows.net/blob?sig=synthetic-secret&se=synthetic-expiry"
	for _, class := range []publicationDownloadClass{actionsArtifactDownload, releaseAssetDownload} {
		for _, redirected := range []bool{false, true} {
			for _, status := range []int{200, 401, 403, 404, 500, 303, 308} {
				t.Run(fmt.Sprintf("class%d/redirect%t/status%d", class, redirected, status), func(t *testing.T) {
					location := signed
					if class == releaseAssetDownload {
						location = "https://release-assets.githubusercontent.com/blob?sig=synthetic-secret"
					}
					calls := 0
					server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Set-Cookie", "session=synthetic-cookie")
						w.Header().Set("X-Sensitive", "synthetic-header")
						w.Header().Set("Location", location)
						if redirected && r.URL.Path != "/blob" {
							w.WriteHeader(http.StatusFound)
							return
						}
						w.WriteHeader(status)
						_, _ = io.WriteString(w, "synthetic-body "+location)
					}))
					defer server.Close()
					jar, err := cookiejar.New(nil)
					if err != nil {
						t.Fatal(err)
					}
					u, _ := url.Parse(location)
					jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "synthetic-cookie"}})
					g := &githubPublication{api: server.URL, token: "synthetic-token"}
					g.client = &http.Client{Timeout: time.Second, Jar: jar, Transport: publicationLocalDownloadTransport(t, server, func(r *http.Request) {
						calls++
						if calls == 1 {
							if r.Header.Get("Authorization") != "Bearer synthetic-token" {
								t.Error("initial API request lost credential")
							}
							return
						}
						if r.URL.String() != location {
							t.Error("signed redirect input changed")
						}
						for _, key := range []string{"Authorization", "Proxy-Authorization", "Cookie", "X-Sensitive", "X-GitHub-Api-Version", "Accept", "Referer"} {
							if r.Header.Get(key) != "" {
								t.Error("unvetted redirect header", key)
							}
						}
					})}
					endpoint := "/actions/artifacts/456/zip"
					if class == releaseAssetDownload {
						endpoint = "/releases/assets/456"
					}
					stream, err := g.download(context.Background(), endpoint, class)
					wantCalls := 1
					if redirected {
						wantCalls++
					}
					if calls != wantCalls {
						t.Fatal("unexpected retry, fallback or extra request", calls)
					}
					if status == http.StatusOK {
						if err != nil || stream == nil {
							t.Fatal("successful download rejected", err)
						}
						data, readErr := io.ReadAll(stream)
						if err := errors.Join(readErr, stream.Close()); err != nil || string(data) != "synthetic-body "+location {
							t.Fatal("successful payload changed", err)
						}
						return
					}
					var diagnostic *publicationDownloadHTTPError
					if stream != nil || !errors.As(err, &diagnostic) || diagnostic.statusCode != status || diagnostic.redirected != redirected {
						t.Fatal("status/phase classification missing", err)
					}
					phase := "api_response"
					if redirected {
						phase = "redirect_response"
					}
					if err.Error() != fmt.Sprintf("download unavailable (HTTP %d, phase=%s)", status, phase) {
						t.Fatal("diagnostic contains unexpected data", err)
					}
				})
			}
		}
	}
}

func TestPublicationDownloadTransportDiagnostics(t *testing.T) {
	for _, redirected := range []bool{false, true} {
		for _, canceled := range []bool{false, true} {
			t.Run(fmt.Sprintf("redirect%t/cancel%t", redirected, canceled), func(t *testing.T) {
				calls := 0
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				g := &githubPublication{api: githubAPI, token: "synthetic-token"}
				g.client = &http.Client{Transport: publicationTransport(func(r *http.Request) (*http.Response, error) {
					calls++
					if redirected && calls == 1 {
						return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://objects.githubusercontent.com/blob?sig=synthetic-secret"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
					}
					if canceled {
						cancel()
						return nil, ctx.Err()
					}
					return nil, errors.New("synthetic-token Authorization Cookie https://objects.githubusercontent.com/blob?sig=synthetic-secret")
				})}
				stream, err := g.ReadAsset(ctx, 1)
				want := "provider transport failed or ambiguous"
				wantCalls := 1
				if redirected {
					want, wantCalls = "download read failed", 2
				}
				if stream != nil || err == nil || err.Error() != want || calls != wantCalls {
					t.Fatal("transport failed open, leaked data or retried", err, calls)
				}
			})
		}
	}
}

type publicationDownloadErrorProvider struct {
	*publicationMock
	err error
}

func (p *publicationDownloadErrorProvider) ReadAsset(context.Context, int64) (io.ReadCloser, error) {
	return nil, p.err
}

func TestPublicationReadbackDownloadDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"api", &publicationDownloadHTTPError{statusCode: 403}, "remote asset readback unavailable: download unavailable (HTTP 403, phase=api_response)"},
		{"redirect", &publicationDownloadHTTPError{statusCode: 500, redirected: true}, "remote asset readback unavailable: download unavailable (HTTP 500, phase=redirect_response)"},
		{"wrapped", fmt.Errorf("synthetic-secret: %w", &publicationDownloadHTTPError{statusCode: 404}), "remote asset readback unavailable: download unavailable (HTTP 404, phase=api_response)"},
		{"unknown", errors.New("Authorization Cookie https://objects.githubusercontent.com/blob?sig=synthetic-secret"), "remote asset readback unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := publicationUnitPlan(t)
			mock := newPublicationMock(plan)
			id, err := mock.CreateDraft(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			asset := plan.Assets[0]
			mock.state.Release.Assets = []remotePublicationAsset{{ID: 1, Name: asset.Name, Size: asset.Size, State: "uploaded"}}
			provider := &publicationDownloadErrorProvider{mock, tc.err}
			err = publicationReadback(context.Background(), provider, plan, id, true, []publicationAsset{asset})
			if err == nil || err.Error() != tc.want || mock.writes != 1 {
				t.Fatal("readback dropped safe status, leaked context or wrote", err)
			}
		})
	}
}
