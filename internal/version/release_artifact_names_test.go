package version

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseArtifactNameContractIsConsistent(t *testing.T) {
	t.Parallel()

	repositoryRoot := repositoryRootFromTestFile(t)
	checks := map[string][]string{
		"scripts/New-ReleaseArtifact.ps1": {
			`$ArtifactBaseName = "flashgate-mcp_${Version}_windows_${PublicArch}"`,
		},
		"scripts/Test-ReleaseArtifact.ps1": {
			`"flashgate-mcp_${ExpectedVersion}_windows_${ExpectedPublicArch}"`,
		},
		"scripts/new-release-artifact.sh": {
			`artifact_base_name="flashgate-mcp_${version}_linux_${public_arch}"`,
		},
		"scripts/test-release-artifact.sh": {
			`expected_base_name="flashgate-mcp_${expected_version}_linux_${expected_public_arch}"`,
		},
		".github/workflows/release-build.yml": {
			`build/release-1/flashgate-mcp_${{ needs.prepare.outputs.version }}_${{ matrix.platform }}_${{ matrix.public_arch }}.${{ matrix.archive_extension }}`,
			`build/release-1/flashgate-mcp_${{ needs.prepare.outputs.version }}_${{ matrix.platform }}_${{ matrix.public_arch }}.${{ matrix.archive_extension }}.sha256`,
		},
		"README.md": {
			"flashgate-mcp_<version>_windows_x64.zip",
			"flashgate-mcp_<version>_windows_arm64.zip",
			"flashgate-mcp_<version>_linux_x64.tar.gz",
			"flashgate-mcp_<version>_linux_arm64.tar.gz",
		},
		"docs/build-and-release-metadata.md": {
			"flashgate-mcp_<version>_windows_x64.zip",
			"flashgate-mcp_<version>_windows_arm64.zip",
			"flashgate-mcp_<version>_linux_x64.tar.gz",
			"flashgate-mcp_<version>_linux_arm64.tar.gz",
		},
	}

	for relativePath, requiredFragments := range checks {
		relativePath, requiredFragments := relativePath, requiredFragments
		t.Run(relativePath, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(relativePath)))
			if err != nil {
				t.Fatalf("read %s: %v", relativePath, err)
			}
			contents := string(data)
			for _, fragment := range requiredFragments {
				if !strings.Contains(contents, fragment) {
					t.Errorf("%s does not contain the canonical release-name fragment %q", relativePath, fragment)
				}
			}
		})
	}
}

func repositoryRootFromTestFile(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve release artifact name test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
