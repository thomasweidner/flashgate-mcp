//go:build linux

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Exercise real metadata stdout, rather than substituting a PASS-only verifier.
// Windows records remain the existing synthetic fixture: this test proves the
// Linux report contract, not Windows build provenance or native ARM64 execution.
func TestLinuxPortableCandidateEvidence(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	command := func(dir string, argv ...string) string {
		t.Helper()
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s: %v\n%s", argv[0], err, output)
		}
		return string(output)
	}
	commit := strings.TrimSpace(command(root, "git", "--no-optional-locks", "rev-parse", "HEAD"))
	stamp := strings.TrimSpace(command(root, "git", "--no-optional-locks", "show", "-s", "--format=%ct", "HEAD"))
	sourceTime := strings.TrimSpace(command(root, "date", "-u", "-d", "@"+stamp, "+%Y-%m-%dT%H:%M:%SZ"))
	modified := "false"
	if command(root, "git", "--no-optional-locks", "status", "--porcelain") != "" {
		modified = "true"
	}
	t.Setenv("SOURCE_DATE_EPOCH", stamp)
	dir, manifestPath, manifest := candidateFixture(t)
	manifest.SourceCommitSHA = commit
	before := map[string]string{}
	digest := func(path string) string {
		t.Helper()
		sum, err := hashFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return sum
	}
	for _, target := range candidateTargets {
		archive, checksum, evidence, compare, leak := candidateNames(manifest.Version, target)
		if target.Platform == "linux" {
			goarch := "amd64"
			if target.Architecture == "arm64" {
				goarch = "arm64"
			}
			second := t.TempDir()
			for _, out := range []string{dir, second} {
				command(root, "bash", filepath.Join(root, "scripts", "new-release-artifact.sh"),
					"--version", manifest.Version, "--goarch", goarch, "--output-directory", out)
			}
			before[archive] = digest(filepath.Join(dir, archive))
			before[checksum] = digest(filepath.Join(dir, checksum))
			report := command(dir, "bash", filepath.Join(root, "scripts", "test-release-artifact.sh"),
				"--archive", archive, "--checksum", checksum, "--expected-version", manifest.Version,
				"--expected-public-arch", target.Architecture, "--expected-goarch", goarch,
				"--expected-commit", commit, "--expected-source-time", sourceTime, "--expected-modified", modified)
			reference := strings.TrimSuffix(archive, ".tar.gz") + "/flashgate-mcp"
			if !strings.Contains(report, "BinaryPath: "+reference+"\n") ||
				!strings.Contains(report, "ExpectedCommit: "+commit+"\n") ||
				!strings.Contains(report, "ExpectedModified: "+modified+"\n") {
				t.Fatal("missing checked portable path or real build identity:\n", report)
			}
			if goarch == runtime.GOARCH && goarch != "arm64" &&
				(!strings.Contains(report, "RuntimeExecution: PASS\n") || !strings.Contains(report, "HelpContract: PASS\n")) {
				t.Fatal("native runtime contract did not execute:\n", report)
			}
			if goarch == "arm64" && !strings.Contains(report, "RuntimeExecution: SKIPPED\n") {
				t.Fatal("ARM64 archive verification must classify skipped execution:\n", report)
			}
			writeCandidateFixture(t, dir, evidence, report+"SourceCommit: "+commit+"\n")
			t.Logf("real %s report before hash binding:\n%s", target.Architecture, report)
			// The unchanged leak policy must accept actual portable metadata.
			var scanned leakReport
			scanBytes(evidence, []byte(report), []string{root, dir}, &scanned)
			if len(scanned.Findings) != 0 {
				t.Fatalf("portable metadata leaked: %+v", scanned.Findings)
			}
			for _, name := range []string{compare, leak} {
				if err := os.Remove(filepath.Join(dir, name)); err != nil {
					t.Fatal(err)
				}
			}
			if err := runCompare([]string{"--artifact-a", filepath.Join(dir, archive), "--checksum-a", filepath.Join(dir, checksum),
				"--artifact-b", filepath.Join(second, archive), "--checksum-b", filepath.Join(second, checksum),
				"--binary-suffix", "/flashgate-mcp", "--report", filepath.Join(dir, compare)}); err != nil {
				t.Fatal(err)
			}
			if err := runScan([]string{"--artifact", filepath.Join(dir, archive), "--checksum", filepath.Join(dir, checksum),
				"--report", filepath.Join(dir, leak), "--forbidden", root, "--forbidden", dir}); err != nil {
				t.Fatal(err)
			}
			// Use a real extracted binary to check default diagnostics, rejection of
			// a wrong layout, and metadata mismatch without weakening any checks.
			extracted := t.TempDir()
			command(root, "tar", "-xzf", filepath.Join(dir, archive), "-C", extracted, "--no-same-owner", "--no-same-permissions")
			binary := filepath.Join(extracted, filepath.FromSlash(reference))
			args := []string{filepath.Join(root, "scripts", "Test-LinuxMetadata.sh"), "--binary", binary,
				"--expected-product-version", manifest.Version, "--expected-file-version", manifest.Version + ".0",
				"--expected-public-arch", target.Architecture, "--expected-goarch", goarch, "--expected-commit", commit,
				"--expected-source-time", sourceTime, "--expected-modified", modified, "--skip-execution"}
			binaryHash := digest(binary)
			diagnostic := command(root, append([]string{"bash"}, args...)...)
			if !strings.Contains(diagnostic, "BinaryPath: "+binary+"\n") {
				t.Fatal("default diagnostic path changed")
			}
			t.Logf("real %s default diagnostic:\n%s", target.Architecture, diagnostic)
			var legacy leakReport
			scanBytes(evidence, []byte(diagnostic), []string{extracted}, &legacy)
			if len(legacy.Findings) == 0 {
				t.Fatal("unchanged leak policy accepted absolute diagnostics")
			}
			for _, mismatch := range []string{"layout", "commit"} {
				badArgs := append(append([]string{}, args...), "--portable-report")
				if mismatch == "layout" {
					wrong := filepath.Join(extracted, "wrong-layout")
					data, err := os.ReadFile(binary)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(wrong, data, 0o700); err != nil {
						t.Fatal(err)
					}
					badArgs[2] = wrong
				} else {
					for i := range badArgs {
						if badArgs[i] == "--expected-commit" {
							badArgs[i+1] = strings.Repeat("0", 40)
						}
					}
				}
				cmd := exec.Command("bash", badArgs...)
				cmd.Dir = root
				output, err := cmd.CombinedOutput()
				if err == nil || !strings.Contains(string(output), "Status: FAIL\n") {
					t.Fatalf("%s did not fail closed: %v\n%s", mismatch, err, output)
				}
			}
			if digest(binary) != binaryHash {
				t.Fatal("metadata verification changed binary bytes")
			}
		} else {
			data, err := os.ReadFile(filepath.Join(dir, evidence))
			if err != nil {
				t.Fatal(err)
			}
			writeCandidateFixture(t, dir, evidence, strings.ReplaceAll(string(data), testCandidateCommit, commit))
		}
		recordPath := filepath.Join(dir, "record-"+target.Platform+"-"+target.Architecture+".json")
		if err := os.Remove(recordPath); err != nil {
			t.Fatal(err)
		}
		if err := runCandidateRecord([]string{"--root", root, "--dir", dir, "--version", manifest.Version, "--commit", commit,
			"--platform", target.Platform, "--architecture", target.Architecture, "--output", recordPath}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(manifestPath); err != nil {
		t.Fatal(err)
	}
	if err := runCandidateManifest([]string{"--root", root, "--dir", dir, "--version", manifest.Version,
		"--commit", commit, "--output", manifestPath}); err != nil {
		t.Fatal(err)
	}
	if err := strictCandidateJSON(manifestPath, &manifest); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "supply")
	manifestHash := digest(manifestPath)
	if err := runSupply([]string{"--root", root, "--dir", dir, "--output", out, "--version", manifest.Version,
		"--commit", commit, "--manifest-sha256", manifestHash, "--forbidden", root, "--forbidden", dir}, false); err != nil {
		t.Fatal(err)
	}
	indexHash := digest(filepath.Join(out, supplyIndexName))
	verifyArgs := []string{"--dir", out, "--version", manifest.Version, "--commit", commit,
		"--manifest-sha256", manifestHash, "--index-sha256", indexHash, "--forbidden", root, "--forbidden", dir}
	if err := runSupply(verifyArgs, true); err != nil {
		t.Fatal(err)
	}
	for name, sum := range before {
		if digest(filepath.Join(dir, name)) != sum || digest(filepath.Join(out, "candidate", name)) != sum {
			t.Fatal("candidate bytes changed:", name)
		}
	}
	for _, record := range manifest.Records {
		if record.Platform != "linux" {
			continue
		}
		t.Logf("bound %s report SHA256: %s", record.Architecture, record.VerificationSHA256)
		path := filepath.Join(out, "candidate", record.VerificationEvidence)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, []byte("tampered\n")...), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := runSupply(verifyArgs, true); err == nil {
			t.Fatal("changed report accepted after hash binding")
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := runSupply(verifyArgs, true); err != nil {
		t.Fatal("restored exact evidence did not verify:", err)
	}
	t.Logf("real Linux reports, record hashes, leak scans, reproducibility and supply-chain CLI verification PASS; manifest=%s index=%s", manifestHash, indexHash)
}
