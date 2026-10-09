package version

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type workflowTarget struct {
	runner, platform, goarch, publicArch, extension string
}

var requiredWorkflowTargets = []workflowTarget{
	{"windows-latest", "windows", "amd64", "x64", "zip"},
	{"windows-latest", "windows", "arm64", "arm64", "zip"},
	{"ubuntu-latest", "linux", "amd64", "x64", "tar.gz"},
	{"ubuntu-latest", "linux", "arm64", "arm64", "tar.gz"},
}

func readRepositoryFile(t *testing.T, relative string) string {
	t.Helper()
	// go test executes this package in internal/version. Getwd remains absolute
	// with -trimpath, whereas runtime.Caller source paths do not.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(wd, "..", "..", relative))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func workflowMatrix(contents string) ([]workflowTarget, error) {
	lines := strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n")
	start := -1
	for i, line := range lines {
		if line == "        include:" && i > 0 && lines[i-1] == "      matrix:" {
			if start != -1 {
				return nil, fmt.Errorf("duplicate workflow matrix.include")
			}
			start = i + 1
		}
	}
	if start == -1 {
		return nil, fmt.Errorf("missing workflow matrix.include")
	}
	var targets []workflowTarget
	for i := start; i < len(lines); {
		line := lines[i]
		if line == "" {
			i++
			continue
		}
		if !strings.HasPrefix(line, "          ") {
			break
		}
		if !strings.HasPrefix(line, "          - os: ") || i+4 >= len(lines) {
			return nil, fmt.Errorf("unexpected matrix row at line %d", i+1)
		}
		prefixes := []string{
			"          - os: ", "            platform: ", "            goarch: ",
			"            public_arch: ", "            archive_extension: ",
		}
		values := make([]string, len(prefixes))
		for offset, prefix := range prefixes {
			candidate := lines[i+offset]
			if !strings.HasPrefix(candidate, prefix) {
				return nil, fmt.Errorf("matrix row missing %q at line %d", prefix, i+offset+1)
			}
			values[offset] = strings.TrimPrefix(candidate, prefix)
			if values[offset] == "" || strings.TrimSpace(values[offset]) != values[offset] {
				return nil, fmt.Errorf("invalid matrix value at line %d", i+offset+1)
			}
		}
		targets = append(targets, workflowTarget{values[0], values[1], values[2], values[3], values[4]})
		i += len(prefixes)
	}
	if len(targets) != len(requiredWorkflowTargets) {
		return nil, fmt.Errorf("expected four matrix rows, got %d", len(targets))
	}
	required := make(map[workflowTarget]bool, len(requiredWorkflowTargets))
	for _, target := range requiredWorkflowTargets {
		required[target] = true
	}
	seen := make(map[workflowTarget]bool, len(targets))
	for i, target := range targets {
		if !required[target] || seen[target] {
			return nil, fmt.Errorf("unexpected or duplicate matrix row %d: %+v", i+1, target)
		}
		seen[target] = true
	}
	return targets, nil
}

func namedWorkflowStep(contents, name string) (string, error) {
	marker := "      - name: " + name + "\n"
	if strings.Count(contents, marker) != 1 {
		return "", fmt.Errorf("expected one %s step", name)
	}
	after := strings.SplitN(contents, marker, 2)[1]
	if next := strings.Index(after, "      - name: "); next >= 0 {
		after = after[:next]
	}
	return after, nil
}

func uploadIdentity(step string) (string, string, string, error) {
	var name, archive, checksum string
	inPaths := false
	for _, line := range strings.Split(step, "\n") {
		if strings.HasPrefix(line, "          name: ") {
			if name != "" {
				return "", "", "", fmt.Errorf("duplicate upload name")
			}
			name = strings.TrimPrefix(line, "          name: ")
		}
		if line == "          path: |" {
			inPaths = true
			continue
		}
		if inPaths && strings.HasPrefix(line, "            build/") {
			value := strings.TrimSpace(line)
			if strings.HasSuffix(value, ".sha256") {
				if checksum != "" {
					return "", "", "", fmt.Errorf("duplicate checksum upload path")
				}
				checksum = value
			} else if strings.Contains(value, "flashgate-mcp_") || strings.Contains(value, "filesystem-mcp_") {
				if archive != "" {
					return "", "", "", fmt.Errorf("duplicate archive upload path")
				}
				archive = value
			}
		}
	}
	if name == "" || archive == "" || checksum == "" {
		return "", "", "", fmt.Errorf("upload name, archive, or checksum missing")
	}
	return name, archive, checksum, nil
}

func candidateUploadIdentity(contents, step string) (string, string, string, error) {
	if !strings.Contains(step, "          path: build/candidate-bundle/\n") {
		return "", "", "", fmt.Errorf("candidate upload does not bind the verified bundle")
	}
	bind, err := namedWorkflowStep(contents, "Bind verified candidate record")
	if err != nil {
		return "", "", "", err
	}
	if !strings.Contains(bind, "\"build/candidate-1/$base\"") ||
		!strings.Contains(bind, "\"build/candidate-1/$base.sha256\"") ||
		!strings.Contains(bind, "Copy-Item -LiteralPath $source -Destination $bundle") ||
		!strings.Contains(bind, "./cmd/releaseaudit candidate-record") {
		return "", "", "", fmt.Errorf("candidate bundle does not copy and bind verified archive/checksum")
	}
	const prefix = "          $base = \""
	var archive string
	for _, line := range strings.Split(bind, "\n") {
		if strings.HasPrefix(line, prefix) {
			archive = strings.TrimSuffix(strings.TrimPrefix(line, prefix), "\"")
		}
	}
	if archive == "" {
		return "", "", "", fmt.Errorf("candidate bundle archive name missing")
	}
	var container string
	for _, line := range strings.Split(step, "\n") {
		if strings.HasPrefix(line, "          name: ") {
			container = strings.TrimPrefix(line, "          name: ")
		}
	}
	if container == "" {
		return "", "", "", fmt.Errorf("candidate upload name missing")
	}
	return container, archive, archive + ".sha256", nil
}

func summaryIdentity(step, label string) (string, error) {
	prefix := "          echo \"- " + label + ": "
	var value string
	for _, line := range strings.Split(step, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		parts := strings.Split(line, "\\`")
		if value != "" || len(parts) != 3 || parts[2] != "\" >> \"$GITHUB_STEP_SUMMARY\"" {
			return "", fmt.Errorf("invalid or duplicate %s summary line", label)
		}
		value = parts[1]
	}
	if value == "" {
		return "", fmt.Errorf("missing %s summary line", label)
	}
	return value, nil
}

func renderIdentity(value, version string, target workflowTarget) (string, error) {
	const commit = "0123456789abcdef0123456789abcdef01234567"
	replacer := strings.NewReplacer(
		"${{ needs.prepare.outputs.version }}", version,
		"${{ needs.prepare.outputs.source_commit }}", commit,
		"${{ matrix.platform }}", target.platform,
		"${{ matrix.public_arch }}", target.publicArch,
		"${{ matrix.archive_extension }}", target.extension,
		"${{ github.run_id }}", "42",
		"${{ github.run_attempt }}", "1",
		"$RELEASE_VERSION", version,
		"$CANDIDATE_VERSION", version,
	)
	result := replacer.Replace(value)
	if strings.Contains(result, "${") || strings.Contains(result, "$") || strings.Contains(result, "${{") {
		return "", fmt.Errorf("unresolved name expression %q", result)
	}
	return result, nil
}

func validateWorkflowNames(contents, kind, version string) error {
	if strings.Contains(contents, "filesystem-mcp_") || strings.Contains(contents, "voxtronic-mcp_") {
		return fmt.Errorf("obsolete product artifact name in %s workflow", kind)
	}
	targets, err := workflowMatrix(contents)
	if err != nil {
		return err
	}
	upload, err := namedWorkflowStep(contents, "Upload validated "+kind+" artifact")
	if err != nil {
		return err
	}
	summary, err := namedWorkflowStep(contents, "Write "+kind+" summary")
	if err != nil {
		return err
	}
	var containerTemplate, archiveTemplate, checksumTemplate string
	if kind == "candidate" {
		containerTemplate, archiveTemplate, checksumTemplate, err = candidateUploadIdentity(contents, upload)
	} else {
		containerTemplate, archiveTemplate, checksumTemplate, err = uploadIdentity(upload)
	}
	if err != nil {
		return err
	}
	summaryArchive, err := summaryIdentity(summary, "Archive")
	if err != nil {
		return err
	}
	summaryChecksum, err := summaryIdentity(summary, "Checksum")
	if err != nil {
		return err
	}
	summaryContainer, err := summaryIdentity(summary, "Workflow artifact")
	if err != nil {
		return err
	}
	for _, target := range targets {
		archive := fmt.Sprintf("flashgate-mcp_%s_%s_%s.%s", version, target.platform, target.publicArch, target.extension)
		checksum := archive + ".sha256"
		container := fmt.Sprintf("flashgate-mcp-%s-%s-%s", version, target.platform, target.publicArch)
		directory := "build/release-1/"
		if kind == "candidate" {
			container = fmt.Sprintf("candidate-target-%s-%s-%s-42-1", "0123456789abcdef0123456789abcdef01234567", target.platform, target.publicArch)
			directory = ""
		}
		checks := []struct{ name, template, expected string }{
			{"archive upload", archiveTemplate, directory + archive},
			{"checksum upload", checksumTemplate, directory + checksum},
			{"Actions artifact", containerTemplate, container},
			{"archive summary", summaryArchive, archive},
			{"checksum summary", summaryChecksum, checksum},
			{"artifact summary", summaryContainer, container},
		}
		for _, check := range checks {
			actual, err := renderIdentity(check.template, version, target)
			if err != nil || actual != check.expected {
				return fmt.Errorf("%s %s/%s: got %q, want %q: %v", kind, target.platform, target.publicArch, check.name+"="+actual, check.expected, err)
			}
		}
	}
	return nil
}

func TestReleaseAndCandidateArtifactNameParity(t *testing.T) {
	fixtureData, err := os.ReadFile(filepath.Join("testdata", "build-metadata-fixtures.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures buildMetadataFixtureFile
	if err := json.Unmarshal(fixtureData, &fixtures); err != nil {
		t.Fatal(err)
	}
	var stable *buildMetadataFixture
	for i := range fixtures.Fixtures {
		if fixtures.Fixtures[i].Name == "stable" {
			stable = &fixtures.Fixtures[i]
		}
	}
	if stable == nil || stable.Version != "1.2.3" || len(stable.Targets) != 4 {
		t.Fatal("stable metadata fixture does not define four 1.2.3 targets")
	}
	for i, expected := range requiredWorkflowTargets {
		actual := stable.Targets[i]
		archive := fmt.Sprintf("flashgate-mcp_%s_%s_%s.%s", stable.Version, expected.platform, expected.publicArch, expected.extension)
		if actual.GOOS != expected.platform || actual.GOARCH != expected.goarch || actual.PublicArch != expected.publicArch || actual.Archive != archive {
			t.Fatalf("stable fixture target %d does not match workflow contract", i)
		}
	}

	for path, snippets := range map[string][]string{
		"scripts/New-ReleaseArtifact.ps1":  {"flashgate-mcp_${Version}_windows_${PublicArch}", "$ArtifactBaseName.zip"},
		"scripts/Test-ReleaseArtifact.ps1": {"flashgate-mcp_${ExpectedVersion}_windows_${ExpectedPublicArch}", "$ExpectedBaseName.zip"},
		"scripts/new-release-artifact.sh":  {"flashgate-mcp_${version}_linux_${public_arch}", "${artifact_base_name}.tar.gz"},
		"scripts/test-release-artifact.sh": {"flashgate-mcp_${expected_version}_linux_${expected_public_arch}", "${expected_base_name}.tar.gz"},
	} {
		contents := readRepositoryFile(t, path)
		for _, snippet := range snippets {
			if !strings.Contains(contents, snippet) {
				t.Fatalf("%s lacks canonical naming fragment %q", path, snippet)
			}
		}
	}
	for _, path := range []string{"README.md", "docs/build-metadata.md"} {
		contents := readRepositoryFile(t, path)
		for _, target := range requiredWorkflowTargets {
			archive := fmt.Sprintf("flashgate-mcp_<version>_%s_%s.%s", target.platform, target.publicArch, target.extension)
			if !strings.Contains(contents, archive) {
				t.Fatalf("%s lacks documented archive %q", path, archive)
			}
		}
	}
	workflows := []struct{ path, kind string }{
		{".github/workflows/release-build.yml", "release"},
		{".github/workflows/current-version-candidate.yml", "candidate"},
	}
	for _, workflow := range workflows {
		t.Run(workflow.kind, func(t *testing.T) {
			contents := readRepositoryFile(t, workflow.path)
			if err := validateWorkflowNames(contents, workflow.kind, stable.Version); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkflowMatrixAndNameMutations(t *testing.T) {
	for _, workflow := range []struct{ path, kind string }{
		{".github/workflows/release-build.yml", "release"},
		{".github/workflows/current-version-candidate.yml", "candidate"},
	} {
		t.Run(workflow.kind, func(t *testing.T) {
			contents := readRepositoryFile(t, workflow.path)
			rows := []string{
				"          - os: windows-latest\n            platform: windows\n            goarch: amd64\n            public_arch: x64\n            archive_extension: zip\n",
				"          - os: windows-latest\n            platform: windows\n            goarch: arm64\n            public_arch: arm64\n            archive_extension: zip\n",
				"          - os: ubuntu-latest\n            platform: linux\n            goarch: amd64\n            public_arch: x64\n            archive_extension: tar.gz\n",
				"          - os: ubuntu-latest\n            platform: linux\n            goarch: arm64\n            public_arch: arm64\n            archive_extension: tar.gz\n",
			}
			mutations := map[string]string{}
			for i, row := range rows {
				if !strings.Contains(contents, row) {
					t.Fatalf("matrix row %d missing before mutation", i)
				}
				mutations[fmt.Sprintf("remove-target-%d", i)] = strings.Replace(contents, row, "", 1)
			}
			mutations["wrong-amd64-x64"] = strings.Replace(contents, "            goarch: amd64\n            public_arch: x64", "            goarch: amd64\n            public_arch: arm64", 1)
			mutations["wrong-arm64"] = strings.Replace(contents, "            goarch: arm64\n            public_arch: arm64", "            goarch: amd64\n            public_arch: arm64", 1)
			mutations["wrong-extension"] = strings.Replace(contents, "            archive_extension: zip", "            archive_extension: tar.gz", 1)
			mutations["wrong-platform"] = strings.Replace(contents, "            platform: windows", "            platform: linux", 1)
			mutations["fifth-target"] = strings.Replace(contents, rows[3], rows[3]+rows[3], 1)
			uploadName := "          name: flashgate-mcp-${{ needs.prepare.outputs.version }}-${{ matrix.platform }}-${{ matrix.public_arch }}"
			if workflow.kind == "candidate" {
				uploadName = "          name: candidate-target-${{ needs.prepare.outputs.source_commit }}-${{ matrix.platform }}-${{ matrix.public_arch }}-${{ github.run_id }}-${{ github.run_attempt }}"
			}
			mutations["wrong-upload-name"] = strings.Replace(contents, uploadName, uploadName+"-wrong", 1)
			mutations["wrong-summary-archive"] = strings.Replace(contents, "- Archive: \\`", "- Archive: \\`wrong-", 1)
			mutations["wrong-summary-checksum"] = strings.Replace(contents, "- Checksum: \\`", "- Checksum: \\`wrong-", 1)
			mutations["old-product-name"] = strings.Replace(contents, "flashgate-mcp_", "filesystem-mcp_", 1)
			for name, mutated := range mutations {
				t.Run(name, func(t *testing.T) {
					if mutated == contents {
						t.Fatal("mutation did not change workflow")
					}
					if err := validateWorkflowNames(mutated, workflow.kind, "1.2.3"); err == nil {
						t.Fatal("unsafe workflow mutation passed")
					}
				})
			}
		})
	}
}
