package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var securityTestNow = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func securityTestAdvisory(id, name, version, severity string) osvAdvisory {
	return osvAdvisory{ID: id, Modified: "2026-11-01T11:00:00Z", Aliases: []string{}, Affected: []osvAffected{{Package: osvPackage{name, "Go"}, Versions: []string{version}}}, Database: osvDatabase{severity}}
}
func securityTestSnapshot(identity securityIdentity, modules []securityModule) securitySnapshot {
	s := securitySnapshot{Schema: securitySnapshotSchema, Provider: "OSV", CapturedAt: securityTestNow.Format(time.RFC3339), Identity: identity, Queries: []securityQuery{}, Advisories: []osvAdvisory{}, Errors: []string{}}
	for _, m := range modules {
		s.Queries = append(s.Queries, securityQuery{m.Name, m.Version, true, []string{}})
	}
	return s
}
func securityTestAssess(modules []securityModule, s securitySnapshot) securityAssessment {
	return assessSecurity(s.Identity, modules, s, securityTriage{"flashgate-security-triage/v1", []securityException{}}, securityTestNow)
}
func TestSecurityInventoryAnchors(t *testing.T) {
	dir, mh, ih, manifest := supplyFixture(t)
	id := securityIdentity{Version: manifest.Version, Commit: manifest.SourceCommitSHA, Manifest: mh, Index: ih}
	bound, modules, err := readSecurityInventory(dir, id)
	if err != nil || bound.GoMod == "" || bound.SBOM == "" {
		t.Fatal(bound, modules, err)
	}
	want := []securityModule{
		{resourceModule, "v0.10.2", "INDIRECT", "BUILD"},
		{versionInfoModule, "v1.7.0", "DIRECT", "BUILD"},
		{tomlModule, tomlVersion, "DIRECT", "RUNTIME"},
		{schemaModule, "v6.0.3", "DIRECT", "TEST"},
		{textModule, "v0.41.0", "INDIRECT", "TEST"},
	}
	if !reflect.DeepEqual(modules, want) {
		t.Fatalf("module classification: got %+v, want %+v", modules, want)
	}
	for _, change := range []func(*securityIdentity){func(i *securityIdentity) { i.Version = "9.9.9" }, func(i *securityIdentity) { i.Commit = strings.Repeat("0", 40) }, func(i *securityIdentity) { i.Manifest = ih }, func(i *securityIdentity) { i.Index = mh }, func(i *securityIdentity) { i.Index = "" }} {
		bad := id
		change(&bad)
		if _, _, err := readSecurityInventory(dir, bad); err == nil {
			t.Fatal("untrusted identity accepted")
		}
	}
	for _, name := range []string{sbomName, "source/go.mod", "candidate/candidate-manifest.json"} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		before, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, append(before, ' '), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readSecurityInventory(dir, id); err == nil {
			t.Fatal("tampered inventory accepted", name)
		}
		if err := os.WriteFile(p, before, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(dir, sbomName)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readSecurityInventory(dir, id); err == nil {
		t.Fatal("missing inventory accepted")
	}
}

func TestSecurityClassification(t *testing.T) {
	doc := `{"spdxVersion":"SPDX-2.3","packages":[{"SPDXID":"A","name":"example.org/a","versionInfo":"v1.0.0"},{"SPDXID":"B","name":"example.org/b","versionInfo":"v2.0.0"}],"relationships":[{"spdxElementId":"A","relationshipType":"DEPENDS_ON","relatedSpdxElement":"B"},{"spdxElementId":"A","relationshipType":"BUILD_DEPENDENCY_OF","relatedSpdxElement":"SPDXRef-FlashGate"},{"spdxElementId":"B","relationshipType":"RUNTIME_DEPENDENCY_OF","relatedSpdxElement":"SPDXRef-FlashGate"}]}`
	mod := []byte("require (\nexample.org/a v1.0.0\nexample.org/b v2.0.0 // indirect\n)\n")
	modules, err := classifySecurityModules([]byte(doc), mod)
	if err != nil || modules[0].Exposure != "BUILD" || modules[1].Exposure != "RUNTIME" || modules[1].Dependency != "INDIRECT" {
		t.Fatal(modules, err)
	}
	unknown := strings.ReplaceAll(doc, "BUILD_DEPENDENCY_OF", "OTHER")
	modules, err = classifySecurityModules([]byte(unknown), mod)
	if err != nil || modules[0].Exposure != "UNKNOWN" {
		t.Fatal(modules, err)
	}
	for _, bad := range []string{strings.ReplaceAll(doc, "v1.0.0", "v1.0.1"), strings.ReplaceAll(doc, "DEPENDS_ON", "OTHER"), strings.ReplaceAll(doc, "SPDX-2.3", "SPDX-3.0")} {
		if _, err := classifySecurityModules([]byte(bad), mod); err == nil {
			t.Fatal("classification mismatch accepted")
		}
	}
	if _, err := classifySecurityModules([]byte(doc), bytes.ReplaceAll(mod, []byte(" // indirect"), nil)); err == nil {
		t.Fatal("direct relation contradiction accepted")
	}
}

func TestSecurityAssessmentSeverityNonblocking(t *testing.T) {
	modules := []securityModule{{"example.org/a", "v1.2.3", "DIRECT", "BUILD"}}
	for _, label := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", ""} {
		s := securityTestSnapshot(securityIdentity{}, modules)
		s.Advisories = []osvAdvisory{securityTestAdvisory("GO-2026-1000", modules[0].Name, modules[0].Version, label)}
		s.Queries[0].Advisories = []string{s.Advisories[0].ID}
		r := securityTestAssess(modules, s)
		if r.Status != "COMPLETE" || !r.Informational || len(r.Findings) != 1 || r.Findings[0].Disposition != "TRIAGE_REQUIRED" || r.Findings[0].Remediation != "NO_CONFIRMED_FIX_REVIEW_MITIGATION" {
			t.Fatal(r)
		}
		if label == "" && r.Findings[0].Severity != "UNKNOWN" {
			t.Fatal("invented severity")
		}
	}
	s := securityTestSnapshot(securityIdentity{}, modules)
	if r := securityTestAssess(modules, s); r.Status != "COMPLETE" || len(r.Findings) != 0 {
		t.Fatal(r)
	}
}

func TestSecurityAffectedSeverityByVersion(t *testing.T) {
	const module = "example.org/mod"
	explicit := func(version, severity string) osvAffected {
		return osvAffected{Package: osvPackage{module, "Go"}, Versions: []string{version}, Database: osvDatabase{severity}}
	}
	interval := func(introduced, fixed, severity string) osvAffected {
		return osvAffected{Package: osvPackage{module, "Go"}, Ranges: []osvRange{{Type: "SEMVER", Events: []osvEvent{{Introduced: introduced}, {Fixed: fixed}}}}, Database: osvDatabase{severity}}
	}
	low := explicit("1.0.0", "LOW")
	critical := explicit("2.0.0", "CRITICAL")
	lowRange := interval("1.0.0", "1.1.0", "LOW")
	criticalRange := interval("2.0.0", "2.1.0", "CRITICAL")
	unsupported := osvAffected{Package: osvPackage{module, "Go"}, Ranges: []osvRange{{Type: "ECOSYSTEM", Events: []osvEvent{{Introduced: "2.0.0"}, {Fixed: "2.1.0"}}}}, Database: osvDatabase{"CRITICAL"}}
	for _, tc := range []struct {
		name, version, global, status, severity string
		affected                                []osvAffected
		fixes                                   []string
	}{
		{"disjoint explicit versions", "v1.0.0", "", "AFFECTED", "LOW", []osvAffected{low, critical}, nil},
		{"disjoint semver intervals", "v1.0.0", "", "AFFECTED", "LOW", []osvAffected{lowRange, criticalRange}, []string{"1.1.0"}},
		{"matching explicit and unrelated interval", "v1.0.0", "", "AFFECTED", "LOW", []osvAffected{low, criticalRange}, nil},
		{"matching interval and unrelated explicit", "v1.0.0", "", "AFFECTED", "LOW", []osvAffected{lowRange, critical}, []string{"1.1.0"}},
		{"matching severity unavailable", "v1.0.0", "", "AFFECTED", "UNKNOWN", []osvAffected{explicit("1.0.0", ""), critical}, nil},
		{"matching interval severity unavailable", "v1.0.0", "", "AFFECTED", "UNKNOWN", []osvAffected{interval("1.0.0", "1.1.0", ""), criticalRange}, []string{"1.1.0"}},
		{"no explicit version match", "v1.0.0", "", "UNKNOWN", "UNKNOWN", []osvAffected{explicit("1.2.0", "LOW"), critical}, nil},
		{"no interval match", "v0.9.0", "", "UNKNOWN", "UNKNOWN", []osvAffected{lowRange, criticalRange}, nil},
		{"fixed boundary is not affected", "v1.1.0", "", "UNKNOWN", "UNKNOWN", []osvAffected{lowRange, criticalRange}, nil},
		{"advisory global severity retained", "v1.0.0", "HIGH", "AFFECTED", "HIGH", []osvAffected{low, critical}, nil},
		{"advisory global severity without local match", "v1.0.0", "HIGH", "UNKNOWN", "HIGH", []osvAffected{critical}, nil},
		{"higher matching severity retained", "v1.0.0", "LOW", "AFFECTED", "MEDIUM", []osvAffected{interval("1.0.0", "1.1.0", "MODERATE"), critical}, []string{"1.1.0"}},
		{"multiple relevant entries use their highest severity", "v1.0.0", "", "AFFECTED", "HIGH", []osvAffected{lowRange, explicit("1.0.0", "HIGH"), critical}, []string{"1.1.0"}},
		{"unsupported interval remains incomplete", "v1.0.0", "", "UNKNOWN", "LOW", []osvAffected{lowRange, unsupported}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, reverse := range []bool{false, true} {
				a := securityTestAdvisory("GO-2026-1000", module, tc.version, tc.global)
				a.Affected = append([]osvAffected{}, tc.affected...)
				if reverse {
					for i, j := 0, len(a.Affected)-1; i < j; i, j = i+1, j-1 {
						a.Affected[i], a.Affected[j] = a.Affected[j], a.Affected[i]
					}
				}
				m := securityModule{module, tc.version, "DIRECT", "BUILD"}
				status, fixes, severity := securityAffected(a, m)
				if status != tc.status || severity != tc.severity || !reflect.DeepEqual(fixes, securityUnion(nil, tc.fixes)) {
					t.Fatalf("reverse=%v: status=%s severity=%s fixes=%v; want %s/%s/%v", reverse, status, severity, fixes, tc.status, tc.severity, tc.fixes)
				}
				s := securityTestSnapshot(securityIdentity{}, []securityModule{m})
				s.Advisories = []osvAdvisory{a}
				s.Queries[0].Advisories = []string{a.ID}
				r := securityTestAssess([]securityModule{m}, s)
				wantStatus := "COMPLETE"
				if tc.status == "UNKNOWN" {
					wantStatus = "INCOMPLETE"
				}
				if r.Status != wantStatus || !r.Informational || len(r.Findings) != 1 || r.Findings[0].Severity != tc.severity || r.Findings[0].Priority != securityPriority(tc.severity) || r.Findings[0].Affected != tc.status || !reflect.DeepEqual(r.Findings[0].FixVersions, fixes) {
					t.Fatalf("reverse=%v: assessment lost version-specific severity, priority, completeness or fix provenance: %+v", reverse, r)
				}
			}
		})
	}
}

func TestSecurityAssessmentMissingStaleSwapped(t *testing.T) {
	modules := []securityModule{{"example.org/a", "v1.2.3", "DIRECT", "BUILD"}, {"example.org/b", "v2.0.0", "INDIRECT", "RUNTIME"}}
	for name, mutate := range map[string]func(*securitySnapshot){
		"stale":               func(s *securitySnapshot) { s.CapturedAt = securityTestNow.Add(-25 * time.Hour).Format(time.RFC3339) },
		"future":              func(s *securitySnapshot) { s.CapturedAt = securityTestNow.Add(time.Hour).Format(time.RFC3339) },
		"missing":             func(s *securitySnapshot) { s.Queries = s.Queries[:1] },
		"swapped":             func(s *securitySnapshot) { s.Queries[0], s.Queries[1] = s.Queries[1], s.Queries[0] },
		"identity":            func(s *securitySnapshot) { s.Identity.Index = "different" },
		"partial":             func(s *securitySnapshot) { s.Queries[0].Complete = false },
		"missingAdvisoryList": func(s *securitySnapshot) { s.Queries[0].Advisories = nil },
		"provider":            func(s *securitySnapshot) { s.Errors = []string{"private failure"} },
		"missingDetail":       func(s *securitySnapshot) { s.Queries[0].Advisories = []string{"GO-2026-1000"} },
		"wrongVersion": func(s *securitySnapshot) {
			s.Advisories = []osvAdvisory{securityTestAdvisory("GO-2026-1000", modules[0].Name, "v9.0.0", "")}
			s.Queries[0].Advisories = []string{"GO-2026-1000"}
		},
		"wrongModule": func(s *securitySnapshot) {
			s.Advisories = []osvAdvisory{securityTestAdvisory("GO-2026-1000", "other/module", "v1.2.3", "")}
			s.Queries[0].Advisories = []string{"GO-2026-1000"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := securityTestSnapshot(securityIdentity{}, modules)
			mutate(&s)
			r := assessSecurity(securityIdentity{}, modules, s, securityTriage{"flashgate-security-triage/v1", []securityException{}}, securityTestNow)
			if r.Status != "INCOMPLETE" || len(r.Quality) == 0 {
				t.Fatal(r)
			}
			data, _ := json.Marshal(r)
			if bytes.Contains(data, []byte("private failure")) {
				t.Fatal("provider error leaked")
			}
		})
	}
}

func TestSecurityAliasesAndDeterminism(t *testing.T) {
	modules := []securityModule{{"example.org/a", "v1.2.3", "DIRECT", "BUILD"}}
	s := securityTestSnapshot(securityIdentity{}, modules)
	a := securityTestAdvisory("GO-2026-1000", modules[0].Name, "v1.2.3", "HIGH")
	b := securityTestAdvisory("GHSA-aaaa-bbbb-cccc", modules[0].Name, "v1.2.3", "CRITICAL")
	a.Aliases = []string{b.ID, "CVE-2026-12345"}
	b.Aliases = []string{a.ID, "CVE-2026-12345"}
	s.Advisories = []osvAdvisory{a, b}
	s.Queries[0].Advisories = []string{b.ID, a.ID}
	r := securityTestAssess(modules, s)
	if r.Status != "COMPLETE" || len(r.Findings) != 1 || len(r.Findings[0].Records) != 2 || r.Findings[0].Severity != "CRITICAL" {
		t.Fatal(r)
	}
	first, _ := supplyJSON(r)
	s.Advisories[0], s.Advisories[1] = s.Advisories[1], s.Advisories[0]
	s.Queries[0].Advisories = []string{a.ID, b.ID}
	second, _ := supplyJSON(securityTestAssess(modules, s))
	if !bytes.Equal(first, second) {
		t.Fatal("nondeterministic report")
	}
	s.Advisories[0].Aliases = []string{"CVE-2026-12345"}
	s.Advisories[1].Aliases = []string{"CVE-2026-12345"}
	r = securityTestAssess(modules, s)
	if len(r.Findings) != 2 || r.Status != "INCOMPLETE" {
		t.Fatal("unjustified alias merge", r)
	}
}

func TestSecurityAliasFixConsistency(t *testing.T) {
	const module = "example.org/a"
	modules := []securityModule{{module, "v1.2.3", "DIRECT", "BUILD"}}
	makeAdvisory := func(id, fix string) osvAdvisory {
		a := securityTestAdvisory(id, module, modules[0].Version, "HIGH")
		if fix != "" {
			a.Affected[0].Versions = nil
			a.Affected[0].Ranges = []osvRange{{Type: "SEMVER", Events: []osvEvent{{Introduced: "0"}, {Fixed: fix}}}}
		}
		return a
	}
	pair := func(first, second string) []osvAdvisory {
		a, b := makeAdvisory("GO-2026-1000", first), makeAdvisory("GHSA-aaaa-bbbb-cccc", second)
		a.Aliases, b.Aliases = []string{b.ID}, []string{a.ID}
		return []osvAdvisory{a, b}
	}
	unknown := pair("", "")
	for i := range unknown {
		unknown[i].Affected[0].Ranges = []osvRange{{Type: "GIT", Events: []osvEvent{{Introduced: "0"}}}}
	}
	malformed := pair("", "")
	for i := range malformed {
		malformed[i].Affected[0].Ranges = []osvRange{{Type: "SEMVER", Events: []osvEvent{{Introduced: "1.4.0"}, {Fixed: "1.3.0"}}}}
	}
	chain := pair("1.3.0", "")
	c := makeAdvisory("CVE-2026-12345", "1.4.0")
	chain[1].Aliases = append(chain[1].Aliases, c.ID)
	c.Aliases = []string{chain[1].ID}
	chain = append(chain, c)
	consistentChain := pair("1.3.0", "1.3.0")
	c = makeAdvisory("CVE-2026-12345", "1.3.0")
	consistentChain[1].Aliases = append(consistentChain[1].Aliases, c.ID)
	c.Aliases = []string{consistentChain[1].ID}
	consistentChain = append(consistentChain, c)
	explicitConflict := pair("1.3.0", "")
	explicitConflict[1].Affected[0].Versions = append(explicitConflict[1].Affected[0].Versions, "v1.3.0")
	equivalentConflict := pair("1.3.0", "")
	equivalentConflict[1].Affected[0].Versions = append(equivalentConflict[1].Affected[0].Versions, "1.3.0+build")
	missingEvidence := pair("1.3.0", "")
	missingEvidence[1].Affected = append(missingEvidence[1].Affected, osvAffected{Package: osvPackage{module, "Go"}})
	for _, tc := range []struct {
		name       string
		advisories []osvAdvisory
		status     string
		count      int
		quality    string
	}{
		{"conflicting interval fixes", pair("1.3.0", "1.4.0"), "INCOMPLETE", 2, "ALIAS_FIX_CONFLICT"},
		{"identical interval fixes", pair("1.3.0", "1.3.0"), "COMPLETE", 1, ""},
		{"missing fix is not a contradiction", pair("1.3.0", ""), "COMPLETE", 1, ""},
		{"equivalent semver fix spellings", pair("v1.3.0", "1.3.0+build"), "COMPLETE", 1, ""},
		{"explicit affected fix", explicitConflict, "INCOMPLETE", 2, "ALIAS_FIX_CONFLICT"},
		{"semver equivalent affected fix", equivalentConflict, "INCOMPLETE", 2, "ALIAS_FIX_CONFLICT"},
		{"missing version evidence", missingEvidence, "INCOMPLETE", 2, "ALIAS_FIX_EVIDENCE_UNKNOWN"},
		{"unsupported range", unknown, "INCOMPLETE", 2, "ALIAS_FIX_EVIDENCE_UNKNOWN"},
		{"invalid range", malformed, "INCOMPLETE", 2, "ALIAS_FIX_EVIDENCE_UNKNOWN"},
		{"transitive fix conflict", chain, "INCOMPLETE", 3, "ALIAS_FIX_CONFLICT"},
		{"consistent reciprocal chain", consistentChain, "COMPLETE", 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var first []byte
			for _, reverseAdvisories := range []bool{false, true} {
				for _, reverseQueries := range []bool{false, true} {
					s := securityTestSnapshot(securityIdentity{}, modules)
					s.Advisories = append([]osvAdvisory{}, tc.advisories...)
					for _, a := range tc.advisories {
						s.Queries[0].Advisories = append(s.Queries[0].Advisories, a.ID)
					}
					if reverseAdvisories {
						for i, j := 0, len(s.Advisories)-1; i < j; i, j = i+1, j-1 {
							s.Advisories[i], s.Advisories[j] = s.Advisories[j], s.Advisories[i]
						}
					}
					if reverseQueries {
						for i, j := 0, len(s.Queries[0].Advisories)-1; i < j; i, j = i+1, j-1 {
							s.Queries[0].Advisories[i], s.Queries[0].Advisories[j] = s.Queries[0].Advisories[j], s.Queries[0].Advisories[i]
						}
					}
					before, _ := supplyJSON(s)
					r := securityTestAssess(modules, s)
					if r.Status != tc.status || !r.Informational || len(r.Findings) != tc.count || tc.quality != "" && !containsString(r.Quality, tc.quality) || tc.quality == "" && len(r.Quality) != 0 {
						t.Fatalf("status=%s quality=%v findings=%+v", r.Status, r.Quality, r.Findings)
					}
					if tc.count > 1 {
						for _, f := range r.Findings {
							for _, a := range tc.advisories {
								if a.ID != f.Advisory {
									continue
								}
								status, fixes, severity := securityAffected(a, modules[0])
								if !reflect.DeepEqual(f.Records, []string{a.ID}) || !reflect.DeepEqual(f.Aliases, a.Aliases) || !reflect.DeepEqual(f.FixVersions, fixes) || f.Affected != status || f.Severity != severity || f.Priority != securityPriority(severity) {
									t.Fatalf("original record provenance changed: %+v", f)
								}
							}
						}
						e := securityException{tc.advisories[0].ID, module, modules[0].Version, "Controlled build inputs; review upgrade.", "maintainer", "2026-12-01T00:00:00Z"}
						excepted := assessSecurity(s.Identity, modules, s, securityTriage{"flashgate-security-triage/v1", []securityException{e}}, securityTestNow)
						matched := 0
						for _, f := range excepted.Findings {
							if f.Exception != nil {
								matched++
								if f.Advisory != e.Advisory || f.Disposition != "DOCUMENTED_EXCEPTION_REVIEW_REQUIRED" {
									t.Fatal("exception crossed original record identity", f)
								}
							}
						}
						if matched != 1 || excepted.Status != "INCOMPLETE" || !containsString(excepted.Quality, tc.quality) {
							t.Fatal("exception suppressed conflicting evidence", excepted)
						}
					} else {
						ids, fixes := []string{}, []string{}
						for _, a := range tc.advisories {
							ids = append(ids, a.ID)
							_, af, _ := securityAffected(a, modules[0])
							fixes = securityUnion(fixes, af)
						}
						if !reflect.DeepEqual(r.Findings[0].Records, securityUnion(nil, ids)) || !reflect.DeepEqual(r.Findings[0].FixVersions, fixes) {
							t.Fatalf("consistent record evidence lost: %+v", r.Findings[0])
						}
					}
					after, _ := supplyJSON(s)
					if !bytes.Equal(before, after) {
						t.Fatal("original snapshot mutated")
					}
					got, _ := supplyJSON(r)
					if first != nil && !bytes.Equal(first, got) {
						t.Fatal("advisory or query order changed assessment")
					}
					first = got
				}
			}
		})
	}
}

func TestSecurityAliasDispositionAndSeverity(t *testing.T) {
	modules := []securityModule{{"example.org/a", "v1.2.3", "DIRECT", "BUILD"}}
	for _, severity := range []string{"CRITICAL", "HIGH", "MODERATE", "MEDIUM", "LOW", ""} {
		t.Run("severity_"+severity, func(t *testing.T) {
			for _, mode := range []string{"active", "withdrawn", "mixed-withdrawn", "exception"} {
				a := securityTestAdvisory("GO-2026-1000", modules[0].Name, modules[0].Version, severity)
				b := securityTestAdvisory("GHSA-aaaa-bbbb-cccc", modules[0].Name, modules[0].Version, "")
				a.Aliases, b.Aliases = []string{b.ID}, []string{a.ID}
				if mode == "withdrawn" || mode == "mixed-withdrawn" {
					a.Withdrawn = "2026-11-01T10:00:00Z"
				}
				if mode == "withdrawn" {
					b.Withdrawn = a.Withdrawn
				}
				s := securityTestSnapshot(securityIdentity{}, modules)
				s.Advisories, s.Queries[0].Advisories = []osvAdvisory{a, b}, []string{a.ID, b.ID}
				triage := securityTriage{"flashgate-security-triage/v1", []securityException{}}
				if mode == "exception" {
					triage.Exceptions = []securityException{{a.ID, modules[0].Name, modules[0].Version, "Controlled build inputs; review upgrade.", "maintainer", "2026-12-01T00:00:00Z"}}
				}
				r := assessSecurity(s.Identity, modules, s, triage, securityTestNow)
				if !r.Informational {
					t.Fatal("severity changed reporting-only policy")
				}
				if mode == "mixed-withdrawn" {
					if r.Status != "INCOMPLETE" || len(r.Findings) != 2 {
						t.Fatal("withdrawn mismatch was merged", r)
					}
					continue
				}
				if r.Status != "COMPLETE" || len(r.Findings) != 1 || r.Findings[0].Severity != securitySeverity(severity) || r.Findings[0].Priority != securityPriority(securitySeverity(severity)) {
					t.Fatalf("mode=%s: severity/priority changed or findings became blocking: %+v", mode, r)
				}
				f := r.Findings[0]
				if mode == "withdrawn" && f.Disposition != "WITHDRAWN_REVIEW_REQUIRED" || mode == "exception" && (f.Disposition != "DOCUMENTED_EXCEPTION_REVIEW_REQUIRED" || f.Exception == nil || f.Exception.Advisory != a.ID) {
					t.Fatalf("mode=%s: original identity disposition lost: %+v", mode, f)
				}
			}
		})
	}
}

func TestSecurityIntraAdvisoryFixConsistency(t *testing.T) {
	const module = "example.org/a"
	modules := []securityModule{{module, "v1.2.3", "DIRECT", "BUILD"}}
	interval := func(fix string) osvRange {
		return osvRange{Type: "SEMVER", Events: []osvEvent{{Introduced: "0"}, {Fixed: fix}}}
	}
	entry := func(ranges ...osvRange) osvAffected {
		return osvAffected{Package: osvPackage{module, "Go"}, Ranges: ranges}
	}
	explicit := entry(interval("1.3.0"))
	explicit.Versions = []string{"1.3.0+build"}
	disjoint := osvRange{Type: "SEMVER", Events: []osvEvent{{Introduced: "2.0.0"}, {Fixed: "2.1.0"}}}
	unsupported := osvRange{Type: "ECOSYSTEM", Events: []osvEvent{{Introduced: "0"}, {Fixed: "1.4.0"}}}
	invalid := osvRange{Type: "SEMVER", Events: []osvEvent{{Introduced: "1.4.0"}, {Fixed: "1.3.0"}}}
	last := osvRange{Type: "SEMVER", Events: []osvEvent{{Introduced: "0"}, {LastAffected: "1.3.0"}}}
	limit := osvRange{Type: "SEMVER", Events: []osvEvent{{Introduced: "0"}, {Limit: "1.3.0"}}}
	irrelevant := osvAffected{Package: osvPackage{"example.org/other", "Go"}, Ranges: []osvRange{unsupported}}
	for _, tc := range []struct {
		name, status, affected, quality string
		entries                         []osvAffected
		fixes                           []string
	}{
		{"overlapping ranges", "INCOMPLETE", "AFFECTED", "ADVISORY_FIX_CONFLICT", []osvAffected{entry(interval("1.3.0"), interval("1.4.0"))}, []string{"1.4.0"}},
		{"overlapping entries", "INCOMPLETE", "AFFECTED", "ADVISORY_FIX_CONFLICT", []osvAffected{entry(interval("1.3.0")), entry(interval("1.4.0"))}, []string{"1.4.0"}},
		{"explicit affected fix", "INCOMPLETE", "AFFECTED", "ADVISORY_FIX_CONFLICT", []osvAffected{explicit}, nil},
		{"inclusive last affected boundary", "INCOMPLETE", "AFFECTED", "ADVISORY_FIX_CONFLICT", []osvAffected{entry(interval("1.3.0"), last)}, nil},
		{"disjoint intervals", "COMPLETE", "AFFECTED", "", []osvAffected{entry(interval("1.3.0"), disjoint)}, []string{"1.3.0"}},
		{"identical fixes", "COMPLETE", "AFFECTED", "", []osvAffected{entry(interval("1.3.0"), interval("1.3.0"))}, []string{"1.3.0"}},
		{"equivalent fixes", "COMPLETE", "AFFECTED", "", []osvAffected{entry(interval("v1.3.0"), interval("1.3.0+build"))}, []string{"1.3.0+build", "v1.3.0"}},
		{"exclusive limit boundary", "COMPLETE", "AFFECTED", "", []osvAffected{entry(interval("1.3.0"), limit)}, []string{"1.3.0"}},
		{"unsupported range", "INCOMPLETE", "UNKNOWN", "ADVISORY_FIX_EVIDENCE_UNKNOWN", []osvAffected{entry(interval("1.3.0"), unsupported)}, nil},
		{"invalid range", "INCOMPLETE", "UNKNOWN", "ADVISORY_FIX_EVIDENCE_UNKNOWN", []osvAffected{entry(interval("1.3.0"), invalid)}, nil},
		{"missing matching evidence", "INCOMPLETE", "AFFECTED", "ADVISORY_FIX_EVIDENCE_UNKNOWN", []osvAffected{entry(interval("1.3.0")), entry()}, nil},
		{"irrelevant module evidence", "COMPLETE", "AFFECTED", "", []osvAffected{entry(interval("1.3.0")), irrelevant}, []string{"1.3.0"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var first []byte
			for _, reverse := range []bool{false, true} {
				a := securityTestAdvisory("GO-2026-1000", module, modules[0].Version, "HIGH")
				a.Affected = append([]osvAffected{}, tc.entries...)
				if reverse {
					for i := range a.Affected {
						a.Affected[i].Ranges = append([]osvRange{}, a.Affected[i].Ranges...)
						for x, y := 0, len(a.Affected[i].Ranges)-1; x < y; x, y = x+1, y-1 {
							a.Affected[i].Ranges[x], a.Affected[i].Ranges[y] = a.Affected[i].Ranges[y], a.Affected[i].Ranges[x]
						}
					}
					for i, j := 0, len(a.Affected)-1; i < j; i, j = i+1, j-1 {
						a.Affected[i], a.Affected[j] = a.Affected[j], a.Affected[i]
					}
				}
				s := securityTestSnapshot(securityIdentity{}, modules)
				s.Advisories, s.Queries[0].Advisories = []osvAdvisory{a}, []string{a.ID}
				before, _ := supplyJSON(s)
				r := securityTestAssess(modules, s)
				if r.Status != tc.status || !r.Informational || len(r.Findings) != 1 || tc.quality != "" && !containsString(r.Quality, tc.quality) || tc.quality == "" && len(r.Quality) != 0 {
					t.Fatalf("reverse=%v: status=%s quality=%v findings=%+v", reverse, r.Status, r.Quality, r.Findings)
				}
				f := r.Findings[0]
				if f.Affected != tc.affected || f.Severity != "HIGH" || f.Priority != "P2" || !reflect.DeepEqual(f.Records, []string{a.ID}) || !reflect.DeepEqual(f.FixVersions, securityUnion(nil, tc.fixes)) {
					t.Fatalf("fix validation lost installed impact or exposed unconfirmed fix: %+v", f)
				}
				if len(tc.fixes) == 0 && f.Remediation != "NO_CONFIRMED_FIX_REVIEW_MITIGATION" {
					t.Fatal("unconfirmed fix still offered for upgrade", f)
				}
				after, _ := supplyJSON(s)
				if !bytes.Equal(before, after) {
					t.Fatal("original provider evidence mutated")
				}
				got, _ := supplyJSON(r)
				if first != nil && !bytes.Equal(first, got) {
					t.Fatal("affected-entry/range order changed assessment")
				}
				first = got
			}
		})
	}
}

func TestSecurityIntraAdvisoryConflictPreventsAliasMerge(t *testing.T) {
	modules := []securityModule{{"example.org/a", "v1.2.3", "DIRECT", "BUILD"}}
	for _, severity := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", ""} {
		var first []byte
		for _, reverse := range []bool{false, true} {
			s := securityTestSnapshot(securityIdentity{}, modules)
			ids := []string{"GO-2026-1000", "GHSA-aaaa-bbbb-cccc", "CVE-2026-12345"}
			for i, id := range ids {
				a := securityTestAdvisory(id, modules[0].Name, modules[0].Version, severity)
				a.Affected[0].Versions = nil
				a.Affected[0].Ranges = []osvRange{{Type: "SEMVER", Events: []osvEvent{{Introduced: "0"}, {Fixed: "1.4.0"}}}}
				if i == 0 {
					a.Affected[0].Ranges = append(a.Affected[0].Ranges, osvRange{Type: "SEMVER", Events: []osvEvent{{Introduced: "0"}, {Fixed: "1.3.0"}}})
				}
				for _, other := range ids {
					if other != id {
						a.Aliases = append(a.Aliases, other)
					}
				}
				s.Advisories = append(s.Advisories, a)
				s.Queries[0].Advisories = append(s.Queries[0].Advisories, id)
			}
			if reverse {
				s.Advisories[0], s.Advisories[2] = s.Advisories[2], s.Advisories[0]
				s.Queries[0].Advisories[0], s.Queries[0].Advisories[2] = s.Queries[0].Advisories[2], s.Queries[0].Advisories[0]
			}
			before, _ := supplyJSON(s)
			r := securityTestAssess(modules, s)
			if r.Status != "INCOMPLETE" || !r.Informational || len(r.Findings) != len(ids) || !containsString(r.Quality, "ADVISORY_FIX_CONFLICT") {
				t.Fatal("internal conflict was hidden by alias merge", r)
			}
			for _, f := range r.Findings {
				if !reflect.DeepEqual(f.Records, []string{f.Advisory}) || !reflect.DeepEqual(f.FixVersions, []string{"1.4.0"}) || f.Affected != "AFFECTED" || f.Severity != securitySeverity(severity) || f.Priority != securityPriority(securitySeverity(severity)) {
					t.Fatal("original record identity or impact changed", f)
				}
			}
			after, _ := supplyJSON(s)
			got, _ := supplyJSON(r)
			if !bytes.Equal(before, after) || first != nil && !bytes.Equal(first, got) {
				t.Fatal("snapshot mutation or nondeterministic report")
			}
			first = got
		}
	}
}

func TestSecurityRangesAndVersions(t *testing.T) {
	for _, pair := range []struct {
		a, b string
		c    int
	}{{"v1.2.3", "1.2.4", -1}, {"v1.2.3-rc.1", "v1.2.3", -1}, {"v1.2.3-rc.10", "v1.2.3-rc.2", 1}, {"v1.2.3+meta", "1.2.3", 0}, {"v1.2.3-0.20261010120000-abcdef123456", "v1.2.3", -1}} {
		if c, err := securityCompare(pair.a, pair.b); err != nil || c != pair.c {
			t.Fatal(pair, c, err)
		}
	}
	for _, v := range []string{"1.2", "v01.2.3", "v1.2.3-01", "*", "1.2.3\n"} {
		if securityVersion(v) {
			t.Fatal("invalid semver accepted", v)
		}
	}
	r := osvRange{"SEMVER", []osvEvent{{Introduced: "0"}, {Fixed: "1.2.4"}, {Introduced: "2.0.0"}, {Fixed: "2.1.0"}}}
	for _, tc := range []struct {
		v        string
		affected bool
		fix      string
	}{{"v1.2.3", true, "1.2.4"}, {"v1.2.4", false, ""}, {"v2.0.1", true, "2.1.0"}, {"v2.1.0", false, ""}} {
		ok, fix, err := securityRange(tc.v, r)
		if err != nil || ok != tc.affected || (tc.fix != "" && (len(fix) != 1 || fix[0] != tc.fix)) {
			t.Fatal(tc, ok, fix, err)
		}
	}
	for _, r := range []osvRange{{"GIT", []osvEvent{{Introduced: "0"}}}, {"SEMVER", []osvEvent{{Fixed: "1.2.4"}}}, {"SEMVER", []osvEvent{{Introduced: "0", Fixed: "1.2.4"}}}, {"SEMVER", []osvEvent{{Introduced: "1.3.0"}, {Fixed: "1.2.0"}}}, {"SEMVER", []osvEvent{{Introduced: "0"}, {Fixed: "1.2.4"}, {Introduced: "1.1.0"}}}} {
		if _, _, err := securityRange("v1.2.3", r); err == nil {
			t.Fatal("malformed range accepted", r)
		}
	}
}

func TestSecurityExceptions(t *testing.T) {
	modules := []securityModule{{"example.org/a", "v1.2.3", "DIRECT", "BUILD"}}
	s := securityTestSnapshot(securityIdentity{}, modules)
	s.Advisories = []osvAdvisory{securityTestAdvisory("GO-2026-1000", modules[0].Name, modules[0].Version, "CRITICAL")}
	s.Queries[0].Advisories = []string{s.Advisories[0].ID}
	exact := securityException{s.Advisories[0].ID, modules[0].Name, modules[0].Version, "Controlled build inputs; review upgrade before next build.", "maintainer", "2026-12-01T00:00:00Z"}
	for name, mutate := range map[string]func(*securityException){"valid": func(*securityException) {}, "broad": func(e *securityException) { e.Version = "*" }, "module": func(e *securityException) { e.Module = "example.org/*" }, "expired": func(e *securityException) { e.ReviewBy = "2026-10-31T00:00:00Z" }, "owner": func(e *securityException) { e.Owner = "" }, "reason": func(e *securityException) { e.Reason = "" }, "unmatched": func(e *securityException) { e.Advisory = "CVE-2026-99999" }} {
		t.Run(name, func(t *testing.T) {
			e := exact
			mutate(&e)
			r := assessSecurity(s.Identity, modules, s, securityTriage{"flashgate-security-triage/v1", []securityException{e}}, securityTestNow)
			if len(r.Findings) != 1 || !r.Informational {
				t.Fatal(r)
			}
			if name == "valid" {
				if r.Status != "COMPLETE" || r.Findings[0].Exception == nil {
					t.Fatal(r)
				}
			} else if r.Status != "INCOMPLETE" || r.Findings[0].Exception != nil {
				t.Fatal(r)
			}
		})
	}
}

func TestSecurityCLI(t *testing.T) {
	dir, mh, ih, manifest := supplyFixture(t)
	id, modules, err := readSecurityInventory(dir, securityIdentity{Version: manifest.Version, Commit: manifest.SourceCommitSHA, Manifest: mh, Index: ih})
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	t.Setenv("FLASHGATE_WORK_ROOT", work)
	s := securityTestSnapshot(id, modules)
	s.Advisories = []osvAdvisory{securityTestAdvisory("GO-2026-1000", modules[0].Name, modules[0].Version, "CRITICAL")}
	s.Queries[0].Advisories = []string{s.Advisories[0].ID}
	snapshot := filepath.Join(work, "offline.json")
	data, _ := supplyJSON(s)
	if err := os.WriteFile(snapshot, data, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--dir", dir, "--version", id.Version, "--commit", id.Commit, "--manifest-sha256", mh, "--index-sha256", ih, "--snapshot", snapshot, "--at", securityTestNow.Format(time.RFC3339), "--output", filepath.Join(work, "result")}
	before, err := os.ReadFile(filepath.Join(dir, supplyIndexName))
	if err != nil {
		t.Fatal(err)
	}
	if err := runSecurity(args); err != nil {
		t.Fatal("critical finding blocked CLI", err)
	}
	cliArgs := append([]string{}, args...)
	cliArgs[len(cliArgs)-1] = filepath.Join(work, "cli-result")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", append([]string{"run", "-mod=vendor", ".", "security-assess"}, cliArgs...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("manual CLI routing failed: %v\n%s", err, output)
	}
	var result securityAssessment
	report, err := os.ReadFile(filepath.Join(work, "result", "assessment.json"))
	if err != nil || json.Unmarshal(report, &result) != nil || len(result.Findings) != 1 || result.Status != "COMPLETE" {
		t.Fatal(result, err)
	}
	after, err := os.ReadFile(filepath.Join(dir, supplyIndexName))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("input mutation")
	}
	if err := runSecurity(args); err == nil {
		t.Fatal("output collision accepted")
	}
	for _, bad := range [][]string{nil, {"--online", "--snapshot", snapshot, "--dir", dir}, {"--dir", dir, "--snapshot", snapshot, "--output", filepath.Join(dir, "unsafe")}} {
		if err := runSecurity(bad); err == nil {
			t.Fatal("invalid invocation accepted")
		}
	}
	args[len(args)-1] = filepath.Join(work, "invalid")
	if err := os.WriteFile(snapshot, []byte(`{"schema":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runSecurity(args); err == nil {
		t.Fatal("bad snapshot accepted")
	}
	bad, err := os.ReadFile(filepath.Join(work, "invalid", "assessment.json"))
	if err != nil || !strings.Contains(string(bad), "INCOMPLETE") {
		t.Fatal(string(bad), err)
	}
	if reflect.DeepEqual(result.Findings, []securityFinding{}) {
		t.Fatal("missing finding")
	}
}
