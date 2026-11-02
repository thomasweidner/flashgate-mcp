package main

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
)

type securityException struct {
	Advisory string `json:"advisory"`
	Module   string `json:"module"`
	Version  string `json:"version"`
	Reason   string `json:"reason"`
	Owner    string `json:"owner"`
	ReviewBy string `json:"reviewBy"`
}
type securityTriage struct {
	Schema     string              `json:"schema"`
	Exceptions []securityException `json:"exceptions"`
}
type securityFinding struct {
	Module      securityModule     `json:"module"`
	Advisory    string             `json:"advisory"`
	Records     []string           `json:"records"`
	Aliases     []string           `json:"aliases"`
	Severity    string             `json:"severity"`
	Priority    string             `json:"priority"`
	Affected    string             `json:"affected"`
	FixVersions []string           `json:"fixVersions"`
	Disposition string             `json:"disposition"`
	Remediation string             `json:"remediation"`
	Exception   *securityException `json:"exception,omitempty"`
}
type securityAssessment struct {
	Schema         string            `json:"schema"`
	Status         string            `json:"status"`
	Informational  bool              `json:"informational"`
	Identity       securityIdentity  `json:"inventory"`
	AssessedAt     string            `json:"assessedAt"`
	CapturedAt     string            `json:"capturedAt"`
	SnapshotSHA256 string            `json:"snapshotSha256"`
	TriageSHA256   string            `json:"triageSha256,omitempty"`
	Freshness      string            `json:"freshness"`
	Scope          string            `json:"scope"`
	Modules        []securityModule  `json:"modules"`
	Findings       []securityFinding `json:"findings"`
	Quality        []string          `json:"qualityFindings"`
}

// A source inventory proves declared usage, never symbol reachability. Even
// BUILD findings require triage because compromised tooling can affect output.
func assessSecurity(identity securityIdentity, modules []securityModule, snapshot securitySnapshot, triage securityTriage, now time.Time) securityAssessment {
	r := securityAssessment{Schema: "flashgate-security-assessment/v1", Status: "COMPLETE", Informational: true, Identity: identity, AssessedAt: now.UTC().Format(time.RFC3339), Freshness: "UNKNOWN", Scope: "SPDX source Go modules only; toolchain, standard library, OS, CI tools and branding are not CVE-assessed; runtime reachability is unproven", Modules: modules, Findings: []securityFinding{}, Quality: []string{}}
	quality := func(code string) { r.Quality = append(r.Quality, code); r.Status = "INCOMPLETE" }
	if snapshot.Schema != securitySnapshotSchema || snapshot.Provider != "OSV" || snapshot.Identity != identity {
		quality("SNAPSHOT_IDENTITY_MISMATCH")
		return r
	}
	captured, err := time.Parse(time.RFC3339, snapshot.CapturedAt)
	if err == nil {
		r.CapturedAt = captured.UTC().Format(time.RFC3339)
	}
	if err != nil || captured.After(now) {
		quality("SNAPSHOT_TIME_INVALID")
	} else if now.Sub(captured) > 24*time.Hour {
		r.Freshness = "STALE"
		quality("SNAPSHOT_STALE")
	} else {
		r.Freshness = "FRESH"
	}
	if len(snapshot.Errors) > 0 {
		quality("PROVIDER_INCOMPLETE")
	}
	if snapshot.Queries == nil || snapshot.Advisories == nil || snapshot.Errors == nil || len(snapshot.Queries) != len(modules) || len(modules) == 0 || len(modules) > securityMaxModules || len(snapshot.Advisories) > securityMaxAdvisories {
		quality("SNAPSHOT_SHAPE_INVALID")
		return r
	}
	advisories := map[string]osvAdvisory{}
	for _, a := range snapshot.Advisories {
		if _, exists := advisories[a.ID]; exists || !validOSVAdvisory(a) {
			quality("ADVISORY_INVALID")
			return r
		}
		modified, _ := time.Parse(time.RFC3339Nano, a.Modified)
		if modified.After(captured) {
			quality("ADVISORY_TIME_CONFLICT")
		}
		advisories[a.ID] = a
	}
	used := map[string]bool{}
	for i, m := range modules {
		if m.Dependency == "UNKNOWN" || m.Exposure == "UNKNOWN" {
			quality("INVENTORY_CLASSIFICATION_UNKNOWN")
		}
		q := snapshot.Queries[i]
		if q.Advisories == nil {
			quality("QUERY_ADVISORY_DATA_MISSING")
		}
		if q.Name != m.Name || q.Version != m.Version {
			quality("QUERY_IDENTITY_MISMATCH")
			continue
		}
		if !q.Complete {
			quality("QUERY_INCOMPLETE")
		}
		if len(q.Advisories) > securityMaxAdvisories {
			quality("QUERY_ADVISORY_LIMIT")
			continue
		}
		seen := map[string]bool{}
		var current []securityFinding
		for _, id := range q.Advisories {
			a, ok := advisories[id]
			if !ok || seen[id] {
				quality("QUERY_DETAIL_MISSING_OR_DUPLICATE")
				continue
			}
			seen[id] = true
			used[id] = true
			affected, fixes, severity := securityAffected(a, m)
			if affected != "AFFECTED" {
				quality("ADVISORY_MODULE_VERSION_UNCONFIRMED")
			}
			f := securityFinding{Module: m, Advisory: id, Records: []string{id}, Aliases: append([]string{}, a.Aliases...), Severity: severity, Priority: securityPriority(severity), Affected: affected, FixVersions: fixes, Disposition: "TRIAGE_REQUIRED", Remediation: "REVIEW_UPGRADE_OR_DOCUMENT_EXCEPTION"}
			if len(fixes) == 0 {
				f.Remediation = "NO_CONFIRMED_FIX_REVIEW_MITIGATION"
			}
			if a.Withdrawn != "" {
				f.Disposition = "WITHDRAWN_REVIEW_REQUIRED"
			}
			current = append(current, f)
		}
		// Validate original records before merging any aliases; otherwise a
		// transitive merge can hide a conflicting record's fix provenance.
		sort.Slice(current, func(i, j int) bool { return current[i].Advisory < current[j].Advisory })
		current, codes := securityConsolidateAliases(current, advisories)
		for _, code := range codes {
			quality(code)
		}
		r.Findings = append(r.Findings, current...)
	}
	if len(used) != len(advisories) {
		quality("UNQUERIED_ADVISORY_DETAIL")
	}
	if triage.Schema != "flashgate-security-triage/v1" || triage.Exceptions == nil || len(triage.Exceptions) > 128 {
		quality("TRIAGE_INVALID")
	} else {
		keys := map[string]bool{}
		for _, e := range triage.Exceptions {
			key := e.Advisory + "\x00" + e.Module + "\x00" + e.Version
			expires, err := time.Parse(time.RFC3339, e.ReviewBy)
			if !securityIDPattern.MatchString(e.Advisory) || !securityText(e.Module, 512) || !securityVersion(e.Version) || !securityText(e.Reason, 512) || !securityText(e.Owner, 128) || err != nil || expires.UTC().Format(time.RFC3339) != e.ReviewBy || !expires.After(now) || keys[key] {
				quality("EXCEPTION_INVALID_DUPLICATE_OR_EXPIRED")
				continue
			}
			keys[key] = true
			matched := false
			for i := range r.Findings {
				f := &r.Findings[i]
				if f.Module.Name == e.Module && f.Module.Version == e.Version && (f.Advisory == e.Advisory || containsString(f.Records, e.Advisory)) {
					if f.Exception != nil {
						quality("EXCEPTION_CONFLICT")
						continue
					}
					copy := e
					f.Exception = &copy
					f.Disposition = "DOCUMENTED_EXCEPTION_REVIEW_REQUIRED"
					matched = true
				}
			}
			if !matched {
				quality("EXCEPTION_IDENTITY_UNMATCHED")
			}
		}
	}
	sort.Slice(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		if a.Module.Name != b.Module.Name {
			return a.Module.Name < b.Module.Name
		}
		return a.Advisory < b.Advisory
	})
	r.Quality = securityUnion(nil, r.Quality)
	return r
}

func securityText(s string, maximum int) bool {
	if strings.TrimSpace(s) == "" || len(s) > maximum || strings.ContainsAny(s, "*?\\\r\n\x00") {
		return false
	}
	for _, c := range s {
		if c < 32 || c > 126 {
			return false
		}
	}
	r := leakReport{}
	scanBytes("triage", []byte(s), nil, &r)
	return len(r.Findings) == 0
}
func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func securityUnion(a, b []string) []string {
	m := map[string]bool{}
	for _, s := range append(append([]string{}, a...), b...) {
		m[s] = true
	}
	r := []string{}
	for s := range m {
		r = append(r, s)
	}
	sort.Strings(r)
	return r
}
func securityAliasOverlap(a, b securityFinding) bool {
	return len(securityUnion(append(append([]string{}, a.Aliases...), a.Advisory), append(append([]string{}, b.Aliases...), b.Advisory))) < len(securityUnion(a.Aliases, []string{a.Advisory}))+len(securityUnion(b.Aliases, []string{b.Advisory}))
}

func securityConsolidateAliases(current []securityFinding, advisories map[string]osvAdvisory) ([]securityFinding, []string) {
	reciprocal := func(a, b securityFinding) bool {
		return containsString(a.Aliases, b.Advisory) && containsString(b.Aliases, a.Advisory) && a.Affected == b.Affected && a.Disposition == b.Disposition
	}
	quality := []string{}
	groups := [][]int{}
	component := make([]int, len(current))
	seen := make([]bool, len(current))
	for start := range current {
		if seen[start] {
			continue
		}
		// Form groups using only original reciprocal edges, never aliases
		// accumulated by a partial merge. Check the entire group first.
		group := []int{start}
		seen[start] = true
		for at := 0; at < len(group); at++ {
			for j := range current {
				if !seen[j] && reciprocal(current[group[at]], current[j]) {
					seen[j] = true
					group = append(group, j)
				}
			}
		}
		sort.Ints(group)
		for _, i := range group {
			component[i] = len(groups)
		}
		groups = append(groups, group)
	}
	for x, a := range current {
		for y := x + 1; y < len(current); y++ {
			if component[x] != component[y] && securityAliasOverlap(a, current[y]) {
				quality = append(quality, "ALIAS_AMBIGUOUS_OR_CONFLICTING")
			}
		}
	}
	result := []securityFinding{}
	for _, group := range groups {
		conflict, unknown := false, false
		fixes := []string{}
		if len(group) > 1 {
			for _, i := range group {
				fixes = securityUnion(fixes, current[i].FixVersions)
				unknown = unknown || current[i].Affected != "AFFECTED"
			}
			for _, fix := range fixes {
				for _, i := range group {
					m := current[i].Module
					m.Version = "v" + strings.TrimPrefix(fix, "v")
					status, _, _, conclusive := securityVersionEvidence(advisories[current[i].Advisory], m)
					conflict = conflict || status == "AFFECTED"
					unknown = unknown || !conclusive
				}
			}
		}
		if conflict || unknown {
			if conflict {
				quality = append(quality, "ALIAS_FIX_CONFLICT")
			}
			if unknown {
				quality = append(quality, "ALIAS_FIX_EVIDENCE_UNKNOWN")
			}
			for _, i := range group {
				result = append(result, current[i])
			}
			continue
		}
		merged := current[group[0]]
		for _, i := range group[1:] {
			b := current[i]
			merged.Records = securityUnion(merged.Records, b.Records)
			merged.Aliases = securityUnion(securityUnion(merged.Aliases, b.Aliases), b.Records)
			merged.FixVersions = securityUnion(merged.FixVersions, b.FixVersions)
			if securityRank(b.Severity) > securityRank(merged.Severity) {
				merged.Severity, merged.Priority = b.Severity, b.Priority
			}
		}
		if len(merged.FixVersions) > 0 {
			merged.Remediation = "REVIEW_UPGRADE_OR_DOCUMENT_EXCEPTION"
		}
		result = append(result, merged)
	}
	return result, securityUnion(nil, quality)
}

func securityRank(s string) int {
	switch s {
	case "CRITICAL":
		return 5
	case "HIGH":
		return 4
	case "MODERATE", "MEDIUM":
		return 3
	case "LOW":
		return 2
	}
	return 0
}
func securityPriority(s string) string {
	switch securityRank(s) {
	case 5:
		return "P1"
	case 4:
		return "P2"
	case 3:
		return "P3"
	case 2:
		return "P4"
	}
	return "P2_REVIEW_UNKNOWN"
}
func securitySeverity(label string) string {
	switch label {
	case "CRITICAL", "HIGH", "LOW":
		return label
	case "MODERATE", "MEDIUM":
		return "MEDIUM"
	}
	return "UNKNOWN"
}

// Keep CVSS vectors as provider input, not an invented numeric score or label.
// Only explicit OSV database severity labels are classified. Unknown stays
// unknown when a provider supplies only a vector, an unsupported scale or no data.
func securityAffected(a osvAdvisory, m securityModule) (string, []string, string) {
	status, fixes, severity, _ := securityVersionEvidence(a, m)
	return status, fixes, severity
}

// Keep the installed-version UNKNOWN contract, while distinguishing supported
// evidence that excludes a proposed fix from evidence we cannot evaluate.
func securityVersionEvidence(a osvAdvisory, m securityModule) (string, []string, string, bool) {
	status, fixes, severity := "UNKNOWN", []string{}, securitySeverity(a.Database.Severity)
	invalid, conclusive, missing := false, false, false
	for _, f := range a.Affected {
		if f.Package.Ecosystem != "Go" || f.Package.Name != m.Name {
			continue
		}
		entryMatches := false
		for _, version := range f.Versions {
			c, err := securityCompare(m.Version, version)
			if err != nil {
				invalid = true
			} else if c == 0 {
				entryMatches = true
			}
		}
		conclusive = conclusive || len(f.Versions) > 0
		missing = missing || len(f.Versions) == 0 && len(f.Ranges) == 0
		for _, r := range f.Ranges {
			matched, fixed, err := securityRange(m.Version, r)
			if err != nil {
				invalid = true
			} else {
				conclusive = true
			}
			if err == nil && matched {
				entryMatches = true
				fixes = append(fixes, fixed...)
			}
		}
		if entryMatches {
			status = "AFFECTED"
			if label := securitySeverity(f.Database.Severity); securityRank(label) > securityRank(severity) {
				severity = label
			}
		}
	}
	if invalid {
		return "UNKNOWN", []string{}, severity, false
	}
	return status, securityUnion(nil, fixes), severity, conclusive && !missing
}

var securitySemver = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

func securityVersion(s string) bool { _, err := securityCompare(s, s); return err == nil }
func securityCompare(a, b string) (int, error) {
	parts := [][]string{securitySemver.FindStringSubmatch(a), securitySemver.FindStringSubmatch(b)}
	for _, p := range parts {
		if p == nil {
			return 0, errors.New("invalid semver")
		}
		for _, id := range strings.Split(p[4], ".") {
			if len(id) > 1 && id[0] == '0' && strings.Trim(id, "0123456789") == "" {
				return 0, errors.New("invalid prerelease")
			}
		}
	}
	cmp := func(a, b string) int {
		if len(a) < len(b) {
			return -1
		}
		if len(a) > len(b) {
			return 1
		}
		return strings.Compare(a, b)
	}
	for i := 1; i <= 3; i++ {
		if c := cmp(parts[0][i], parts[1][i]); c != 0 {
			return c, nil
		}
	}
	x, y := parts[0][4], parts[1][4]
	if x == y {
		return 0, nil
	}
	if x == "" {
		return 1, nil
	}
	if y == "" {
		return -1, nil
	}
	xs, ys := strings.Split(x, "."), strings.Split(y, ".")
	for i := 0; i < min(len(xs), len(ys)); i++ {
		an, bn := strings.Trim(xs[i], "0123456789") == "", strings.Trim(ys[i], "0123456789") == ""
		c := strings.Compare(xs[i], ys[i])
		if an && bn {
			c = cmp(xs[i], ys[i])
		} else if an {
			c = -1
		} else if bn {
			c = 1
		}
		if c != 0 {
			return c, nil
		}
	}
	return strconvCompareLengths(len(xs), len(ys)), nil
}
func strconvCompareLengths(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func securityRange(version string, r osvRange) (bool, []string, error) {
	bad := errors.New("unsupported or invalid affected range")
	if r.Type != "SEMVER" || len(r.Events) == 0 || !securityVersion(version) {
		return false, nil, bad
	}
	active, matched := false, false
	affected := false
	fixes := []string{}
	previous := ""
	for _, e := range r.Events {
		count := 0
		value := ""
		kind := ""
		for k, v := range map[string]string{"introduced": e.Introduced, "fixed": e.Fixed, "last_affected": e.LastAffected, "limit": e.Limit} {
			if v != "" {
				count++
				value = v
				kind = k
			}
		}
		if count != 1 {
			return false, nil, bad
		}
		if value != "0" || kind != "introduced" {
			if !securityVersion(value) {
				return false, nil, bad
			}
			if previous != "" {
				c, _ := securityCompare(value, previous)
				if c <= 0 {
					return false, nil, bad
				}
			}
			previous = value
		} else if previous != "" {
			return false, nil, bad
		}
		if kind == "introduced" {
			if active {
				return false, nil, bad
			}
			active = true
			c, _ := securityCompare(version, value)
			matched = value == "0" || c >= 0
		} else {
			if !active {
				return false, nil, bad
			}
			c, _ := securityCompare(version, value)
			if matched && (c < 0 || (kind == "last_affected" && c == 0)) {
				if kind == "fixed" {
					fixes = append(fixes, value)
				}
				affected = true
			}
			active = false
			matched = false
		}
	}
	return affected || active && matched, fixes, nil
}
