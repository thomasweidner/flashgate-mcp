package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestControlledBuildsUseSingleServerEntryPoint(t *testing.T) {
	t.Parallel()

	for _, script := range []string{"scripts/build.ps1", "scripts/build.sh"} {
		script := script
		t.Run(filepath.Base(script), func(t *testing.T) {
			t.Parallel()

			content := readRepositoryContractFile(t, script)
			if count := strings.Count(content, "./cmd/server"); count != 1 {
				t.Fatalf("%s must select ./cmd/server exactly once, got %d", script, count)
			}
			for _, forbidden := range []string{"./cmd/proxy", "./cmd/service", "./cmd/worker"} {
				if strings.Contains(content, forbidden) {
					t.Fatalf("%s selects forbidden split runtime entry point %q", script, forbidden)
				}
			}
		})
	}
}

func TestReleaseValidatorsRequireExactlyOneRuntimeBinary(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		path           string
		requiredBinary string
		unexpectedText string
	}{
		{
			path:           "scripts/Test-ReleaseArtifact.ps1",
			requiredBinary: "flashgate-mcp.exe",
			unexpectedText: "Unexpected ZIP content",
		},
		{
			path:           "scripts/test-release-artifact.sh",
			requiredBinary: "flashgate-mcp",
			unexpectedText: "unexpected archive content",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(filepath.Base(testCase.path), func(t *testing.T) {
			t.Parallel()

			content := readRepositoryContractFile(t, testCase.path)
			if !strings.Contains(content, testCase.unexpectedText) {
				t.Fatalf("%s must reject release archives with unexpected entries", testCase.path)
			}
			if !strings.Contains(content, testCase.requiredBinary) {
				t.Fatalf("%s does not require the primary runtime binary %q", testCase.path, testCase.requiredBinary)
			}
			for _, forbidden := range []string{"flashgate-proxy", "flashgate-service", "flashgate-worker"} {
				if strings.Contains(content, forbidden) {
					t.Fatalf("%s permits forbidden split runtime binary %q", testCase.path, forbidden)
				}
			}
		})
	}
}

func readRepositoryContractFile(t *testing.T, relativePath string) string {
	t.Helper()

	content, err := os.ReadFile(filepath.Join("..", "..", relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return string(content)
}
