package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

const securitySnapshotSchema = "flashgate-security-snapshot/v1"
const securityMaxResponse = 1 << 20
const securityMaxAdvisories = 256
const securityMaxModules = 128

var securityIDPattern = regexp.MustCompile(`^(GO-[0-9]{4}-[0-9]{4,}|OSV-[A-Za-z0-9_.-]+|GHSA-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}|CVE-[0-9]{4}-[0-9]{4,})$`)
var securityModulePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~+/-]*$`)
var securityCVSSPattern = regexp.MustCompile(`^CVSS:(2\.0|3\.[01]|4\.0)(/[A-Za-z]{1,6}:[A-Za-z0-9.]+)+$`)

type osvPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}
type osvSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}
type osvEvent struct {
	Introduced   string `json:"introduced,omitempty"`
	Fixed        string `json:"fixed,omitempty"`
	LastAffected string `json:"last_affected,omitempty"`
	Limit        string `json:"limit,omitempty"`
}
type osvRange struct {
	Type   string     `json:"type"`
	Events []osvEvent `json:"events"`
}
type osvDatabase struct {
	Severity string `json:"severity,omitempty"`
}
type osvAffected struct {
	Package  osvPackage    `json:"package"`
	Versions []string      `json:"versions,omitempty"`
	Ranges   []osvRange    `json:"ranges,omitempty"`
	Severity []osvSeverity `json:"severity,omitempty"`
	Database osvDatabase   `json:"database_specific"`
}
type osvAdvisory struct {
	ID        string        `json:"id"`
	Modified  string        `json:"modified"`
	Withdrawn string        `json:"withdrawn,omitempty"`
	Aliases   []string      `json:"aliases"`
	Affected  []osvAffected `json:"affected"`
	Severity  []osvSeverity `json:"severity,omitempty"`
	Database  osvDatabase   `json:"database_specific"`
}
type securityQuery struct {
	Name       string   `json:"name"`
	Version    string   `json:"version"`
	Complete   bool     `json:"complete"`
	Advisories []string `json:"advisories"`
}
type securitySnapshot struct {
	Schema     string           `json:"schema"`
	Provider   string           `json:"provider"`
	CapturedAt string           `json:"capturedAt"`
	Identity   securityIdentity `json:"inventory"`
	Queries    []securityQuery  `json:"queries"`
	Advisories []osvAdvisory    `json:"advisories"`
	Errors     []string         `json:"errors"`
}

// The transport has one fixed public authority, no credentials, cookies,
// redirects, retries or endpoint override. Tests inject a loopback server.
type securityOSV struct {
	client *http.Client
	base   string
}

func newSecurityOSV() securityOSV {
	return securityOSV{&http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, "https://api.osv.dev/v1"}
}
func (p securityOSV) request(ctx context.Context, method, suffix string, body []byte, value any) error {
	req, err := http.NewRequestWithContext(ctx, method, p.base+suffix, bytes.NewReader(body))
	if err != nil {
		return errors.New("OSV_REQUEST_INVALID")
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := p.client.Do(req)
	if err != nil {
		return errors.New("OSV_TRANSPORT_ERROR")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("OSV_HTTP_ERROR")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, securityMaxResponse+1))
	if err != nil || len(data) > securityMaxResponse {
		return errors.New("OSV_RESPONSE_SIZE_OR_READ_ERROR")
	}
	// OSV adds fields compatibly. Ignore unconsumed fields, but never duplicate
	// keys, trailing JSON, absent result lists or invalid consumed identities.
	if rejectDuplicateJSONKeys(data) != nil || json.Unmarshal(data, value) != nil {
		return errors.New("OSV_RESPONSE_INVALID")
	}
	return nil
}

type osvQuery struct {
	Package   osvPackage `json:"package"`
	Version   string     `json:"version"`
	PageToken string     `json:"page_token,omitempty"`
}
type osvHit struct {
	ID       string `json:"id"`
	Modified string `json:"modified"`
}
type osvResult struct {
	Vulns []osvHit `json:"vulns,omitempty"`
	Next  string   `json:"next_page_token,omitempty"`
}

func (r *osvResult) UnmarshalJSON(data []byte) error {
	var wire struct {
		Vulns json.RawMessage `json:"vulns"`
		Next  json.RawMessage `json:"next_page_token"`
	}
	if err := strictCandidateBytes(data, &wire); err != nil {
		return errors.New("OSV_RESULT_SHAPE")
	}
	if len(wire.Vulns) > 0 {
		if bytes.Equal(bytes.TrimSpace(wire.Vulns), []byte("null")) || strictCandidateBytes(wire.Vulns, &r.Vulns) != nil {
			return errors.New("OSV_VULNS_SHAPE")
		}
	}
	if len(wire.Next) > 0 {
		if bytes.Equal(bytes.TrimSpace(wire.Next), []byte("null")) || json.Unmarshal(wire.Next, &r.Next) != nil {
			return errors.New("OSV_TOKEN_SHAPE")
		}
	}
	return nil
}

func (p securityOSV) capture(ctx context.Context, identity securityIdentity, modules []securityModule, now time.Time) securitySnapshot {
	s := securitySnapshot{Schema: securitySnapshotSchema, Provider: "OSV", CapturedAt: now.UTC().Format(time.RFC3339), Identity: identity, Queries: []securityQuery{}, Advisories: []osvAdvisory{}, Errors: []string{}}
	if len(modules) == 0 || len(modules) > securityMaxModules {
		s.Errors = append(s.Errors, "QUERY_MODULE_LIMIT")
		return s
	}
	for _, m := range modules {
		s.Queries = append(s.Queries, securityQuery{Name: m.Name, Version: m.Version, Advisories: []string{}})
	}
	details := map[string]osvAdvisory{}
	for start := 0; start < len(modules); start += 16 {
		end := min(start+16, len(modules))
		pending := []int{}
		tokens := map[int]string{}
		seenTokens := map[int]map[string]bool{}
		seenIDs := map[int]map[string]bool{}
		for i := start; i < end; i++ {
			pending = append(pending, i)
			seenTokens[i] = map[string]bool{}
			seenIDs[i] = map[string]bool{}
		}
		for page := 0; len(pending) > 0; page++ {
			if page >= 16 {
				s.Errors = append(s.Errors, "OSV_PAGE_LIMIT")
				return s
			}
			queries := []osvQuery{}
			for _, i := range pending {
				m := modules[i]
				queries = append(queries, osvQuery{osvPackage{m.Name, "Go"}, m.Version, tokens[i]})
			}
			body, _ := json.Marshal(struct {
				Queries []osvQuery `json:"queries"`
			}{queries})
			var response struct {
				Results []*osvResult `json:"results"`
			}
			var raw json.RawMessage
			if err := p.request(ctx, http.MethodPost, "/querybatch", body, &raw); err != nil {
				s.Errors = append(s.Errors, err.Error())
				return s
			}
			if strictCandidateBytes(raw, &response) != nil {
				s.Errors = append(s.Errors, "OSV_BATCH_SHAPE_INVALID")
				return s
			}
			if response.Results == nil || len(response.Results) != len(pending) {
				s.Errors = append(s.Errors, "OSV_RESULT_COUNT")
				return s
			}
			next := []int{}
			for pos, r := range response.Results {
				i := pending[pos]
				if r == nil || len(r.Next) > 2048 || len(r.Vulns) > securityMaxAdvisories {
					s.Errors = append(s.Errors, "OSV_RESULT_SHAPE")
					return s
				}
				for _, hit := range r.Vulns {
					if !securityIDPattern.MatchString(hit.ID) || !securityTimestamp(hit.Modified) || seenIDs[i][hit.ID] {
						s.Errors = append(s.Errors, "OSV_HIT_INVALID")
						return s
					}
					seenIDs[i][hit.ID] = true
					detail, ok := details[hit.ID]
					if !ok {
						if len(details) >= securityMaxAdvisories {
							s.Errors = append(s.Errors, "OSV_ADVISORY_LIMIT")
							return s
						}
						if err := p.request(ctx, http.MethodGet, "/vulns/"+hit.ID, nil, &detail); err != nil {
							s.Errors = append(s.Errors, err.Error())
							return s
						}
						if detail.ID != hit.ID || !validOSVAdvisory(detail) {
							s.Errors = append(s.Errors, "OSV_DETAIL_ID_OR_SHAPE")
							return s
						}
						details[hit.ID] = detail
						s.Advisories = append(s.Advisories, detail)
					}
					if detail.Modified != hit.Modified {
						s.Errors = append(s.Errors, "OSV_DETAIL_MODIFIED_CONFLICT")
						return s
					}
					s.Queries[i].Advisories = append(s.Queries[i].Advisories, hit.ID)
					encoded, encodeErr := supplyJSON(s)
					if encodeErr != nil || len(encoded) > securityMaxResponse-4096 {
						// Retain a replayable incomplete snapshot within its bound.
						s.Advisories = []osvAdvisory{}
						for j := range s.Queries {
							s.Queries[j].Complete = false
							s.Queries[j].Advisories = []string{}
						}
						s.Errors = append(s.Errors, "OSV_SNAPSHOT_SIZE_LIMIT")
						return s
					}
				}
				if r.Next != "" {
					if seenTokens[i][r.Next] {
						s.Errors = append(s.Errors, "OSV_PAGE_TOKEN_CYCLE")
						return s
					}
					seenTokens[i][r.Next] = true
					tokens[i] = r.Next
					next = append(next, i)
				} else {
					s.Queries[i].Complete = true
				}
			}
			pending = next
		}
	}
	for i := range s.Queries {
		sort.Strings(s.Queries[i].Advisories)
	}
	sort.Slice(s.Advisories, func(i, j int) bool { return s.Advisories[i].ID < s.Advisories[j].ID })
	return s
}

func securityTimestamp(s string) bool {
	t, err := time.Parse(time.RFC3339Nano, s)
	return err == nil && !t.IsZero()
}
func validOSVAdvisory(a osvAdvisory) bool {
	if len(a.ID) > 128 || !securityIDPattern.MatchString(a.ID) || !securityTimestamp(a.Modified) || len(a.Affected) == 0 || len(a.Affected) > 128 || len(a.Aliases) > 64 || !validSecuritySeverity(a.Severity, a.Database) {
		return false
	}
	if a.Withdrawn != "" && !securityTimestamp(a.Withdrawn) {
		return false
	}
	seen := map[string]bool{a.ID: true}
	for _, alias := range a.Aliases {
		if len(alias) > 128 || !securityIDPattern.MatchString(alias) || seen[alias] {
			return false
		}
		seen[alias] = true
	}
	for _, f := range a.Affected {
		if !securityModulePattern.MatchString(f.Package.Name) || strings.Contains(f.Package.Name, "..") || f.Package.Ecosystem != "Go" || len(f.Package.Name) > 512 || len(f.Ranges) > 128 || len(f.Versions) > 4096 || !validSecuritySeverity(f.Severity, f.Database) {
			return false
		}
		for _, v := range f.Versions {
			if !securityVersion(v) {
				return false
			}
		}
		for _, r := range f.Ranges {
			if len(r.Events) == 0 || len(r.Events) > 256 || !securityModulePattern.MatchString(r.Type) || len(r.Type) > 32 {
				return false
			}
			for _, e := range r.Events {
				for _, v := range []string{e.Introduced, e.Fixed, e.LastAffected, e.Limit} {
					if v != "" && v != "0" && !securityVersion(v) {
						return false
					}
				}
			}
		}
	}
	return true
}

func validSecuritySeverity(values []osvSeverity, database osvDatabase) bool {
	if len(values) > 16 || database.Severity != "" && securitySeverity(database.Severity) == "UNKNOWN" {
		return false
	}
	for _, v := range values {
		if (v.Type != "CVSS_V2" && v.Type != "CVSS_V3" && v.Type != "CVSS_V4") || len(v.Score) > 512 || !securityCVSSPattern.MatchString(v.Score) {
			return false
		}
	}
	return true
}
