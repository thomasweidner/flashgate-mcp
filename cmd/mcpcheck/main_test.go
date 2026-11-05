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
}
