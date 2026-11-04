package main

import (
	"strings"
	"testing"
)

func TestReportRejectsIncompleteOrFailedEvidence(t *testing.T) {
	for _, raw := range []string{"", "invalid", `{"Action":"pass","Package":"server"}`, `{"Action":"fail","Test":"TestMCPConformanceSnapshots"}`, `{"Action":"skip","Test":"TestMCPConformanceSTDIOProcess"}`} {
		if _, err := testResults([]byte(raw)); err == nil {
			t.Fatalf("incomplete or failed evidence accepted: %q", raw)
		}
	}
	var complete strings.Builder
	for _, name := range []string{"TestMCPConformanceSnapshots", "TestMCPConformanceCompletenessMutations", "TestMCPConformanceWireNegatives", "TestMCPConformanceLegacyNegotiation", "TestMCPConformanceCrossRevisionStateless", "TestMCPConformanceSTDIOProcess", "TestMCPConformanceInventory", "TestStatelessCandidateWire", "TestStatelessMetadataAndProductionGate"} {
		complete.WriteString(`{"Action":"pass","Test":"` + name + `"}` + "\n")
	}
	complete.WriteString(`{"Action":"pass","Package":"server"}` + "\n")
	if _, err := testResults([]byte(complete.String())); err != nil {
		t.Fatal(err)
	}
	if _, err := testResults([]byte(complete.String() + `{"Action":"fail","Package":"server"}` + "\n")); err == nil {
		t.Fatal("late failure accepted")
	}
}
