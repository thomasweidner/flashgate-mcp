package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func securityTestProvider(t *testing.T, handler http.HandlerFunc) securityOSV {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	p := newSecurityOSV()
	p.base = server.URL
	p.client.Transport = server.Client().Transport
	return p
}
func TestSecurityOSVBatchPaginationAndDetails(t *testing.T) {
	modules := []securityModule{{"example.org/a", "v1.2.3", "DIRECT", "BUILD"}, {"example.org/b", "v2.0.0", "INDIRECT", "RUNTIME"}}
	posts, gets := 0, 0
	p := securityTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("credential forwarding")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			gets++
			id := strings.TrimPrefix(r.URL.Path, "/vulns/")
			name, version := modules[0].Name, modules[0].Version
			if id == "GO-2026-2000" {
				name, version = modules[1].Name, modules[1].Version
			}
			_ = json.NewEncoder(w).Encode(securityTestAdvisory(id, name, version, "HIGH"))
			return
		}
		posts++
		var request struct {
			Queries []osvQuery `json:"queries"`
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil {
			t.Error("invalid request")
		}
		for _, q := range request.Queries {
			if q.Package.Ecosystem != "Go" || q.Package.Name == "" || q.Version == "" {
				t.Error(q)
			}
		}
		if posts == 1 {
			if len(request.Queries) != 2 || request.Queries[0].Package.Name != modules[0].Name || request.Queries[1].Package.Name != modules[1].Name {
				t.Error(request)
			}
			_, _ = w.Write([]byte(`{"results":[{"vulns":[{"id":"GO-2026-1000","modified":"2026-11-01T11:00:00Z"}],"next_page_token":"next-a"},{"vulns":[{"id":"GO-2026-2000","modified":"2026-11-01T11:00:00Z"}]}]}`))
		} else {
			if len(request.Queries) != 1 || request.Queries[0].Package.Name != modules[0].Name || request.Queries[0].PageToken != "next-a" {
				t.Error("pagination lost original position", request)
			}
			_, _ = w.Write([]byte(`{"results":[{"vulns":[{"id":"GO-2026-1001","modified":"2026-11-01T11:00:00Z"}]}]}`))
		}
	})
	s := p.capture(context.Background(), securityIdentity{}, modules, securityTestNow)
	if len(s.Errors) > 0 || posts != 2 || gets != 3 || len(s.Queries[0].Advisories) != 2 || len(s.Queries[1].Advisories) != 1 || !s.Queries[0].Complete || !s.Queries[1].Complete {
		t.Fatal(s, posts, gets)
	}
	r := securityTestAssess(modules, s)
	if r.Status != "COMPLETE" || len(r.Findings) != 3 {
		t.Fatal(r)
	}
}

func TestSecurityOSVResponseNegatives(t *testing.T) {
	modules := []securityModule{{"example.org/a", "v1.2.3", "DIRECT", "BUILD"}}
	for name, body := range map[string]string{
		"errorItem": `{"results":[{"error":"private body"}]}`,
		"nullVulns": `{"results":[{"vulns":null}]}`,
		"nullToken": `{"results":[{"next_page_token":null}]}`,
		"malformed": `{`, "truncated": `{"results":[`, "null": `null`, "nullResults": `{"results":null}`, "nullItem": `{"results":[null]}`, "missing": `{}`, "count": `{"results":[]}`, "extra": `{"results":[{},{}]}`, "duplicate": `{"results":[{}],"results":[{}]}`, "trailing": `{"results":[{}]} {}`,
		"oversize": strings.Repeat(" ", securityMaxResponse+1), "badID": `{"results":[{"vulns":[{"id":"../private","modified":"2026-11-01T11:00:00Z"}]}]}`,
		"badTime": `{"results":[{"vulns":[{"id":"GO-2026-1000"}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			p := securityTestProvider(t, func(w http.ResponseWriter, r *http.Request) { calls++; _, _ = w.Write([]byte(body)) })
			s := p.capture(context.Background(), securityIdentity{}, modules, securityTestNow)
			if len(s.Errors) == 0 || s.Queries[0].Complete || calls != 1 {
				t.Fatal(s, calls)
			}
		})
	}
	for name, handler := range map[string]http.HandlerFunc{
		"http": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(503)
			_, _ = w.Write([]byte("sensitive-private-body"))
		},
		"redirect": func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/unexpected", 302) },
		"cycle": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"results":[{"next_page_token":"same"}]}`))
		},
		"detail": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"results":[{"vulns":[{"id":"GO-2026-1000","modified":"2026-11-01T11:00:00Z"}]}]}`))
				return
			}
			_ = json.NewEncoder(w).Encode(securityTestAdvisory("GO-2026-9999", modules[0].Name, modules[0].Version, ""))
		},
		"detailModified": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`{"results":[{"vulns":[{"id":"GO-2026-1000","modified":"2026-11-01T10:00:00Z"}]}]}`))
				return
			}
			_ = json.NewEncoder(w).Encode(securityTestAdvisory("GO-2026-1000", modules[0].Name, modules[0].Version, ""))
		},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			p := securityTestProvider(t, func(w http.ResponseWriter, r *http.Request) { calls++; handler(w, r) })
			s := p.capture(context.Background(), securityIdentity{}, modules, securityTestNow)
			if len(s.Errors) == 0 {
				t.Fatal(s)
			}
			data, _ := json.Marshal(s)
			if strings.Contains(string(data), "sensitive-private-body") {
				t.Fatal("raw error leak")
			}
			if (name == "http" || name == "redirect") && calls != 1 {
				t.Fatal("redirect/retry", calls)
			}
		})
	}
}

func TestSecurityOSVTimeoutAndBoundedBatches(t *testing.T) {
	modules := []securityModule{{"example.org/a", "v1.2.3", "DIRECT", "BUILD"}}
	p := securityTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
		w.WriteHeader(504)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if s := p.capture(ctx, securityIdentity{}, modules, securityTestNow); len(s.Errors) == 0 || s.Queries[0].Complete {
		t.Fatal(s)
	}
	calls := 0
	p = securityTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request struct {
			Queries []osvQuery `json:"queries"`
		}
		_ = json.NewDecoder(r.Body).Decode(&request)
		if len(request.Queries) > 16 {
			t.Error("unbounded batch")
		}
		results := make([]osvResult, len(request.Queries))
		_ = json.NewEncoder(w).Encode(struct {
			Results []osvResult `json:"results"`
		}{results})
	})
	for len(modules) < 33 {
		modules = append(modules, securityModule{Name: "example.org/other", Version: "v1.0.0"})
	}
	s := p.capture(context.Background(), securityIdentity{}, modules, securityTestNow)
	if len(s.Errors) > 0 || calls != 3 {
		t.Fatal(s, calls)
	}
}
