package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseWorkflowUsesCanonicalArtifactNames(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "release-build.yml"))
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	workflow := string(data)
	required := []string{
		"base = \"flashgate-mcp_$($env:RELEASE_VERSION)_windows_${{ matrix.public_arch }}\"",
		"base=\"flashgate-mcp_${RELEASE_VERSION}_linux_${{ matrix.public_arch }}\"",
		"name: flashgate-mcp-${{ needs.prepare.outputs.version }}-${{ matrix.platform }}-${{ matrix.public_arch }}",
		"build/release-1/flashgate-mcp_${{ needs.prepare.outputs.version }}_${{ matrix.platform }}_${{ matrix.public_arch }}.${{ matrix.archive_extension }}",
		"archive=\"flashgate-mcp_${RELEASE_VERSION}_${{ matrix.platform }}_${{ matrix.public_arch }}.${{ matrix.archive_extension }}\"",
		"echo \"- Archive: \\`$archive\\`\"",
		"echo \"- Checksum: \\`$archive.sha256\\`\"",
		"echo \"- Workflow artifact: \\`flashgate-mcp-${RELEASE_VERSION}-${{ matrix.platform }}-${{ matrix.public_arch }}\\`\"",
	}
	for _, fragment := range required {
		if !strings.Contains(workflow, fragment) {
			t.Errorf("release workflow is missing canonical artifact-name fragment %q", fragment)
		}
	}
	for _, legacy := range []string{"fileserver_", "fileserver-", "filesystem-mcp_", "filesystem-mcp-"} {
		if strings.Contains(strings.ToLower(workflow), legacy) {
			t.Errorf("release workflow contains legacy artifact-name marker %q", legacy)
		}
	}
}

func TestReleaseArtifactFixtureCoversPublishedMatrix(t *testing.T) {
	t.Parallel()
	fixture := loadBuildMetadataFixtures(t)
	stable := fixtureByName(t, fixture, "stable")
	want := map[string]bool{
		"flashgate-mcp_1.2.3_windows_x64.zip":    false,
		"flashgate-mcp_1.2.3_windows_arm64.zip":  false,
		"flashgate-mcp_1.2.3_linux_x64.tar.gz":   false,
		"flashgate-mcp_1.2.3_linux_arm64.tar.gz": false,
	}
	for _, target := range stable.Targets {
		if _, ok := want[target.Archive]; !ok {
			t.Errorf("unexpected stable release archive %q", target.Archive)
			continue
		}
		want[target.Archive] = true
	}
	for archive, found := range want {
		if !found {
			t.Errorf("stable release fixture does not cover %q", archive)
		}
	}
}

func loadBuildMetadataFixtures(t *testing.T) buildMetadataFixtureFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "build-metadata-fixtures.json"))
	if err != nil {
		t.Fatalf("read build metadata fixtures: %v", err)
	}
	var fixture buildMetadataFixtureFile
	decodeStrictFixture(t, data, &fixture)
	return fixture
}

func fixtureByName(t *testing.T, fixture buildMetadataFixtureFile, name string) buildMetadataFixture {
	t.Helper()
	for _, candidate := range fixture.Fixtures {
		if candidate.Name == name {
			return candidate
		}
	}
	t.Fatalf("build metadata fixture %q is missing", name)
	return buildMetadataFixture{}
}
