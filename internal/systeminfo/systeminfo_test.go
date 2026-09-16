package systeminfo

import (
	"reflect"
	"testing"
)

func TestRuntimeProviderReturnsReleasedFacts(t *testing.T) {
	info, err := NewProvider().Info()
	if err != nil {
		t.Fatal(err)
	}
	if info.OS == "" || info.Architecture == "" || info.Version == "" {
		t.Fatalf("incomplete system information: %#v", info)
	}
}

func TestFilteredEnvironmentReturnsOnlyNonEmptyAllowlistedValues(t *testing.T) {
	environment := map[string]string{
		"LANG":                  "en_US.UTF-8",
		"LC_ALL":                "",
		"PATH":                  "/private/bin",
		"API_TOKEN":             "secret",
		"AWS_SECRET_ACCESS_KEY": "secret",
	}
	got := filteredEnvironment(func(name string) (string, bool) {
		value, ok := environment[name]
		return value, ok
	})
	want := map[string]string{"LANG": "en_US.UTF-8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filtered environment=%#v, want %#v", got, want)
	}
}

func TestReleasedEnvironmentVariablesStayNarrow(t *testing.T) {
	want := [...]string{"COLORTERM", "LANG", "LC_ALL", "LC_CTYPE", "TERM"}
	if !reflect.DeepEqual(ReleasedEnvironmentVariables(), want[:]) {
		t.Fatalf("environment allowlist=%#v, want %#v", ReleasedEnvironmentVariables(), want)
	}
}
