//go:build linux

package process

import (
	"os"
	"testing"
)

func TestParseProcStatHandlesNamesWithSpacesAndParentheses(t *testing.T) {
	value := "42 (worker (one)) S 7 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 3 0"
	details, err := parseProcStat(42, value)
	if err != nil {
		t.Fatal(err)
	}
	if details.PID != 42 || details.Name != "worker (one)" || details.ParentPID != 7 || details.ThreadCount != 3 {
		t.Fatalf("unexpected details: %+v", details)
	}
}

func TestLocalDetailerReturnsCurrentProcessDetails(t *testing.T) {
	details, err := (LocalDetailer{}).Details(t.Context(), uint32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	if details.PID == 0 || details.Name == "" || details.ThreadCount == 0 {
		t.Fatalf("missing portable process details: %+v", details)
	}
}
