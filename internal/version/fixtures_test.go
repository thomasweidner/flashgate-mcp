package version

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type buildMetadataFixtureFile struct {
	Schema   string                 `json:"schema"`
	Fixtures []buildMetadataFixture `json:"fixtures"`
}

type buildMetadataFixture struct {
	Name             string                       `json:"name"`
	Version          string                       `json:"version"`
	FileVersion      string                       `json:"fileVersion"`
	Compact          string                       `json:"compact"`
	ReleaseArtifacts bool                         `json:"releaseArtifacts"`
	Targets          []buildMetadataTargetFixture `json:"targets"`
}

type buildMetadataTargetFixture struct {
	GOOS       string `json:"goos"`
	GOARCH     string `json:"goarch"`
	PublicArch string `json:"publicArch"`
	Archive    string `json:"archive"`
}

type buildInputValidationFixtureFile struct {
	Schema           string                    `json:"schema"`
	SemanticVersions semanticVersionFixtureSet `json:"semanticVersions"`
	SourceDateEpoch  sourceEpochFixtureSet     `json:"sourceDateEpoch"`
}

type semanticVersionFixtureSet struct {
	Valid   []validSemanticVersionFixture `json:"valid"`
	Invalid []string                      `json:"invalid"`
}

type validSemanticVersionFixture struct {
	Value       string `json:"value"`
	FileVersion string `json:"fileVersion"`
}

type sourceEpochFixtureSet struct {
	Valid   []validSourceEpochFixture `json:"valid"`
	Invalid []string                  `json:"invalid"`
}

type validSourceEpochFixture struct {
	Value      string `json:"value"`
	SourceTime string `json:"sourceTime"`
}

func decodeStrictFixture(t *testing.T, data []byte, target any) {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		t.Fatalf("decode fixture file: %v", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("fixture file contains trailing JSON data: %v", err)
	}
}

func TestBuildMetadataFixtures(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("testdata", "build-metadata-fixtures.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture file: %v", err)
	}

	var fixtureFile buildMetadataFixtureFile
	decodeStrictFixture(t, data, &fixtureFile)
	if fixtureFile.Schema != "flashgate-build-metadata-fixtures/v1" {
		t.Fatalf("unexpected fixture schema %q", fixtureFile.Schema)
	}
	if len(fixtureFile.Fixtures) != 3 {
		t.Fatalf("expected 3 fixtures, got %d", len(fixtureFile.Fixtures))
	}

	seenNames := make(map[string]struct{}, len(fixtureFile.Fixtures))
	requiredNames := map[string]bool{
		"stable":      false,
		"prerelease":  false,
		"development": false,
	}

	for _, fixture := range fixtureFile.Fixtures {
		fixture := fixture
		t.Run(fixture.Name, func(t *testing.T) {
			if fixture.Name == "" {
				t.Fatal("fixture name is empty")
			}
			if _, exists := seenNames[fixture.Name]; exists {
				t.Fatalf("duplicate fixture name %q", fixture.Name)
			}
			seenNames[fixture.Name] = struct{}{}
			if _, required := requiredNames[fixture.Name]; required {
				requiredNames[fixture.Name] = true
			}

			mapped, err := WindowsFileVersion(fixture.Version)
			if err != nil {
				t.Fatalf("WindowsFileVersion(%q): %v", fixture.Version, err)
			}
			if mapped != fixture.FileVersion {
				t.Fatalf("expected file version %q, got %q", fixture.FileVersion, mapped)
			}

			info := Info{
				ProductName: ProductName,
				BinaryName:  BinaryName,
				Version:     fixture.Version,
				FileVersion: fixture.FileVersion,
			}
			if compact := info.String(); compact != fixture.Compact {
				t.Fatalf("expected compact output %q, got %q", fixture.Compact, compact)
			}

			if !fixture.ReleaseArtifacts {
				if len(fixture.Targets) != 0 {
					t.Fatalf("non-release fixture contains %d targets", len(fixture.Targets))
				}
				return
			}
			if len(fixture.Targets) != 4 {
				t.Fatalf("release fixture must contain 4 targets, got %d", len(fixture.Targets))
			}

			targetKeys := make(map[string]struct{}, len(fixture.Targets))
			for _, target := range fixture.Targets {
				if target.GOOS != "windows" && target.GOOS != "linux" {
					t.Fatalf("unsupported GOOS %q", target.GOOS)
				}
				if target.GOARCH != "amd64" && target.GOARCH != "arm64" {
					t.Fatalf("unsupported GOARCH %q", target.GOARCH)
				}
				if PublicArchitecture(target.GOARCH) != target.PublicArch {
					t.Fatalf(
						"public architecture mismatch for %s: expected %q, got %q",
						target.GOARCH,
						PublicArchitecture(target.GOARCH),
						target.PublicArch,
					)
				}

				key := target.GOOS + "/" + target.GOARCH
				if _, exists := targetKeys[key]; exists {
					t.Fatalf("duplicate target %q", key)
				}
				targetKeys[key] = struct{}{}

				extension := ".tar.gz"
				if target.GOOS == "windows" {
					extension = ".zip"
				}
				expectedArchive := fmt.Sprintf(
					"flashgate-mcp_%s_%s_%s%s",
					fixture.Version,
					target.GOOS,
					target.PublicArch,
					extension,
				)
				if target.Archive != expectedArchive {
					t.Fatalf("expected archive %q, got %q", expectedArchive, target.Archive)
				}
				if strings.Contains(target.Archive, "_amd64") ||
					strings.Contains(target.Archive, "_x86_64") ||
					strings.Contains(target.Archive, "_aarch64") {
					t.Fatalf("archive exposes a forbidden public architecture alias: %q", target.Archive)
				}
			}
		})
	}

	for name, found := range requiredNames {
		if !found {
			t.Fatalf("required fixture %q is missing", name)
		}
	}
}

func TestBuildInputValidationFixtures(t *testing.T) {
	t.Parallel()

	fixturePath := filepath.Join("testdata", "build-input-validation-fixtures.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture file: %v", err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()

	var fixtureFile buildInputValidationFixtureFile
	if err := decoder.Decode(&fixtureFile); err != nil {
		t.Fatalf("decode fixture file: %v", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		t.Fatalf("fixture file contains trailing JSON data: %v", err)
	}
	if fixtureFile.Schema != "flashgate-build-input-validation-fixtures/v1" {
		t.Fatalf("unexpected fixture schema %q", fixtureFile.Schema)
	}

	for _, fixture := range fixtureFile.SemanticVersions.Valid {
		fixture := fixture
		t.Run("semver-valid-"+fixture.Value, func(t *testing.T) {
			actual, err := WindowsFileVersion(fixture.Value)
			if err != nil {
				t.Fatalf("WindowsFileVersion(%q): %v", fixture.Value, err)
			}
			if actual != fixture.FileVersion {
				t.Fatalf("expected %q, got %q", fixture.FileVersion, actual)
			}
		})
	}
	for index, value := range fixtureFile.SemanticVersions.Invalid {
		value := value
		t.Run(fmt.Sprintf("semver-invalid-%d", index), func(t *testing.T) {
			if _, err := WindowsFileVersion(value); err == nil {
				t.Fatalf("WindowsFileVersion(%q) unexpectedly succeeded", value)
			}
		})
	}

	for _, fixture := range fixtureFile.SourceDateEpoch.Valid {
		fixture := fixture
		t.Run("epoch-valid-"+fixture.Value, func(t *testing.T) {
			actual, err := SourceTimeFromEpoch(fixture.Value)
			if err != nil {
				t.Fatalf("SourceTimeFromEpoch(%q): %v", fixture.Value, err)
			}
			if actual != fixture.SourceTime {
				t.Fatalf("expected %q, got %q", fixture.SourceTime, actual)
			}
		})
	}
	for index, value := range fixtureFile.SourceDateEpoch.Invalid {
		value := value
		t.Run(fmt.Sprintf("epoch-invalid-%d", index), func(t *testing.T) {
			if _, err := SourceTimeFromEpoch(value); err == nil {
				t.Fatalf("SourceTimeFromEpoch(%q) unexpectedly succeeded", value)
			}
		})
	}
}
