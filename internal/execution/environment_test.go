package execution

import (
	"errors"
	"reflect"
	"testing"
)

func TestEnvironmentPolicyBuildsExplicitDeterministicEnvironment(t *testing.T) {
	policy, err := NewEnvironmentPolicy([]string{"LANG", "TMPDIR", "FLASHGATE_MODE"})
	if err != nil {
		t.Fatalf("NewEnvironmentPolicy() error = %v", err)
	}

	got, err := policy.Build(map[string]string{
		"tmpdir":         "/tmp/command",
		"FLASHGATE_MODE": "safe",
		"LANG":           "C.UTF-8",
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	want := []string{"FLASHGATE_MODE=safe", "LANG=C.UTF-8", "TMPDIR=/tmp/command"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Build() = %#v, want %#v", got, want)
	}
}

func TestEnvironmentPolicyDoesNotInheritEnvironment(t *testing.T) {
	t.Setenv("FLASHGATE_PARENT_SECRET", "must-not-propagate")
	policy, err := NewEnvironmentPolicy(nil)
	if err != nil {
		t.Fatalf("NewEnvironmentPolicy() error = %v", err)
	}
	got, err := policy.Build(nil)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Build() inherited environment: %#v", got)
	}
}

func TestEnvironmentPolicyRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		values  map[string]string
		wantErr error
	}{
		{name: "malformed allowlist name", allowed: []string{"NOT=NAME"}, wantErr: ErrEnvironmentName},
		{name: "duplicate portable name", allowed: []string{"LANG", "lang"}, wantErr: ErrEnvironmentName},
		{name: "loader control", allowed: []string{"LD_PRELOAD"}, wantErr: ErrEnvironmentVariableDenied},
		{name: "interpreter control", allowed: []string{"PYTHONPATH"}, wantErr: ErrEnvironmentVariableDenied},
		{name: "hook control", allowed: []string{"GIT_CONFIG_SYSTEM"}, wantErr: ErrEnvironmentVariableDenied},
		{name: "not allowed", allowed: []string{"LANG"}, values: map[string]string{"SECRET_TOKEN": "secret"}, wantErr: ErrEnvironmentVariableNotAllowed},
		{name: "nul in value", allowed: []string{"LANG"}, values: map[string]string{"LANG": "C\x00evil"}, wantErr: ErrEnvironmentValue},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy, err := NewEnvironmentPolicy(test.allowed)
			if err == nil {
				_, err = policy.Build(test.values)
			}
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
		})
	}
}
