//go:build windows

package namedpipe

import "testing"

func TestValidName(t *testing.T) {
	for _, test := range []struct {
		name  string
		valid bool
	}{
		{`\\.\pipe\flashgate-test`, true},
		{`\\.\PIPE\flashgate-test`, true},
		{`\\server\pipe\flashgate`, false},
		{`\\.\pipe\`, false},
		{"\\\\.\\pipe\\flashgate\x00other", false},
		{`\\.\pipe\nested\name`, false},
		{`flashgate`, false},
	} {
		if got := validName(test.name); got != test.valid {
			t.Errorf("validName(%q) = %v, want %v", test.name, got, test.valid)
		}
	}
}

func TestListenRequiresBounds(t *testing.T) {
	if _, err := Listen(Config{Name: `\\.\pipe\flashgate-test`}); err == nil {
		t.Fatal("Listen accepted zero limits")
	}
}
