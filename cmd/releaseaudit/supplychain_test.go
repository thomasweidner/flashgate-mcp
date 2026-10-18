package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const supplyTestTime = "2026-10-11T12:00:00Z"

func validateSupplyWorkflow(contents string) error {
	for _, required := range []string{
		"      - name: Test supply-chain contracts independently of candidate matrix\n        shell: bash",
		"run: go test -mod=vendor ./cmd/releaseaudit",
		"./cmd/releaseaudit supply-chain-generate",
		"./cmd/releaseaudit supply-chain-verify",
		"--root . --dir build/candidate-bundle --output build/supply-chain",
		"--manifest-sha256 \"$manifest_hash\" --index-sha256 \"$index_hash\"",
		"--forbidden \"$GITHUB_WORKSPACE\" --forbidden \"$RUNNER_TEMP\" --forbidden \"$HOME\" --forbidden \"$USER\" --forbidden \"$(hostname)\"",
		"path: build/supply-chain/",
		"name: candidate-evidence-${{ needs.prepare.outputs.source_commit }}-${{ github.run_id }}-${{ github.run_attempt }}",
		"SupplyChainIndexSHA256: %s",
	} {
		if !strings.Contains(contents, required) {
			return errors.New("missing supply-chain workflow gate: " + required)
		}
	}
	if strings.Contains(contents, "      - name: Set up Go\n        if: steps.candidate.outputs.run_candidate") {
		return errors.New("supply tests must run when VERSION is unchanged")
	}
	if strings.Count(contents, "./cmd/releaseaudit supply-chain-generate") != 1 || strings.Count(contents, "matrix:") != 1 {
		return errors.New("duplicate supply generation or build matrix")
	}
	if strings.Count(contents, "--manifest-sha256 \"$manifest_hash\"") != 2 || strings.Count(contents, "--index-sha256 \"$index_hash\"") != 1 {
		return errors.New("both evidence commands require independent manifest anchors")
	}
	start := strings.Index(contents, "      - name: Generate and independently verify supply-chain evidence")
	verify := strings.Index(contents, "./cmd/releaseaudit supply-chain-verify")
	upload := strings.Index(contents, "      - name: Upload complete verified supply-chain bundle")
	if start < 0 || verify < start || upload < verify {
		return errors.New("evidence must verify before upload")
	}
	return nil
}

func TestSupplyWorkflowPolicy(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "current-version-candidate.yml"))
	if err != nil {
		t.Fatal(err)
	}
	contents := strings.ReplaceAll(string(data), "\r\n", "\n")
	if err := validateCandidateWorkflow(contents); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct{ old, new string }{
		{"supply-chain-verify", "echo skip-verification"},
		{"supply-chain-generate", "other-generator"},
		{"--index-sha256 \"$index_hash\"", ""},
		{"--manifest-sha256 \"$manifest_hash\"", ""},
		{"--forbidden \"$RUNNER_TEMP\"", ""},
		{"path: build/supply-chain/", "path: build/candidate-bundle/"},
		{"      - name: Test supply-chain contracts independently of candidate matrix\n        shell: bash", "      - name: Test supply-chain contracts independently of candidate matrix\n        if: needs.prepare.outputs.run_candidate == 'true'\n        shell: bash"},
		{"  contents: read", "  contents: write"},
	} {
		changed := strings.Replace(contents, mutation.old, mutation.new, 1)
		if changed == contents {
			t.Fatal("mutation did not apply")
		}
		if err := validateCandidateWorkflow(changed); err == nil {
			t.Fatal("unsafe supply-chain workflow accepted", mutation.old)
		}
	}
}

func supplySourceFixture(t *testing.T) string {
	t.Helper()
	return supplyVersionSourceFixture(t, "0.3.0")
}

func supplyVersionSourceFixture(t *testing.T, value string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range supplySourceNames {
		data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if name == "VERSION" {
			data = []byte(value + "\n")
		}
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestSupplyFixtureVersionChange(t *testing.T) {
	for _, value := range []string{"0.4.0", "0.4.0-rc.1"} {
		t.Run(value, func(t *testing.T) {
			dir, path, manifest := candidateVersionFixture(t, value)
			source := supplyVersionSourceFixture(t, manifest.Version)
			out := filepath.Join(t.TempDir(), "evidence")
			if err := generateSupply(source, dir, out, supplyTestTime, manifest, nil, nil); err != nil {
				t.Fatal("matching changed version rejected:", err)
			}
			manifestHash, err := hashFile(path)
			if err != nil {
				t.Fatal(err)
			}
			indexHash, err := hashFile(filepath.Join(out, supplyIndexName))
			if err != nil {
				t.Fatal(err)
			}
			if err := verifySupply(out, value, manifest.SourceCommitSHA, manifestHash, indexHash, nil); err != nil {
				t.Fatal("changed version evidence did not verify:", err)
			}
			if err := verifySupply(out, "0.3.0", manifest.SourceCommitSHA, manifestHash, indexHash, nil); err == nil {
				t.Fatal("wrong external version accepted")
			}
			writeCandidateFixture(t, source, "VERSION", "0.3.0\n")
			parent := t.TempDir()
			if err := generateSupply(source, dir, filepath.Join(parent, "rejected"), supplyTestTime, manifest, nil, nil); err == nil || !strings.Contains(err.Error(), "source VERSION mismatch") {
				t.Fatalf("mismatched source version did not fail closed: %v", err)
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 0 {
				t.Fatal("mismatched version left visible output:", entries, err)
			}
		})
	}
}

func supplyFixture(t *testing.T) (string, string, string, candidateManifest) {
	t.Helper()
	dir, manifestPath, manifest := candidateFixture(t)
	out := filepath.Join(t.TempDir(), "evidence")
	if err := generateSupply(supplySourceFixture(t), dir, out, supplyTestTime, manifest, nil, nil); err != nil {
		t.Fatal(err)
	}
	manifestHash, err := hashFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	indexHash, err := hashFile(filepath.Join(out, supplyIndexName))
	if err != nil {
		t.Fatal(err)
	}
	return out, manifestHash, indexHash, manifest
}

func TestSupplyDeterminismAndSPDX(t *testing.T) {
	dir, _, manifest := candidateFixture(t)
	source := supplySourceFixture(t)
	outputs := []string{filepath.Join(t.TempDir(), "one"), filepath.Join(t.TempDir(), "two")}
	for _, out := range outputs {
		if err := generateSupply(source, dir, out, supplyTestTime, manifest, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{sbomName, supplyIndexName, "provenance-windows-x64.json", "provenance-windows-arm64.json", "provenance-linux-x64.json", "provenance-linux-arm64.json"} {
		a, err := os.ReadFile(filepath.Join(outputs[0], name))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(outputs[1], name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Fatal("nondeterministic evidence", name)
		}
	}
	data, _ := os.ReadFile(filepath.Join(outputs[0], sbomName))
	var sbom map[string]any
	if err := json.Unmarshal(data, &sbom); err != nil {
		t.Fatal(err)
	}
	if sbom["spdxVersion"] != "SPDX-2.3" || sbom["dataLicense"] != "CC0-1.0" {
		t.Fatal("invalid SPDX identity")
	}
	packages := sbom["packages"].([]any)
	if len(packages) != 5 {
		t.Fatal("incomplete source inventory")
	}
	ids := map[string]bool{"SPDXRef-DOCUMENT": true}
	for _, p := range packages {
		pkg := p.(map[string]any)
		id := pkg["SPDXID"].(string)
		if ids[id] {
			t.Fatal("duplicate package")
		}
		ids[id] = true
		if strings.HasPrefix(id, "SPDXRef-GoModule") {
			if _, ok := pkg["checksums"]; ok {
				t.Fatal("h1 misclassified as archive checksum")
			}
			if !strings.Contains(pkg["comment"].(string), "h1:") {
				t.Fatal("missing Go checksum")
			}
		}
	}
	for _, r := range sbom["relationships"].([]any) {
		rel := r.(map[string]any)
		if !ids[rel["spdxElementId"].(string)] || !ids[rel["relatedSpdxElement"].(string)] {
			t.Fatal("dangling relationship")
		}
	}
	for _, record := range manifest.Records {
		data, _ := os.ReadFile(filepath.Join(outputs[0], "provenance-"+record.Platform+"-"+record.Architecture+".json"))
		var statement map[string]any
		if err := json.Unmarshal(data, &statement); err != nil {
			t.Fatal(err)
		}
		manifestBytes, err := os.ReadFile(filepath.Join(dir, "candidate-manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		expected := map[string]any{
			"_type":         "https://in-toto.io/Statement/v1",
			"subject":       []any{map[string]any{"name": record.Archive, "digest": map[string]any{"sha256": record.ArtifactSHA256}}},
			"predicateType": "https://github.com/thomasweidner/flashgate-mcp/attestations/candidate-verification/v1",
			"predicate": map[string]any{
				"version": manifest.Version, "platform": record.Platform, "architecture": record.Architecture,
				"source":            map[string]any{"uri": "git+https://github.com/thomasweidner/flashgate-mcp", "digest": map[string]any{"gitCommit": manifest.SourceCommitSHA}},
				"candidateManifest": map[string]any{"path": "candidate/candidate-manifest.json", "sha256": supplyDigest(manifestBytes)},
				"verification":      map[string]any{"path": "candidate/" + record.VerificationEvidence, "sha256": record.VerificationSHA256},
				"reproduction":      map[string]any{"path": "candidate/" + record.CompareReport, "sha256": record.CompareSHA256},
			},
		}
		if !reflect.DeepEqual(statement, expected) {
			t.Fatal("candidate verification contract differs", statement)
		}
		for _, unproven := range []string{"startedOn", "finishedOn", "builder", "signature"} {
			if bytes.Contains(data, []byte(`"`+unproven+`"`)) {
				t.Fatal("invented provenance", unproven)
			}
		}
	}
	for _, name := range promotedNames(manifest) {
		original, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		carried, err := os.ReadFile(filepath.Join(outputs[0], "candidate", name))
		if err != nil || !bytes.Equal(original, carried) {
			t.Fatal("candidate bytes changed", name, err)
		}
	}
	if err := generateSupply(source, dir, outputs[0], supplyTestTime, manifest, nil, nil); err == nil {
		t.Fatal("existing immutable evidence accepted")
	}
}

func TestSupplyPredicateRejectsReboundStatements(t *testing.T) {
	d, manifestHash, _, c := supplyFixture(t)
	name := "provenance-windows-x64.json"
	original, err := os.ReadFile(filepath.Join(d, name))
	if err != nil {
		t.Fatal(err)
	}
	indexBytes, err := os.ReadFile(filepath.Join(d, supplyIndexName))
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"old SLSA", "legacy shape", "future type", "subject", "source", "manifest", "verification", "reproduction", "missing", "unknown", "null", "duplicate", "trailing"} {
		t.Run(mutation, func(t *testing.T) {
			var statement map[string]any
			if err := json.Unmarshal(original, &statement); err != nil {
				t.Fatal(err)
			}
			predicate := statement["predicate"].(map[string]any)
			switch mutation {
			case "old SLSA":
				statement["predicateType"] = "https://slsa.dev/provenance/v1"
				fallthrough
			case "legacy shape":
				statement["predicate"] = map[string]any{"buildDefinition": map[string]any{}, "runDetails": map[string]any{}}
			case "future type":
				statement["predicateType"] = candidateVerificationPredicate + "-future"
			case "subject":
				statement["subject"].([]any)[0].(map[string]any)["name"] = "other.tar.gz"
			case "source":
				predicate["source"].(map[string]any)["digest"] = map[string]any{"gitCommit": strings.Repeat("a", 40)}
			case "manifest":
				predicate["candidateManifest"].(map[string]any)["sha256"] = strings.Repeat("0", 64)
			case "verification", "reproduction":
				predicate[mutation].(map[string]any)["path"] = "candidate/other.json"
			case "missing":
				delete(predicate, "platform")
			case "unknown":
				predicate["builder"] = "invented"
			case "null":
				statement["predicate"] = nil
			}
			data, err := supplyJSON(statement)
			if err != nil {
				t.Fatal(err)
			}
			if mutation == "duplicate" {
				data = bytes.Replace(data, []byte(`"version":`), []byte(`"version":"wrong","version":`), 1)
			}
			if mutation == "trailing" {
				data = append(data, []byte("{}\n")...)
			}
			if err := os.WriteFile(filepath.Join(d, name), data, 0o600); err != nil {
				t.Fatal(err)
			}
			var index supplyIndex
			if err := json.Unmarshal(indexBytes, &index); err != nil {
				t.Fatal(err)
			}
			for i := range index.Targets {
				if index.Targets[i].Provenance.Path == name {
					index.Targets[i].Provenance.SHA256 = supplyDigest(data)
				}
			}
			rebound, err := supplyJSON(index)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(d, supplyIndexName), rebound, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := verifySupply(d, c.Version, c.SourceCommitSHA, manifestHash, supplyDigest(rebound), nil); err == nil {
				t.Fatal("invalid predicate accepted with rebound trust anchor")
			}
		})
	}
}

func TestSupplyFailClosed(t *testing.T) {
	cases := map[string]func(*testing.T, string){
		"missing": func(t *testing.T, d string) {
			if err := os.Remove(filepath.Join(d, sbomName)); err != nil {
				t.Fatal(err)
			}
		},
		"extra":  func(t *testing.T, d string) { writeCandidateFixture(t, d, "extra.json", "{}") },
		"tamper": func(t *testing.T, d string) { writeCandidateFixture(t, d, sbomName, "{}") },
		"swapped provenance": func(t *testing.T, d string) {
			b, _ := os.ReadFile(filepath.Join(d, "provenance-linux-x64.json"))
			if err := os.WriteFile(filepath.Join(d, "provenance-windows-x64.json"), b, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"source drift": func(t *testing.T, d string) {
			writeCandidateFixture(t, filepath.Join(d, "source"), "go.mod", "module other\ngo 1.26.4\n")
		},
		"artifact drift": func(t *testing.T, d string) {
			writeCandidateFixture(t, filepath.Join(d, "candidate"), "flashgate-mcp_0.3.0_linux_x64.tar.gz", "bad")
		},
		"nonregular": func(t *testing.T, d string) {
			p := filepath.Join(d, sbomName)
			if err := os.Remove(p); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(p, 0o700); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			d, m, i, c := supplyFixture(t)
			mutate(t, d)
			if err := verifySupply(d, c.Version, c.SourceCommitSHA, m, i, nil); err == nil {
				t.Fatal("unsafe evidence accepted")
			}
		})
	}
	for _, name := range []string{"schema", "contract", "version", "commit", "manifest", "signing", "matrix", "traversal", "hash", "time", "duplicate", "unknown"} {
		t.Run("rebound index/"+name, func(t *testing.T) {
			d, m, _, c := supplyFixture(t)
			p := filepath.Join(d, supplyIndexName)
			var index supplyIndex
			if err := strictCandidateJSON(p, &index); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "schema":
				index.Schema = "other"
			case "contract":
				index.ContractVersion = 2
			case "version":
				index.Version = "0.4.0"
			case "commit":
				index.SourceCommitSHA = strings.Repeat("a", 40)
			case "manifest":
				index.Candidate.SHA256 = strings.Repeat("0", 64)
			case "signing":
				index.SigningStatus = "signed"
			case "matrix":
				index.Targets = index.Targets[:3]
			case "traversal":
				index.SBOM.Path = "../sbom.spdx.json"
			case "hash":
				index.SBOM.SHA256 = strings.Repeat("0", 64)
			case "time":
				index.SourceCommitTime = "bad"
			}
			data, err := supplyJSON(index)
			if err != nil {
				t.Fatal(err)
			}
			if name == "duplicate" {
				data = bytes.Replace(data, []byte(`"schema":`), []byte(`"schema":"duplicate","schema":`), 1)
			}
			if name == "unknown" {
				data = bytes.Replace(data, []byte("{\n"), []byte("{\n\"unexpected\":true,\n"), 1)
			}
			if err := os.WriteFile(p, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := verifySupply(d, c.Version, c.SourceCommitSHA, m, supplyDigest(data), nil); err == nil {
				t.Fatal("invalid rebound index accepted")
			}
		})
	}
	d, m, i, c := supplyFixture(t)
	for _, args := range [][]string{
		{"--dir", d, "--version", c.Version, "--commit", c.SourceCommitSHA, "--manifest-sha256", m},
		{"--dir", d, "--version", "0.4.0", "--commit", c.SourceCommitSHA, "--manifest-sha256", m, "--index-sha256", i},
		{"--dir", d, "--version", c.Version, "--commit", c.SourceCommitSHA, "--manifest-sha256", m, "--index-sha256", strings.Repeat("0", 64)},
	} {
		if err := runSupply(args, true); err == nil {
			t.Fatal("missing or wrong trust anchor accepted")
		}
	}
	if err := runSupply([]string{"--dir", d, "--version", c.Version, "--commit", c.SourceCommitSHA, "--manifest-sha256", m, "--index-sha256", i}, true); err != nil {
		t.Fatal(err)
	}
	if err := verifySupply(d, c.Version, c.SourceCommitSHA, m, i, []string{"flashgate-releaseaudit"}); err == nil {
		t.Fatal("forbidden host value accepted")
	}
}

func TestSupplyUnsafePaths(t *testing.T) {
	dir := t.TempDir()
	writeCandidateFixture(t, dir, "file", "value")
	root, err := supplyOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, p := range []string{"../file", "/file", `C:\file`, `a\file`, "./file", "a/../file", ".", "file:stream"} {
		if _, err := supplyRead(root, p); err == nil {
			t.Fatal("unsafe path accepted", p)
		}
	}
	if err := os.Symlink(filepath.Join(dir, "file"), filepath.Join(dir, "link")); err != nil {
		t.Skip("symlink creation unavailable on this host")
	}
	if _, err := supplyRead(root, "link"); err == nil {
		t.Fatal("symlink accepted")
	}
	if err := os.Symlink(dir, filepath.Join(dir, "parent")); err != nil {
		t.Fatal(err)
	}
	if _, err := supplyRead(root, "parent/file"); err == nil {
		t.Fatal("symlink parent accepted")
	}
}

func TestSupplyModuleConsistency(t *testing.T) {
	root := supplySourceFixture(t)
	mod, _ := os.ReadFile(filepath.Join(root, "go.mod"))
	sums, _ := os.ReadFile(filepath.Join(root, "go.sum"))
	vendor, _ := os.ReadFile(filepath.Join(root, "vendor/modules.txt"))
	if modules, err := supplyModules(mod, sums, vendor); err != nil || len(modules) != 2 {
		t.Fatal("valid inventory rejected", err)
	}
	for name, mutate := range map[string]func() ([]byte, []byte, []byte){
		"version": func() ([]byte, []byte, []byte) {
			return bytes.ReplaceAll(mod, []byte("v1.7.0"), []byte("v1.8.0")), sums, vendor
		},
		"replacement": func() ([]byte, []byte, []byte) {
			return append(bytes.Clone(mod), []byte("\nreplace x => y\n")...), sums, vendor
		},
		"missing sum": func() ([]byte, []byte, []byte) { return mod, nil, vendor },
		"invalid h1": func() ([]byte, []byte, []byte) {
			return mod, bytes.ReplaceAll(sums, []byte("h1:"), []byte("sha256:")), vendor
		},
		"duplicate sum": func() ([]byte, []byte, []byte) { return mod, append(bytes.Clone(sums), sums...), vendor },
		"vendor version": func() ([]byte, []byte, []byte) {
			return mod, sums, bytes.ReplaceAll(vendor, []byte("v1.7.0"), []byte("v1.8.0"))
		},
		"vendor explicit": func() ([]byte, []byte, []byte) {
			return mod, sums, bytes.ReplaceAll(vendor, []byte("explicit;"), []byte("implicit;"))
		},
		"vendor missing": func() ([]byte, []byte, []byte) { return mod, sums, nil },
	} {
		t.Run(name, func(t *testing.T) {
			a, b, c := mutate()
			if _, err := supplyModules(a, b, c); err == nil {
				t.Fatal("inconsistent module inventory accepted")
			}
		})
	}
}

func TestSupplyRollbackSmoke(t *testing.T) {
	dir, p, manifest := candidateFixture(t)
	root := t.TempDir()
	if err := promoteCandidate(dir, root, p, manifest, true); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "current"))
	identity := strings.TrimSpace(string(before))
	// Simulate selecting another candidate, then return to the previously
	// verified identity via the existing promotion path, without rebuilding.
	writeCandidateFixture(t, root, "current", "synthetic-other-selection\n")
	immutable := filepath.Join(root, identity)
	if err := runCandidatePromote([]string{"--dir", immutable, "--manifest", filepath.Join(immutable, "candidate-manifest.json"), "--destination", root, "--version", manifest.Version, "--commit", manifest.SourceCommitSHA, "--current"}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "current"))
	if !bytes.Equal(before, after) {
		t.Fatal("rollback alias parity differs")
	}
	if err := sameCandidateFiles(dir, immutable, p, manifest); err != nil {
		t.Fatal(err)
	}
	writeCandidateFixture(t, immutable, manifest.Records[0].Archive, "tampered")
	if err := promoteCandidate(immutable, root, filepath.Join(immutable, "candidate-manifest.json"), manifest, true); err == nil {
		t.Fatal("rollback selected invalid prior candidate")
	}
	after, _ = os.ReadFile(filepath.Join(root, "current"))
	if !bytes.Equal(before, after) {
		t.Fatal("failed rollback changed current")
	}
}

func TestSupplyGenerationCommandSourceBinding(t *testing.T) {
	dir, p, manifest := candidateFixture(t)
	// Use a task-local committed fixture; the real working candidate can have
	// modified inventory documentation and must not masquerade as HEAD bytes.
	repo := supplySourceFixture(t)
	fixtureGit := func(args ...string) []byte {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", repo}, args...)...)
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull)
		data, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git: %v: %s", err, data)
		}
		return data
	}
	fixtureGit("init", "--quiet")
	fixtureGit("add", "--", ".")
	fixtureGit("-c", "user.name=Release audit fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "Supply source fixture")
	commit, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	manifest.SourceCommitSHA = strings.TrimSpace(string(commit))
	for i := range manifest.Records {
		r := &manifest.Records[i]
		r.SourceCommitSHA = manifest.SourceCommitSHA
		evidence, err := os.ReadFile(filepath.Join(dir, r.VerificationEvidence))
		if err != nil {
			t.Fatal(err)
		}
		r.VerificationSHA256 = writeCandidateFixture(t, dir, r.VerificationEvidence, strings.ReplaceAll(string(evidence), testCandidateCommit, manifest.SourceCommitSHA))
		name := filepath.Join(dir, "record-"+r.Platform+"-"+r.Architecture+".json")
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
		if err := writeJSON(name, *r); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(p, manifest); err != nil {
		t.Fatal(err)
	}
	m, err := hashFile(p)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "evidence")
	args := []string{"--root", repo, "--dir", dir, "--output", out, "--version", manifest.Version, "--commit", manifest.SourceCommitSHA, "--manifest-sha256", m}
	if err := runSupply(args, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range supplySourceNames {
		expected, err := exec.Command("git", "-C", repo, "show", manifest.SourceCommitSHA+":"+name).Output()
		if err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(filepath.Join(out, "source", filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatal("committed byte parity", name, err)
		}
	}
	if err := runSupply(args, false); err == nil {
		t.Fatal("command accepted immutable collision")
	}
	args[1] = supplySourceFixture(t)
	args[5] = filepath.Join(t.TempDir(), "rejected")
	if err := runSupply(args, false); err == nil {
		t.Fatal("unbound source checkout accepted")
	}
}

func TestSupplyBoundedReadsAndStreamCopy(t *testing.T) {
	dir := t.TempDir()
	data := bytes.Repeat([]byte("a"), 64<<10)
	writeCandidateFixture(t, dir, "archive.zip", string(data))
	root, err := supplyOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	file, err := supplyRegularOpen(root, "archive.zip", int64(len(data)-1))
	if err == nil {
		file.Close()
		t.Fatal("oversized file opened")
	}
	out := filepath.Join(t.TempDir(), "copy.zip")
	if err := supplyCopy(root, "archive.zip", out, int64(len(data))); err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(out)
	if err != nil || !bytes.Equal(data, copied) {
		t.Fatal("streamed byte parity", err)
	}
	if err := supplyCopy(root, "archive.zip", out, int64(len(data))); err == nil {
		t.Fatal("copy overwrote existing destination")
	}
	writeCandidateFixture(t, dir, "metadata.json", string(bytes.Repeat([]byte("x"), maximumSupplyMetadataSize+1)))
	if _, err := supplyRead(root, "metadata.json"); err == nil {
		t.Fatal("oversized metadata read")
	}
	if err := os.Rename(filepath.Join(dir, "archive.zip"), filepath.Join(dir, "renamed.zip")); err != nil {
		t.Fatal("stream copy retained source handle", err)
	}
}

func TestSupplyGenerationFailureVisibility(t *testing.T) {
	for _, failure := range []string{"incomplete matrix", "source mismatch", "leak", "bad time", "nonregular candidate"} {
		t.Run(failure, func(t *testing.T) {
			dir, _, manifest := candidateFixture(t)
			source := supplySourceFixture(t)
			parent := t.TempDir()
			out := filepath.Join(parent, "evidence")
			timestamp := supplyTestTime
			var forbidden []string
			switch failure {
			case "incomplete matrix":
				manifest.Records = manifest.Records[:3]
			case "source mismatch":
				writeCandidateFixture(t, source, "VERSION", "0.4.0\n")
			case "leak":
				forbidden = []string{"flashgate-releaseaudit"}
			case "bad time":
				timestamp = "not-a-time"
			case "nonregular candidate":
				p := filepath.Join(dir, manifest.Records[0].Archive)
				if err := os.Remove(p); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(p, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := generateSupply(source, dir, out, timestamp, manifest, nil, forbidden); err == nil {
				t.Fatal("failed evidence became visible")
			}
			entries, err := os.ReadDir(parent)
			if err != nil || len(entries) != 0 {
				t.Fatal("incomplete output/stage remains", entries, err)
			}
		})
	}
}

func TestSupplyRejectsCandidateOutputOverlap(t *testing.T) {
	dir, p, manifest := candidateFixture(t)
	source := supplySourceFixture(t)
	before, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{dir, filepath.Join(dir, "evidence"), filepath.Dir(dir)} {
		if err := generateSupply(source, dir, out, supplyTestTime, manifest, nil, nil); err == nil {
			t.Fatal("candidate overlap accepted", out)
		}
	}
	after, err := os.ReadFile(p)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("candidate manifest changed", err)
	}
	if err := verifyCandidateManifest(dir, manifest); err != nil {
		t.Fatal(err)
	}
}
