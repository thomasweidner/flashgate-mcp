package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestReportSourcePathsHandleTrackedDeletions(t *testing.T) {
	paths, err := sourcePaths([]byte("? new file.go\x00H kept.go\x00H deleted.go\x00R deleted.go\x00"))
	slices.Sort(paths)
	if err != nil || !slices.Equal(paths, []string{"kept.go", "new file.go"}) {
		t.Fatalf("unexpected candidate inventory: %q, %v", paths, err)
	}
}

func TestReportRejectsIncompleteOrFailedEvidence(t *testing.T) {
	focused, expected := nativeFixture()
	const required = "TestMCPConformanceSchema202012"
	const extra = "TestMCPConformanceAdditionalContract"
	const row = "TestMCPConformanceSnapshots/revision/profile"
	event := func(action, test string) string { return fixtureEvent(serverPackage, action, test) }
	for _, tc := range []struct{ name, raw string }{
		{"empty", ""},
		{"malformed", focused + "{\n"},
		{"truncated", strings.TrimSuffix(focused, "\n")},
		{"wrong package", strings.ReplaceAll(focused, serverPackage, "other/cmd/server")},
		{"duplicate JSON field", strings.Replace(focused, `"Action":"run"`, `"Action":"run","action":"run"`, 1)},
		{"undated cached result", strings.Replace(focused, `"Time":"2026-01-01T00:00:00Z",`, "", 1)},
		{"cached output", strings.Replace(focused, event("pass", ""), `{"Time":"2026-01-01T00:00:00Z","Package":"`+serverPackage+`","Action":"output","Output":"ok (cached)"}`+"\n"+event("pass", ""), 1)},
		{"missing start", strings.Replace(focused, event("run", required), "", 1)},
		{"duplicate start", strings.Replace(focused, event("run", required), event("run", required)+event("run", required), 1)},
		{"missing pass", strings.Replace(focused, event("pass", required), "", 1)},
		{"duplicate pass", strings.Replace(focused, event("pass", required), event("pass", required)+event("pass", required), 1)},
		{"selected skip", strings.Replace(focused, event("pass", row), event("skip", row), 1)},
		{"selected failure", strings.Replace(focused, event("pass", row), event("fail", row), 1)},
		{"inverted hierarchy", strings.Replace(focused, event("run", "TestMCPConformanceSnapshots"), "", 1)},
		{"late failure", focused + event("fail", "")},
		{"missing required family", strings.Replace(focused, event("run", required)+event("pass", required), "", 1)},
		{"missing discovered test", strings.Replace(focused, event("run", extra)+event("pass", extra), "", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := testResults([]byte(tc.raw), expected); err == nil {
				t.Fatal("invalid evidence was accepted")
			}
		})
	}
}

func fixtureEvent(pkg, action, name string) string {
	return fmt.Sprintf("{\"Time\":\"2026-01-01T00:00:00Z\",\"Package\":%q,\"Action\":%q,\"Test\":%q}\n", pkg, action, name)
}

func nativeFixture() (string, map[string]bool) {
	var b strings.Builder
	event := func(action, name string) { b.WriteString(fixtureEvent(serverPackage, action, name)) }
	event("start", "")
	expected := make(map[string]bool)
	for _, name := range append(slices.Clone(requiredNativeFamilies), "TestMCPConformanceAdditionalContract") {
		expected[name] = true
		event("run", name)
		if name == "TestMCPConformanceSnapshots" {
			expected[name+"/revision"] = true
			expected[name+"/revision/profile"] = true
			event("run", name+"/revision")
			event("run", name+"/revision/profile")
			event("pass", name+"/revision/profile")
			event("pass", name+"/revision")
		}
		event("pass", name)
	}
	const parent = "TestFilesystemCallToolWireErrorsRemainSafe"
	expected[parent] = true
	event("run", parent)
	for _, child := range strings.Fields(nativeSubtests[parent]) {
		name := parent + "/" + child
		expected[name] = true
		event("run", name)
		event("output", name)
		event("output", name)
		event("pass", name)
	}
	event("pass", parent)
	event("pass", "")
	return b.String(), expected
}

func TestCoverageEvidenceRejectsWhollyMissingSubtest(t *testing.T) {
	raw, expected := nativeFixture()
	const omitted = "TestFilesystemCallToolWireErrorsRemainSafe/unknown_tool"
	if _, _, err := testResults([]byte(raw), expected); err != nil {
		t.Fatalf("complete stream rejected: %v", err)
	}
	var changed strings.Builder
	removed := 0
	for line := range strings.SplitAfterSeq(raw, "\n") {
		if strings.Contains(line, `"Test":"`+omitted+`"`) {
			removed++
			continue
		}
		changed.WriteString(line)
	}
	if removed != 4 {
		t.Fatalf("fixture removed %d events, want 4", removed)
	}
	_, _, err := testResults([]byte(changed.String()), expected)
	if err == nil || !strings.Contains(err.Error(), omitted) {
		t.Fatalf("stream missing %s was not rejected: %v", omitted, err)
	}
}

func TestCoverageInventoryMatchesActiveSource(t *testing.T) {
	t.Chdir(filepath.Join("..", ".."))
	names, err := nativeTestNames(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	const child = "TestFilesystemCallToolWireErrorsRemainSafe/unknown_tool"
	if !names[child] {
		t.Fatalf("active coverage inventory does not require %s", child)
	}
	if err := verifyNativeInventorySource(map[string][sha256.Size]byte{
		"cmd/server/call_tool_result_wire_test.go": sha256.Sum256([]byte("changed fixture")),
	}, nativeSubtestSourceSHA256); err == nil {
		t.Fatal("changed test source was admitted against the reviewed inventory")
	}
}

func TestCoverageEvidenceMatchesFocusedResults(t *testing.T) {
	focused, expected := nativeFixture()
	outside := fixtureEvent("other/cmd/server", "skip", "") + fixtureEvent(serverPackage, "skip", "TestUnrelated")
	broad := strings.Replace(focused, fixtureEvent(serverPackage, "start", ""), fixtureEvent(serverPackage, "start", "")+outside, 1)
	want, _, err := testResults([]byte(focused), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, projection, err := testResults([]byte(broad), expected)
	if err != nil || !maps.Equal(got, want) || !bytes.Equal(projection, []byte(focused)) {
		t.Fatalf("coverage projection differs: %v", err)
	}
}

func TestCoverageEvidenceAllowsSlashInOneSubtestName(t *testing.T) {
	raw, expected := nativeFixture()
	parent := "TestMCPConformanceAdditionalContract"
	name := parent + "/one/name"
	expected[name] = true
	terminal := fixtureEvent(serverPackage, "pass", parent)
	raw = strings.Replace(raw, terminal, fixtureEvent(serverPackage, "run", name)+
		fixtureEvent(serverPackage, "pass", name)+terminal, 1)
	if results, _, err := testResults([]byte(raw), expected); err != nil || results[name] != "PASS" {
		t.Fatalf("slash in one subtest name was rejected: %v", err)
	}
}

func TestRequirementMappingRequiresExecutedNativeIdentity(t *testing.T) {
	var mapping requirementMap
	if err := json.Unmarshal([]byte(`{"scenarios":[{"nativeCoverage":"TestMCPConformanceMapped"}]}`), &mapping); err != nil {
		t.Fatal(err)
	}
	if err := mapping.verifyNative(nil); err == nil {
		t.Fatal("mapping without executed test was accepted")
	}
	if err := mapping.verifyNative(map[string]string{"TestMCPConformanceMapped": "PASS"}); err != nil {
		t.Fatal(err)
	}
}

func TestCoverageInputChecksTheConsumedBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")
	data := []byte("admitted JSONL bytes\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	binding := coverageInput{path: path, digest: fmt.Sprintf("%x", sha256.Sum256(data)), origin: "validation-0123456789ab", size: int64(len(data))}
	if got, err := binding.read(filepath.Dir(path)); err != nil || !bytes.Equal(got, data) {
		t.Fatalf("admitted bytes rejected: %v", err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), len(data)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := binding.read(filepath.Dir(path)); err == nil {
		t.Fatal("changed coverage bytes accepted")
	}
}

func TestReportRequiresExecutedSnapshotRows(t *testing.T) {
	const path = "future-revision-future-profile.json"
	const row = "TestMCPConformanceSnapshots/future-revision/future-profile"
	const snapshot = `{"revision":"future-revision","profile":"future-profile","definitions":[{"name":"future_tool"}]}`
	if _, err := snapshotEvidence(path, []byte(snapshot), map[string]string{"TestMCPConformanceSnapshots": "PASS"}); err == nil {
		t.Fatal("accepted a snapshot with no executed matrix row")
	}
	results := map[string]string{row: "PASS"}
	if got, err := snapshotEvidence(path, []byte(snapshot), results); err != nil || !strings.Contains(got, "1 tools (future_tool)") {
		t.Fatalf("valid extension did not produce evidence: %q, %v", got, err)
	}
	for _, fixture := range []struct{ path, data string }{
		{path, "{invalid"},
		{"different-row.json", snapshot},
	} {
		if _, err := snapshotEvidence(fixture.path, []byte(fixture.data), results); err == nil {
			t.Fatalf("accepted invalid snapshot %s", fixture.path)
		}
	}
}
