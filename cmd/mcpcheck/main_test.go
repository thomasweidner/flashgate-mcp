package main

import (
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
	for _, raw := range []string{"", "invalid", `{"Action":"pass","Package":"server"}`, `{"Action":"fail","Test":"TestMCPConformanceSnapshots"}`, `{"Action":"skip","Test":"TestMCPConformanceSTDIOProcess"}`} {
		if _, err := testResults([]byte(raw)); err == nil {
			t.Fatalf("incomplete or failed evidence accepted: %q", raw)
		}
	}
	var complete strings.Builder
	const schemaEvent = `{"Action":"pass","Test":"TestMCPConformanceSchema202012"}` + "\n"
	for _, name := range []string{"TestMCPConformanceSnapshots", "TestMCPConformanceCompletenessMutations", "TestMCPConformanceWireNegatives", "TestMCPConformanceLegacyNegotiation", "TestMCPConformanceCrossRevisionStateless", "TestMCPConformanceSTDIOProcess", "TestMCPConformanceInventory", "TestStatelessCandidateWire", "TestStatelessMetadataAndProductionGate"} {
		complete.WriteString(`{"Action":"pass","Test":"` + name + `"}` + "\n")
	}
	complete.WriteString(`{"Action":"pass","Package":"server"}` + "\n")
	if _, err := testResults([]byte(complete.String())); err == nil || !strings.Contains(err.Error(), "TestMCPConformanceSchema202012") {
		t.Fatalf("report accepted missing schema evidence: %v", err)
	}
	complete.WriteString(schemaEvent)
	if _, err := testResults([]byte(complete.String())); err != nil {
		t.Fatal(err)
	}
	if _, err := testResults([]byte(complete.String() + `{"Action":"fail","Package":"server"}` + "\n")); err == nil {
		t.Fatal("late failure accepted")
	}
	if _, err := testResults([]byte(complete.String() + `{"Action":"skip","Test":"TestMCPConformanceSnapshots/revision/profile"}` + "\n")); err == nil {
		t.Fatal("skipped matrix row accepted despite passing parent")
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
