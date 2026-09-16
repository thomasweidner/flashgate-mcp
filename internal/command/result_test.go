package command

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func TestNewResultBuildsStableSerializableSchema(t *testing.T) {
	exitCode := 7
	result, err := NewResult(
		"git.status",
		ResultFailed,
		&exitCode,
		OutputSnapshot{Bytes: []byte{0xff, 0x00}, TotalBytes: 5, Truncated: true},
		OutputSnapshot{Bytes: []byte("bad"), TotalBytes: 3},
		[]DiagnosticCode{DiagnosticStdoutTruncated},
	)
	if err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"commandId":"git.status","status":"failed","exitCode":7,"timedOut":false,"stdout":{"encoding":"base64","data":"/wA=","retainedBytes":2,"totalBytes":5,"truncated":true},"stderr":{"encoding":"base64","data":"YmFk","retainedBytes":3,"totalBytes":3,"truncated":false},"diagnostics":["stdout_truncated"]}`
	if string(encoded) != want {
		t.Fatalf("json.Marshal(Result) = %s, want %s", encoded, want)
	}

	exitCode = 99
	if *result.ExitCode != 7 {
		t.Fatal("result aliases caller-owned exit code")
	}
}

func TestNewResultMakesTimeoutExplicitAndUsesEmptyDiagnostics(t *testing.T) {
	result, err := NewResult("build", ResultTimedOut, nil, OutputSnapshot{}, OutputSnapshot{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.TimedOut || result.ExitCode != nil || result.Diagnostics == nil || len(result.Diagnostics) != 0 {
		t.Fatalf("unexpected timeout result: %#v", result)
	}
}

func TestNewResultRejectsInconsistentTerminalStates(t *testing.T) {
	zero, nonzero := 0, 2
	tests := []struct {
		name   string
		status ResultStatus
		exit   *int
	}{
		{"unknown status", ResultStatus("other"), nil},
		{"success missing exit", ResultSucceeded, nil},
		{"success nonzero exit", ResultSucceeded, &nonzero},
		{"failure missing exit", ResultFailed, nil},
		{"failure zero exit", ResultFailed, &zero},
		{"timeout with exit", ResultTimedOut, &nonzero},
		{"canceled with exit", ResultCanceled, &nonzero},
		{"start failure with exit", ResultStartFailed, &nonzero},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewResult("build", test.status, test.exit, OutputSnapshot{}, OutputSnapshot{}, nil)
			if !errors.Is(err, ErrInvalidResult) {
				t.Fatalf("NewResult() error = %v", err)
			}
		})
	}
}

func TestNewResultRejectsInvalidSnapshotsAndDiagnostics(t *testing.T) {
	exitCode := 0
	valid := OutputSnapshot{}
	invalidSnapshots := []OutputSnapshot{
		{TotalBytes: -1},
		{Bytes: []byte("too long"), TotalBytes: 1, Truncated: true},
		{Bytes: []byte("x"), TotalBytes: 2},
		{Bytes: []byte("x"), TotalBytes: 1, Truncated: true},
	}
	for _, snapshot := range invalidSnapshots {
		if _, err := NewResult("build", ResultSucceeded, &exitCode, snapshot, valid, nil); !errors.Is(err, ErrInvalidResult) {
			t.Fatalf("NewResult() accepted snapshot %#v: %v", snapshot, err)
		}
	}

	diagnostics := []DiagnosticCode{DiagnosticCleanupIncomplete, DiagnosticCleanupIncomplete}
	if _, err := NewResult("build", ResultSucceeded, &exitCode, valid, valid, diagnostics); !errors.Is(err, ErrInvalidResult) {
		t.Fatalf("NewResult() accepted duplicate diagnostics: %v", err)
	}
	diagnostics = []DiagnosticCode{"raw operating-system error"}
	if _, err := NewResult("build", ResultSucceeded, &exitCode, valid, valid, diagnostics); !errors.Is(err, ErrInvalidResult) {
		t.Fatalf("NewResult() accepted arbitrary diagnostic: %v", err)
	}
}

func TestNewResultCopiesDiagnostics(t *testing.T) {
	exitCode := 0
	diagnostics := []DiagnosticCode{DiagnosticCleanupIncomplete}
	result, err := NewResult("build", ResultSucceeded, &exitCode, OutputSnapshot{}, OutputSnapshot{}, diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	diagnostics[0] = DiagnosticStderrTruncated
	if !reflect.DeepEqual(result.Diagnostics, []DiagnosticCode{DiagnosticCleanupIncomplete}) {
		t.Fatalf("result aliases diagnostics: %#v", result.Diagnostics)
	}
}
