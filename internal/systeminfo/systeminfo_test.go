package systeminfo

import "testing"

func TestRuntimeProviderReturnsReleasedFacts(t *testing.T) {
	info, err := NewProvider().Info()
	if err != nil {
		t.Fatal(err)
	}
	if info.OS == "" || info.Architecture == "" || info.Version == "" {
		t.Fatalf("incomplete system information: %#v", info)
	}
}
