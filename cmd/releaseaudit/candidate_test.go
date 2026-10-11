package main

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidatePromotionRejectsNonregularCurrentBeforeVisibility(t *testing.T) {
	dir, path, manifest := candidateFixture(t)
	root := t.TempDir()
	alias := filepath.Join(root, "current")
	if err := os.Mkdir(alias, 0o700); err != nil {
		t.Fatal(err)
	}
	writeCandidateFixture(t, alias, "retained", "original current directory")
	if err := promoteCandidate(dir, root, path, manifest, true); err == nil {
		t.Fatal("nonregular current accepted")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "current" || !entries[0].IsDir() {
		t.Fatalf("failed promotion changed destination: %v %v", entries, err)
	}
	data, err := os.ReadFile(filepath.Join(alias, "retained"))
	if err != nil || string(data) != "original current directory" {
		t.Fatal("nonregular current prestate changed", err)
	}
}

func TestCandidateCurrentFailureRollsBackOnlyNewIdentity(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, initialAlias := range []bool{false, true} {
			for _, failure := range []string{"install", "backup move", "backup cleanup", "lock cleanup", "temp cleanup", "restore"} {
				if !initialAlias && (failure == "backup move" || failure == "backup cleanup" || failure == "restore") {
					continue
				}
				t.Run(fmt.Sprintf("existing=%v/alias=%v/%s", existing, initialAlias, failure), func(t *testing.T) {
					dir, path, manifest := candidateFixture(t)
					root := t.TempDir()
					if existing {
						if err := promoteCandidate(dir, root, path, manifest, false); err != nil {
							t.Fatal(err)
						}
					}
					alias := filepath.Join(root, "current")
					if initialAlias {
						writeCandidateFixture(t, root, "current", "original identity\n")
					}
					injected := errors.New("injected alias operation failure")
					backupRemoves := 0
					ops := candidateAliasOps{
						rename: func(from, to string) error {
							base := filepath.Base(from)
							if (failure == "install" || failure == "temp cleanup" || failure == "restore") && strings.HasPrefix(base, ".current-") && !strings.HasPrefix(base, ".current-backup-") && to == alias {
								return injected
							}
							if failure == "backup move" && from == alias {
								return injected
							}
							if failure == "restore" && strings.HasPrefix(base, ".current-backup-") && to == alias {
								return injected
							}
							return os.Rename(from, to)
						},
						remove: func(name string) error {
							base := filepath.Base(name)
							if strings.HasPrefix(base, ".current-backup-") {
								backupRemoves++
								if failure == "backup cleanup" && backupRemoves == 2 {
									return injected
								}
							}
							if failure == "lock cleanup" && base == ".candidate-promotion.lock" {
								return injected
							}
							if failure == "temp cleanup" && strings.HasPrefix(base, ".current-") && !strings.HasPrefix(base, ".current-backup-") {
								return injected
							}
							return os.Remove(name)
						},
					}
					if err := promoteCandidateWithOps(dir, root, path, manifest, true, ops); !errors.Is(err, injected) {
						t.Fatalf("expected injected failure, got %v", err)
					}
					data, err := os.ReadFile(alias)
					if initialAlias {
						if err != nil || string(data) != "original identity\n" {
							t.Fatalf("initial alias not restored: %q %v", data, err)
						}
					} else if !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("initial alias absence not restored: %v", err)
					}
					entries, err := os.ReadDir(root)
					if err != nil {
						t.Fatal(err)
					}
					want := 0
					if initialAlias {
						want++
					}
					if existing {
						want++
					}
					if len(entries) != want {
						t.Fatalf("rollback left unexpected state: %v", entries)
					}
					for _, entry := range entries {
						if entry.Name() == "current" {
							continue
						}
						if !existing || !entry.IsDir() {
							t.Fatal("new identity or temporary artifact survived", entry.Name())
						}
						if err := sameCandidateFiles(dir, filepath.Join(root, entry.Name()), path, manifest); err != nil {
							t.Fatal("preexisting immutable identity changed", err)
						}
					}
				})
			}
		}
	}
}

const testCandidateCommit = "0123456789abcdef0123456789abcdef01234567"

func TestCandidateVerifierHashBinding(t *testing.T) {
	for index, target := range candidateTargets {
		t.Run(target.Platform+"-"+target.Architecture, func(t *testing.T) {
			dir, _, manifest := candidateFixture(t)
			record := manifest.Records[index]
			contents, err := os.ReadFile(filepath.Join(dir, record.VerificationEvidence))
			if err != nil {
				t.Fatal(err)
			}
			valid := string(contents)
			for _, text := range []string{valid, strings.ReplaceAll(valid, "\n", "\r\n"),
				strings.ReplaceAll(valid, "Sha256: ", "Sha256       : ")} {
				if err := validateVerificationEvidence([]byte(text), record); err != nil {
					t.Fatal("valid platform output rejected:", err)
				}
			}
			line := "Sha256: " + record.ArtifactSHA256 + "\n"
			for name, invalid := range map[string]string{
				"missing":          strings.Replace(valid, line, "", 1),
				"empty":            strings.Replace(valid, line, "Sha256: \n", 1),
				"malformed":        strings.Replace(valid, record.ArtifactSHA256, strings.Repeat("g", 64), 1),
				"short":            strings.Replace(valid, record.ArtifactSHA256, record.ArtifactSHA256[:63], 1),
				"long":             strings.Replace(valid, record.ArtifactSHA256, record.ArtifactSHA256+"0", 1),
				"uppercase":        strings.Replace(valid, record.ArtifactSHA256, strings.ToUpper(record.ArtifactSHA256), 1),
				"wrong archive":    strings.Replace(valid, record.ArtifactSHA256, strings.Repeat("0", 64), 1),
				"duplicate":        valid + line,
				"wrong then valid": strings.Replace(valid, line, "Sha256: "+strings.Repeat("0", 64)+"\n"+line, 1),
				"valid then wrong": valid + "Sha256: " + strings.Repeat("0", 64) + "\n",
				"case alias":       valid + "sha256: " + record.ArtifactSHA256 + "\n",
				"field case":       strings.Replace(valid, "Sha256:", "SHA256:", 1),
				"status":           strings.Replace(valid, "Status: PASS", "Status: FAIL", 1),
				"version":          strings.Replace(valid, "Version: "+record.Version, "Version: 0.4.0", 1),
				"architecture":     strings.Replace(valid, "PublicArch: "+record.Architecture, "PublicArch: other", 1),
				"commit":           strings.Replace(valid, record.SourceCommitSHA, strings.Repeat("a", 40), 1),
				"archive path":     strings.Replace(valid, "ArchivePath: "+record.Archive, "ArchivePath: other.tar.gz", 1),
				"checksum path":    strings.Replace(valid, "ChecksumPath: "+record.Checksum, "ChecksumPath: other.sha256", 1),
				"errors":           strings.Replace(valid, "ErrorCount: 0", "ErrorCount: 1", 1),
			} {
				t.Run(name, func(t *testing.T) {
					// Rebind the report file so rejection proves semantic validation,
					// rather than merely detecting a changed evidence-file hash.
					bad := record
					bad.VerificationSHA256 = writeCandidateFixture(t, dir, bad.VerificationEvidence, invalid)
					if err := verifyCandidateRecord(dir, bad); err == nil {
						t.Fatal("unbound or invalid verifier accepted despite valid other evidence")
					}
				})
			}
		})
	}
}

func TestCandidateRejectsCrossArchiveVerifier(t *testing.T) {
	for index, target := range candidateTargets {
		t.Run(target.Platform+"-"+target.Architecture, func(t *testing.T) {
			dir, path, manifest := candidateFixture(t)
			record := &manifest.Records[index]
			// Two real archives share the exact candidate filename and identity.
			// Only their bytes differ; retain verifier evidence for archive A.
			writeArchive := func(name string, second bool) string {
				if target.Platform == "windows" {
					payload := "archive A"
					if second {
						payload = "archive B"
					}
					writeZIPFixture(t, name, []byte(payload))
				} else {
					entry := ""
					if second {
						entry = "root/other"
					}
					writeTarFixture(t, name, tar.TypeReg, entry, tar.TypeReg)
				}
				hash, err := hashFile(name)
				if err != nil {
					t.Fatal(err)
				}
				return hash
			}
			archivePath := filepath.Join(dir, record.Archive)
			originalHash := record.ArtifactSHA256
			hashA := writeArchive(archivePath, false)
			evidence, err := os.ReadFile(filepath.Join(dir, record.VerificationEvidence))
			if err != nil {
				t.Fatal(err)
			}
			record.VerificationSHA256 = writeCandidateFixture(t, dir, record.VerificationEvidence,
				strings.Replace(string(evidence), record.ArtifactSHA256, hashA, 1))
			record.ArtifactSHA256 = writeArchive(archivePath, true)
			if hashA == record.ArtifactSHA256 {
				t.Fatal("fixture archives must differ")
			}
			record.ChecksumSHA256 = writeCandidateFixture(t, dir, record.Checksum, record.ArtifactSHA256+"  "+record.Archive+"\n")
			record.CompareSHA256 = writeCandidateFixture(t, dir, record.CompareReport,
				fmt.Sprintf(`{"schema":"flashgate-release-reproducibility/v1","status":"PASS","archiveSha256":"%s","checksumSha256":"%s","binarySha256":"%s","inventoryCount":5,"errors":[]}`, record.ArtifactSHA256, record.ChecksumSHA256, record.ArtifactSHA256))
			// Every other record/file binding, including the leak PASS, is valid.
			recordPath := filepath.Join(dir, "record-"+target.Platform+"-"+target.Architecture+".json")
			if err := os.Remove(recordPath); err != nil {
				t.Fatal(err)
			}
			if err := writeJSON(recordPath, *record); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := writeJSON(path, manifest); err != nil {
				t.Fatal(err)
			}
			for name, verify := range map[string]func() error{
				"record":   func() error { return verifyCandidateRecord(dir, *record) },
				"manifest": func() error { return verifyCandidateManifest(dir, manifest) },
				"promotion": func() error {
					return promoteCandidate(dir, filepath.Join(t.TempDir(), "promotion"), path, manifest, false)
				},
			} {
				t.Run(name, func(t *testing.T) {
					if err := verify(); err == nil || !strings.Contains(err.Error(), "verifier SHA-256 differs") {
						t.Fatalf("cross-archive evidence did not fail at verifier binding: %v", err)
					}
				})
			}
			// Replacing only the verifier binding with B restores full acceptance.
			record.VerificationSHA256 = writeCandidateFixture(t, dir, record.VerificationEvidence,
				strings.Replace(string(evidence), originalHash, record.ArtifactSHA256, 1))
			if err := verifyCandidateRecord(dir, *record); err != nil {
				t.Fatal("control candidate B should pass:", err)
			}
		})
	}
}

func writeCandidateFixture(t *testing.T, dir, name, contents string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(contents))
	return hex.EncodeToString(digest[:])
}

func candidateFixture(t *testing.T) (string, string, candidateManifest) {
	t.Helper()
	dir := t.TempDir()
	manifest := candidateManifest{Schema: candidateSchema, Version: "0.3.0", SourceCommitSHA: testCandidateCommit}
	for _, target := range candidateTargets {
		archive, checksum, evidence, compare, leak := candidateNames(manifest.Version, target)
		artifactHash := writeCandidateFixture(t, dir, archive, "verified candidate bytes: "+archive)
		checksumHash := writeCandidateFixture(t, dir, checksum, artifactHash+"  "+archive+"\n")
		evidenceHash := writeCandidateFixture(t, dir, evidence,
			fmt.Sprintf("Status: PASS\nArchivePath: %s\nChecksumPath: %s\nVersion: %s\nPublicArch: %s\nSourceCommit: %s\nSha256: %s\nErrorCount: 0\n", archive, checksum, manifest.Version, target.Architecture, manifest.SourceCommitSHA, artifactHash))
		compareHash := writeCandidateFixture(t, dir, compare,
			fmt.Sprintf(`{"schema":"flashgate-release-reproducibility/v1","status":"PASS","archiveSha256":"%s","checksumSha256":"%s","binarySha256":"%s","inventoryCount":5,"errors":[]}`, artifactHash, checksumHash, artifactHash))
		leakHash := writeCandidateFixture(t, dir, leak,
			fmt.Sprintf(`{"schema":"flashgate-release-leak-scan/v1","status":"PASS","artifact":"%s","scannedEntries":5,"findings":[],"allowedMarkers":[],"errors":[]}`, archive))
		record := candidateRecord{Schema: recordSchema, Version: manifest.Version, SourceCommitSHA: manifest.SourceCommitSHA,
			Platform: target.Platform, Architecture: target.Architecture, Archive: archive, Checksum: checksum,
			ArtifactSHA256: artifactHash, ChecksumSHA256: checksumHash, VerificationEvidence: evidence,
			VerificationSHA256: evidenceHash, CompareReport: compare, CompareSHA256: compareHash, LeakReport: leak, LeakSHA256: leakHash}
		if err := writeJSON(filepath.Join(dir, "record-"+target.Platform+"-"+target.Architecture+".json"), record); err != nil {
			t.Fatal(err)
		}
		manifest.Records = append(manifest.Records, record)
	}
	path := filepath.Join(dir, "candidate-manifest.json")
	if err := writeJSON(path, manifest); err != nil {
		t.Fatal(err)
	}
	return dir, path, manifest
}

func TestCandidateManifestAndExactBytePromotion(t *testing.T) {
	dir, path, manifest := candidateFixture(t)
	if err := verifyCandidateManifest(dir, manifest); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "promotion")
	if err := promoteCandidate(dir, root, path, manifest, true); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected immutable directory and current alias, got %d", len(entries))
	}
	alias, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	identity := strings.TrimSpace(string(alias))
	if identity == "" || !strings.HasPrefix(identity, manifest.Version+"_"+manifest.SourceCommitSHA+"_") {
		t.Fatal("wrong current alias")
	}
	if err := sameCandidateFiles(dir, filepath.Join(root, identity), path, manifest); err != nil {
		t.Fatal(err)
	}
	if err := promoteCandidate(dir, root, path, manifest, false); err != nil {
		t.Fatal("identical promotion is not idempotent:", err)
	}
	if err := promoteCandidate(dir, root, path, manifest, true); err != nil {
		t.Fatal("current alias could not be updated:", err)
	}
	if err := sameCandidateFiles(dir, filepath.Join(root, identity), path, manifest); err != nil {
		t.Fatal("mutable current update changed immutable bytes:", err)
	}
	secondDir, secondPath, second := candidateFixture(t)
	second.SourceCommitSHA = strings.Repeat("a", 40)
	for index := range second.Records {
		record := &second.Records[index]
		record.SourceCommitSHA = second.SourceCommitSHA
		evidencePath := filepath.Join(secondDir, record.VerificationEvidence)
		contents, err := os.ReadFile(evidencePath)
		if err != nil {
			t.Fatal(err)
		}
		record.VerificationSHA256 = writeCandidateFixture(t, secondDir, record.VerificationEvidence,
			strings.ReplaceAll(string(contents), testCandidateCommit, second.SourceCommitSHA))
		recordPath := filepath.Join(secondDir, "record-"+record.Platform+"-"+record.Architecture+".json")
		if err := os.Remove(recordPath); err != nil {
			t.Fatal(err)
		}
		if err := writeJSON(recordPath, *record); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(secondPath); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(secondPath, second); err != nil {
		t.Fatal(err)
	}
	if err := verifyCandidateManifest(secondDir, second); err != nil {
		t.Fatal(err)
	}
	if err := promoteCandidate(secondDir, root, secondPath, second, true); err != nil {
		t.Fatal(err)
	}
	newAlias, err := os.ReadFile(filepath.Join(root, "current"))
	if err != nil {
		t.Fatal(err)
	}
	newIdentity := strings.TrimSpace(string(newAlias))
	if newIdentity == identity || !strings.Contains(newIdentity, second.SourceCommitSHA) {
		t.Fatal("current alias did not select new identity")
	}
	if err := sameCandidateFiles(dir, filepath.Join(root, identity), path, manifest); err != nil {
		t.Fatal("old immutable identity changed:", err)
	}
	if err := sameCandidateFiles(secondDir, filepath.Join(root, newIdentity), secondPath, second); err != nil {
		t.Fatal("new immutable identity differs:", err)
	}
	name := manifest.Records[0].Archive
	if err := os.WriteFile(filepath.Join(root, identity, name), []byte("different bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := promoteCandidate(dir, root, path, manifest, false); err == nil {
		t.Fatal("different bytes under immutable identity accepted")
	}
}

func TestCandidateCommands(t *testing.T) {
	dir, manifestPath, manifest := candidateFixture(t)
	writeCandidateFixture(t, dir, "VERSION", manifest.Version+"\n")
	first := manifest.Records[0]
	recordPath := filepath.Join(dir, "record-"+first.Platform+"-"+first.Architecture+".json")
	if err := os.Remove(recordPath); err != nil {
		t.Fatal(err)
	}
	if err := runCandidateRecord([]string{"--root", dir, "--dir", dir, "--version", manifest.Version, "--commit", manifest.SourceCommitSHA,
		"--platform", first.Platform, "--architecture", first.Architecture, "--output", recordPath}); err != nil {
		t.Fatal(err)
	}
	if err := runCandidateRecord([]string{"--root", dir, "--dir", dir, "--version", "0.4.0", "--commit", manifest.SourceCommitSHA,
		"--platform", first.Platform, "--architecture", first.Architecture, "--output", filepath.Join(dir, "wrong-record.json")}); err == nil {
		t.Fatal("record accepted a free version differing from root VERSION")
	}
	if err := os.Remove(manifestPath); err != nil {
		t.Fatal(err)
	}
	if err := runCandidateManifest([]string{"--root", dir, "--dir", dir, "--version", manifest.Version, "--commit", manifest.SourceCommitSHA, "--output", manifestPath}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "promotion")
	if err := runCandidatePromote([]string{"--dir", dir, "--manifest", manifestPath, "--destination", root,
		"--version", manifest.Version, "--commit", manifest.SourceCommitSHA, "--current"}); err != nil {
		t.Fatal(err)
	}
	if err := runCandidatePromote([]string{"--dir", dir, "--manifest", manifestPath, "--destination", root,
		"--version", "0.4.0", "--commit", manifest.SourceCommitSHA}); err == nil {
		t.Fatal("wrong VERSION accepted")
	}
	if err := runCandidatePromote([]string{"--dir", dir, "--manifest", manifestPath, "--destination", root,
		"--version", manifest.Version, "--commit", strings.Repeat("a", 40)}); err == nil {
		t.Fatal("wrong commit accepted")
	}
}

func TestCandidateManifestRejectsMutations(t *testing.T) {
	for name, mutate := range map[string]func(string, *candidateManifest){
		"version":        func(_ string, m *candidateManifest) { m.Version = "0.4.0" },
		"commit":         func(_ string, m *candidateManifest) { m.SourceCommitSHA = strings.Repeat("a", 40) },
		"incomplete":     func(_ string, m *candidateManifest) { m.Records = m.Records[:3] },
		"duplicate":      func(_ string, m *candidateManifest) { m.Records[1] = m.Records[0] },
		"unknown target": func(_ string, m *candidateManifest) { m.Records[0].Platform = "darwin" },
		"archive name":   func(_ string, m *candidateManifest) { m.Records[0].Archive = "wrong.zip" },
		"checksum name":  func(_ string, m *candidateManifest) { m.Records[0].Checksum = "wrong.sha256" },
		"artifact hash":  func(_ string, m *candidateManifest) { m.Records[0].ArtifactSHA256 = strings.Repeat("0", 64) },
		"no verification": func(dir string, m *candidateManifest) {
			name := m.Records[0].VerificationEvidence
			m.Records[0].VerificationSHA256 = writeCandidateFixture(t, dir, name, "Status: FAIL\n")
		},
		"verifier commit": func(dir string, m *candidateManifest) {
			name := m.Records[0].VerificationEvidence
			m.Records[0].VerificationSHA256 = writeCandidateFixture(t, dir, name,
				"Status: PASS\nVersion: 0.3.0\nPublicArch: x64\nSourceCommit: "+strings.Repeat("a", 40)+"\nErrorCount: 0\n")
		},
		"bad checksum": func(dir string, m *candidateManifest) {
			name := m.Records[0].Checksum
			m.Records[0].ChecksumSHA256 = writeCandidateFixture(t, dir, name, "bad checksum\n")
		},
		"repro archive binding": func(dir string, m *candidateManifest) {
			r := &m.Records[0]
			r.CompareSHA256 = writeCandidateFixture(t, dir, r.CompareReport,
				fmt.Sprintf(`{"schema":"flashgate-release-reproducibility/v1","status":"PASS","archiveSha256":"%s","checksumSha256":"%s","binarySha256":"%s","inventoryCount":5,"errors":[]}`, strings.Repeat("0", 64), r.ChecksumSHA256, r.ArtifactSHA256))
		},
		"repro checksum binding": func(dir string, m *candidateManifest) {
			r := &m.Records[0]
			r.CompareSHA256 = writeCandidateFixture(t, dir, r.CompareReport,
				fmt.Sprintf(`{"schema":"flashgate-release-reproducibility/v1","status":"PASS","archiveSha256":"%s","checksumSha256":"%s","binarySha256":"%s","inventoryCount":5,"errors":[]}`, r.ArtifactSHA256, strings.Repeat("0", 64), r.ArtifactSHA256))
		},
		"leak archive binding": func(dir string, m *candidateManifest) {
			r := &m.Records[0]
			r.LeakSHA256 = writeCandidateFixture(t, dir, r.LeakReport,
				`{"schema":"flashgate-release-leak-scan/v1","status":"PASS","artifact":"other.zip","scannedEntries":5,"findings":[],"allowedMarkers":[],"errors":[]}`)
		},
		"rebuild bytes": func(dir string, _ *candidateManifest) {
			writeCandidateFixture(t, dir, "flashgate-mcp_0.3.0_windows_x64.zip", "rebuilt")
		},
		"missing compare": func(dir string, _ *candidateManifest) {
			os.Remove(filepath.Join(dir, "repro-windows-x64.json"))
		},
		"missing scan": func(dir string, _ *candidateManifest) {
			os.Remove(filepath.Join(dir, "leak-windows-x64.json"))
		},
		"record file drift": func(dir string, _ *candidateManifest) {
			writeCandidateFixture(t, dir, "record-windows-x64.json", "{}")
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir, _, manifest := candidateFixture(t)
			mutate(dir, &manifest)
			if err := verifyCandidateManifest(dir, manifest); err == nil {
				t.Fatal("mutated candidate accepted")
			}
		})
	}
}

func TestCandidateStrictManifestAndNoPartialPromotion(t *testing.T) {
	dir, path, manifest := candidateFixture(t)
	if err := os.WriteFile(path, []byte(`{"schema":"flashgate-current-candidate/v1","unknown":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var parsed candidateManifest
	if err := strictCandidateJSON(path, &parsed); err == nil {
		t.Fatal("unknown manifest field accepted")
	}
	duplicate := []byte(`{"schema":"flashgate-current-candidate/v1","schema":"flashgate-current-candidate/v1"}`)
	if err := os.WriteFile(path, duplicate, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := strictCandidateJSON(path, &parsed); err == nil {
		t.Fatal("duplicate JSON key accepted")
	}
	root := filepath.Join(t.TempDir(), "promotion")
	if err := os.Remove(filepath.Join(dir, manifest.Records[3].Archive)); err != nil {
		t.Fatal(err)
	}
	if err := verifyCandidateManifest(dir, manifest); err == nil {
		t.Fatal("missing source artifact accepted")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("failed verification left promotion state")
	}
	dir2, path2, manifest2 := candidateFixture(t)
	if err := os.Remove(filepath.Join(dir2, "record-linux-arm64.json")); err != nil {
		t.Fatal(err)
	}
	if err := promoteCandidate(dir2, root, path2, manifest2, false); err == nil {
		t.Fatal("partial copy unexpectedly succeeded")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed promotion left %d visible entries", len(entries))
	}
	dir3, path3, manifest3 := candidateFixture(t)
	changed := manifest3
	changed.SourceCommitSHA = strings.Repeat("b", 40)
	if err := os.Remove(path3); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(path3, changed); err != nil {
		t.Fatal(err)
	}
	root3 := filepath.Join(t.TempDir(), "changed-manifest")
	if err := promoteCandidate(dir3, root3, path3, manifest3, false); err == nil {
		t.Fatal("manifest changed before promotion was accepted")
	}
	if _, err := os.Stat(root3); !os.IsNotExist(err) {
		t.Fatal("manifest drift created promotion state")
	}
}
