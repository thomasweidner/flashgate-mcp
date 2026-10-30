package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestPublicationFiveAssetProjectionAndSelectedPlatform(t *testing.T) {
	dir, plan := publicationFixture(t)
	if len(plan.Assets) != 5 {
		t.Fatal("not exactly five public assets")
	}
	var archives []publicationAsset
	for _, a := range plan.Assets {
		if a.Name != publicationChecksumsName {
			archives = append(archives, a)
		}
		if strings.HasSuffix(a.Name, ".sha256") || strings.HasSuffix(a.Name, ".json") {
			t.Fatal("evidence projected publicly", a.Name)
		}
	}
	if len(archives) != 4 {
		t.Fatal("missing archives")
	}
	sort.Slice(archives, func(i, j int) bool { return archives[i].Name < archives[j].Name })
	var expected strings.Builder
	for _, a := range archives {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(a.Path)))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&expected, "%s  %s\n", supplyDigest(b), a.Name)
	}
	root, err := supplyOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	sums := plan.Assets[0]
	if sums.Name != publicationChecksumsName || sums.Path != "" {
		t.Fatal("generated asset identity", sums)
	}
	r, err := publicationAssetReader(root, sums, plan.Assets)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(r)
	errClose := r.Close()
	if err != nil || errClose != nil || string(actual) != expected.String() || int64(len(actual)) != sums.Size || supplyDigest(actual) != sums.SHA256 {
		t.Fatal("noncanonical checksum bytes", err, errClose)
	}
	if _, err := os.Stat(filepath.Join(dir, publicationChecksumsName)); !os.IsNotExist(err) {
		t.Fatal("evidence bundle mutated")
	}
	for _, target := range candidateTargets {
		t.Run(target.Platform+"-"+target.Architecture, func(t *testing.T) {
			consumer := t.TempDir()
			name, _, _, _, _ := candidateNames(plan.Authorization.Version, target)
			archive, err := os.ReadFile(filepath.Join(dir, "candidate", name))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(consumer, name), archive, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(consumer, publicationChecksumsName), actual, 0600); err != nil {
				t.Fatal(err)
			}
			if err := runPublicationVerify([]string{"--dir", consumer, "--version", plan.Authorization.Version, "--platform", target.Platform, "--architecture", target.Architecture}); err != nil {
				t.Fatal(err)
			}
			files, _ := os.ReadDir(consumer)
			if len(files) != 2 {
				t.Fatal("consumer needed more than two files")
			}
			if err := os.WriteFile(filepath.Join(consumer, name), append(archive, 1), 0600); err != nil {
				t.Fatal(err)
			}
			if err := verifySelectedPublication(consumer, plan.Authorization.Version, target.Platform, target.Architecture); err == nil {
				t.Fatal("tampered archive accepted")
			}
		})
	}
	if err := verifySupply(dir, plan.Authorization.Version, plan.Authorization.SourceCommit, plan.Authorization.ManifestSHA256, plan.Authorization.IndexSHA256, nil); err != nil {
		t.Fatal("internal verification weakened", err)
	}
}

func TestPublicationChecksumRecordNegatives(t *testing.T) {
	dir, plan := publicationFixture(t)
	sums, err := publicationGeneratedChecksums(plan.Assets)
	if err != nil {
		t.Fatal(err)
	}
	target := candidateTargets[0]
	name, _, _, _, _ := candidateNames(plan.Authorization.Version, target)
	archive, err := os.ReadFile(filepath.Join(dir, "candidate", name))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(sums), "\n")
	mutations := map[string][]byte{
		"missing":     []byte(strings.Join(lines[1:], "\n")),
		"duplicate":   []byte(strings.Replace(string(sums), lines[1], lines[0], 1)),
		"additional":  append(append([]byte{}, sums...), []byte(lines[0]+"\n")...),
		"order":       []byte(strings.Join([]string{lines[1], lines[0], lines[2], lines[3], ""}, "\n")),
		"bom":         append([]byte{0xef, 0xbb, 0xbf}, sums...),
		"crlf":        bytes.ReplaceAll(sums, []byte("\n"), []byte("\r\n")),
		"no-final-lf": bytes.TrimSuffix(sums, []byte("\n")),
		"blank":       append(append([]byte{}, sums...), '\n'),
		"unsafe":      bytes.Replace(sums, []byte(name), []byte("../"+name), 1),
		"alias":       bytes.Replace(sums, []byte(name), []byte("./"+name), 1),
		"upper-hash":  []byte(strings.ToUpper(string(sums[:64])) + string(sums[64:])),
		"bad-hash":    append([]byte("g"), sums[1:]...),
		"tab":         bytes.Replace(sums, []byte("  "), []byte("\t"), 1),
	}
	for label, data := range mutations {
		t.Run(label, func(t *testing.T) {
			consumer := t.TempDir()
			if err := os.WriteFile(filepath.Join(consumer, name), archive, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(consumer, publicationChecksumsName), data, 0600); err != nil {
				t.Fatal(err)
			}
			if err := verifySelectedPublication(consumer, plan.Authorization.Version, target.Platform, target.Architecture); err == nil {
				t.Fatal("malformed checksum accepted")
			}
		})
	}
}

func TestPublicationMandatoryGitHubDigest(t *testing.T) {
	plan := publicationUnitPlan(t)
	asset := plan.Assets[0]
	for _, digest := range []string{"", "SHA256:" + asset.SHA256, "sha256:" + strings.ToUpper(asset.SHA256), "sha256:" + strings.Repeat("0", 64), asset.SHA256} {
		t.Run(digest, func(t *testing.T) {
			m := newPublicationMock(plan)
			_, _ = m.CreateDraft(context.Background(), plan)
			m.data[1] = []byte("untrusted")
			m.state.Release.Assets = []remotePublicationAsset{{ID: 1, Name: asset.Name, Size: asset.Size, State: "uploaded", Digest: digest}}
			if err := publicationReadback(context.Background(), m, plan, 789, true, []publicationAsset{asset}); err == nil {
				t.Fatal("invalid digest accepted")
			}
		})
	}
	// Even a matching provider digest never substitutes for the original bytes.
	m := newPublicationMock(plan)
	_, _ = m.CreateDraft(context.Background(), plan)
	m.data[1] = []byte("altered")
	m.state.Release.Assets = []remotePublicationAsset{{ID: 1, Name: asset.Name, Size: asset.Size, State: "uploaded", Digest: "sha256:" + asset.SHA256}}
	if err := publicationReadback(context.Background(), m, plan, 789, true, []publicationAsset{asset}); err == nil {
		t.Fatal("provider digest replaced byte readback")
	}
}

func TestPublicationOldInventoryCannotAuthorizeProjection(t *testing.T) {
	dir, plan := publicationFixture(t)
	legacy, err := publicationEvidenceAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := supplyJSON(legacy)
	plan.Authorization.AssetsSHA256 = supplyDigest(data)
	if _, err := preparePublicationWithSource("", dir, plan.Authorization, plan.AuthorizationSHA256, func(string, publicationAuthorization) (string, error) { return plan.Notes, nil }); err == nil {
		t.Fatal("old evidence inventory authorized new five-asset distribution")
	}
}
