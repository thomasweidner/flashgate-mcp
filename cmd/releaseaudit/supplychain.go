package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const supplySchema = "flashgate-supply-chain-evidence"
const supplyContract = 1
const supplyIndexName = "supply-chain-index.json"
const sbomName = "sbom.spdx.json"
const projectURI = "https://github.com/thomasweidner/flashgate-mcp"
const candidateVerificationPredicate = projectURI + "/attestations/candidate-verification/v1"
const maximumSupplyMetadataSize = 2 << 20
const maximumSupplyArchiveSize = 4*maximumEntrySize + (64 << 10)

// Source inputs are carried with the bundle for offline revalidation. They are
// checked against the candidate commit before generation, never against HEAD by
// assumption. Vendored module identity is scope, not a binary composition claim.
var supplySourceNames = []string{
	"VERSION", "go.mod", "go.sum", "vendor/modules.txt", "THIRD-PARTY-NOTICES.md",
	"assets/branding/README.md", "assets/branding/fontawesome-bolt-lightning-solid.svg", "assets/branding/flashgate.ico",
}

type supplyFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type supplyTarget struct {
	Platform           string     `json:"platform"`
	Architecture       string     `json:"architecture"`
	ArtifactSHA256     string     `json:"artifactSha256"`
	VerificationSHA256 string     `json:"verificationSha256"`
	ReproductionSHA256 string     `json:"reproductionSha256"`
	Provenance         supplyFile `json:"provenance"`
}

type supplyIndex struct {
	Schema           string         `json:"schema"`
	ContractVersion  int            `json:"contractVersion"`
	Version          string         `json:"version"`
	SourceCommitSHA  string         `json:"sourceCommitSha"`
	SourceCommitTime string         `json:"sourceCommitTime"`
	Candidate        supplyFile     `json:"candidateManifest"`
	SourceInputs     []supplyFile   `json:"sourceInputs"`
	SBOM             supplyFile     `json:"sbom"`
	Targets          []supplyTarget `json:"targets"`
	SigningStatus    string         `json:"signingStatus"`
}

func supplyDigest(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func supplyJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	return append(data, '\n'), err
}

// os.Root confines all opens, including during concurrent directory exchange.
// Lstat additionally rejects links even when their destinations are confined.
func supplyRegularOpen(root *os.Root, name string, maximum int64) (*os.File, error) {
	if name == "" || filepath.IsAbs(name) || strings.ContainsAny(name, `\:`) || filepath.ToSlash(filepath.Clean(name)) != name || name == "." || strings.HasPrefix(name, "../") {
		return nil, errors.New("unsafe supply-chain relative path")
	}
	parts := strings.Split(name, "/")
	var expected os.FileInfo
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || (i < len(parts)-1 && !info.IsDir()) || (i == len(parts)-1 && !info.Mode().IsRegular()) {
			return nil, errors.New("supply-chain input must have real parents and regular file type")
		}
		expected = info
	}
	if expected.Size() > maximum {
		return nil, errors.New("supply-chain file exceeds size bound")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	actual, err := file.Stat()
	if err != nil || !actual.Mode().IsRegular() || !os.SameFile(expected, actual) || actual.Size() > maximum {
		return nil, errors.Join(errors.New("supply-chain file identity/type/size changed"), err, file.Close())
	}
	return file, nil
}

func supplyRead(root *os.Root, name string) ([]byte, error) {
	file, err := supplyRegularOpen(root, name, maximumSupplyMetadataSize)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maximumSupplyMetadataSize+1))
	err = errors.Join(readErr, file.Close())
	if len(data) > maximumSupplyMetadataSize {
		err = errors.Join(err, errors.New("supply-chain metadata grew past size bound"))
	}
	return data, err
}

func supplyCopy(root *os.Root, name, target string, maximum int64) error {
	source, err := supplyRegularOpen(root, name, maximum)
	if err != nil {
		return err
	}
	destination, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.Join(err, source.Close())
	}
	length, copyErr := io.CopyBuffer(destination, io.LimitReader(source, maximum+1), make([]byte, 64<<10))
	if length > maximum {
		copyErr = errors.Join(copyErr, errors.New("supply-chain input grew past copy bound"))
	}
	return errors.Join(copyErr, source.Close(), destination.Close())
}

func supplyOpen(dir string) (*os.Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := rejectSymlinkParents(abs); err != nil {
		return nil, err
	}
	return os.OpenRoot(abs)
}

type supplyModule struct{ path, version, h1, modH1 string }

// This parser intentionally accepts the current simple pinned module contract.
// Replacement, workspace and tool directives require a new reviewed inventory
// contract rather than quietly producing an incomplete SBOM.
func supplyModules(mod, sums, vendor []byte) ([]supplyModule, error) {
	required := map[string]string{}
	inRequire := false
	moduleSeen, goSeen := false, false
	for _, line := range strings.Split(string(mod), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "//", 2)[0])
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "module":
			if moduleSeen || len(fields) != 2 || fields[1] != "github.com/thomasweidner/flashgate-mcp" {
				return nil, errors.New("module identity mismatch")
			}
			moduleSeen = true
		case "go":
			if goSeen || len(fields) != 2 {
				return nil, errors.New("invalid Go version")
			}
			goSeen = true
		case "require":
			if len(fields) == 2 && fields[1] == "(" && !inRequire {
				inRequire = true
				continue
			}
			if len(fields) != 3 || inRequire {
				return nil, errors.New("invalid require")
			}
			fields = fields[1:]
		case ")":
			if !inRequire || len(fields) != 1 {
				return nil, errors.New("invalid require block")
			}
			inRequire = false
			continue
		default:
			if !inRequire {
				return nil, errors.New("unsupported module directive")
			}
		}
		if fields[0] == "module" || fields[0] == "go" {
			continue
		}
		if len(fields) != 2 || required[fields[0]] != "" || !strings.HasPrefix(fields[1], "v") {
			return nil, errors.New("duplicate or invalid module requirement")
		}
		required[fields[0]] = fields[1]
	}
	if inRequire || !moduleSeen || !goSeen || len(required) == 0 {
		return nil, errors.New("incomplete go.mod")
	}
	checks := map[string]string{}
	for _, line := range strings.Split(string(sums), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 3 || !strings.HasPrefix(f[2], "h1:") {
			return nil, errors.New("invalid go.sum checksum")
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(f[2], "h1:"))
		if err != nil || len(decoded) != 32 || base64.StdEncoding.EncodeToString(decoded) != strings.TrimPrefix(f[2], "h1:") {
			return nil, errors.New("invalid Go h1 encoding")
		}
		key := f[0] + " " + f[1]
		if checks[key] != "" {
			return nil, errors.New("duplicate go.sum checksum")
		}
		checks[key] = f[2]
	}
	seen := map[string]bool{}
	var current string
	explicit := false
	packages := 0
	finish := func() error {
		if current != "" && (!explicit || packages == 0) {
			return errors.New("incomplete vendored module")
		}
		return nil
	}
	for _, line := range strings.Split(string(vendor), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			if err := finish(); err != nil {
				return nil, err
			}
			f := strings.Fields(line)
			if len(f) != 3 || required[f[1]] != f[2] || seen[f[1]] {
				return nil, errors.New("vendor module/version mismatch")
			}
			current, explicit, packages = f[1], false, 0
			seen[current] = true
		} else if strings.HasPrefix(line, "## ") {
			if current == "" || explicit || !strings.HasPrefix(line, "## explicit;") {
				return nil, errors.New("vendor explicit identity mismatch")
			}
			explicit = true
		} else {
			if current == "" || !(line == current || strings.HasPrefix(line, current+"/")) {
				return nil, errors.New("vendor package identity mismatch")
			}
			packages++
		}
	}
	if err := finish(); err != nil {
		return nil, err
	}
	if len(seen) != len(required) {
		return nil, errors.New("missing vendored module")
	}
	if len(required) != 2 {
		return nil, errors.New("reviewed build dependency graph changed")
	}
	var result []supplyModule
	for module, value := range required {
		// These are the reviewed build-only resource generators in this product.
		if module != "github.com/akavel/rsrc" && module != "github.com/josephspurrier/goversioninfo" {
			return nil, errors.New("dependency purpose requires review")
		}
		m := supplyModule{module, value, checks[module+" "+value], checks[module+" "+value+"/go.mod"]}
		if m.h1 == "" || m.modH1 == "" {
			return nil, errors.New("missing active module checksums")
		}
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].path < result[j].path })
	return result, nil
}

func supplyDocuments(manifest candidateManifest, manifestHash, sourceTime string, inputs map[string][]byte) (map[string][]byte, supplyIndex, error) {
	files := map[string][]byte{}
	index := supplyIndex{Schema: supplySchema, ContractVersion: supplyContract, Version: manifest.Version, SourceCommitSHA: manifest.SourceCommitSHA, SourceCommitTime: sourceTime,
		Candidate: supplyFile{"candidate/candidate-manifest.json", manifestHash}, SigningStatus: "unsigned"}
	timestamp, err := time.Parse(time.RFC3339, sourceTime)
	if err != nil || timestamp.UTC().Format(time.RFC3339) != sourceTime {
		return nil, index, errors.New("invalid source commit time")
	}
	for _, name := range supplySourceNames {
		if len(inputs[name]) == 0 {
			return nil, index, errors.New("missing source inventory input")
		}
	}
	branding := string(inputs["assets/branding/README.md"])
	if !strings.Contains(branding, "`7.2.0`") || !strings.Contains(branding, "CC BY 4.0") || !strings.Contains(string(inputs["THIRD-PARTY-NOTICES.md"]), "7.2.0") {
		return nil, index, errors.New("branding version or license requires review")
	}
	if string(inputs["VERSION"]) != manifest.Version+"\n" && string(inputs["VERSION"]) != manifest.Version+"\r\n" {
		return nil, index, errors.New("source VERSION mismatch")
	}
	modules, err := supplyModules(inputs["go.mod"], inputs["go.sum"], inputs["vendor/modules.txt"])
	if err != nil {
		return nil, index, err
	}
	for _, name := range supplySourceNames {
		index.SourceInputs = append(index.SourceInputs, supplyFile{"source/" + name, supplyDigest(inputs[name])})
	}
	packages := []any{map[string]any{"SPDXID": "SPDXRef-FlashGate", "name": "flashgate-mcp", "versionInfo": manifest.Version, "downloadLocation": projectURI, "filesAnalyzed": false, "licenseConcluded": "NOASSERTION", "licenseDeclared": "GPL-3.0-only", "copyrightText": "NOASSERTION", "primaryPackagePurpose": "APPLICATION"}}
	relations := []any{map[string]string{"spdxElementId": "SPDXRef-DOCUMENT", "relationshipType": "DESCRIBES", "relatedSpdxElement": "SPDXRef-FlashGate"}}
	for i, m := range modules {
		id := fmt.Sprintf("SPDXRef-GoModule-%d", i+1)
		packages = append(packages, map[string]any{"SPDXID": id, "name": m.path, "versionInfo": m.version, "downloadLocation": "NOASSERTION", "filesAnalyzed": false, "licenseConcluded": "NOASSERTION", "licenseDeclared": "NOASSERTION", "copyrightText": "NOASSERTION", "primaryPackagePurpose": "LIBRARY", "comment": "Build-only Windows resource generator. Go module content h1: " + m.h1 + "; go.mod h1: " + m.modH1 + ". Go h1 is a normalized module directory checksum, not an archive SHA-256."})
		relations = append(relations, map[string]string{"spdxElementId": id, "relationshipType": "BUILD_DEPENDENCY_OF", "relatedSpdxElement": "SPDXRef-FlashGate"})
	}
	relations = append(relations, map[string]string{"spdxElementId": "SPDXRef-GoModule-2", "relationshipType": "DEPENDS_ON", "relatedSpdxElement": "SPDXRef-GoModule-1"})
	for i, name := range []string{"assets/branding/fontawesome-bolt-lightning-solid.svg", "assets/branding/flashgate.ico"} {
		id := fmt.Sprintf("SPDXRef-Branding-%d", i+1)
		packages = append(packages, map[string]any{"SPDXID": id, "name": filepath.Base(name), "versionInfo": "Font-Awesome-Free-7.2.0", "downloadLocation": "NOASSERTION", "filesAnalyzed": false, "licenseConcluded": "NOASSERTION", "licenseDeclared": "CC-BY-4.0", "copyrightText": "Copyright 2026 Fonticons, Inc.", "primaryPackagePurpose": "SOURCE", "checksums": []any{map[string]string{"algorithm": "SHA256", "checksumValue": supplyDigest(inputs[name])}}, "comment": "Third-party branding asset; ICO is a repository-generated derivative used by Windows builds."})
		relations = append(relations, map[string]string{"spdxElementId": "SPDXRef-FlashGate", "relationshipType": "CONTAINS", "relatedSpdxElement": id})
	}
	relations = append(relations, map[string]string{"spdxElementId": "SPDXRef-Branding-2", "relationshipType": "GENERATED_FROM", "relatedSpdxElement": "SPDXRef-Branding-1"})
	sbom := map[string]any{"spdxVersion": "SPDX-2.3", "dataLicense": "CC0-1.0", "SPDXID": "SPDXRef-DOCUMENT", "name": "FlashGate source dependency inventory", "documentNamespace": projectURI + "/sbom/" + manifest.SourceCommitSHA + "/" + manifestHash, "creationInfo": map[string]any{"creators": []string{"Tool: flashgate-releaseaudit"}, "created": sourceTime, "comment": "Deterministic source commit timestamp, not wall-clock generation or build invocation time."}, "documentComment": "Source inventory of active vendored Go build dependencies and third-party branding. Current server has no external Go runtime module dependency. Standard library, compiler, OS libraries, CI utilities and platform binary composition are not exhaustively inventoried. Carried go.sum includes retained test/historical checksum entries, which are not active vendored or shipped dependencies.", "packages": packages, "relationships": relations}
	files[sbomName], err = supplyJSON(sbom)
	if err != nil {
		return nil, index, err
	}
	index.SBOM = supplyFile{sbomName, supplyDigest(files[sbomName])}
	for _, record := range manifest.Records {
		name := "provenance-" + record.Platform + "-" + record.Architecture + ".json"
		// This records verification bindings, not artifact construction or a builder.
		statement := map[string]any{
			"_type":         "https://in-toto.io/Statement/v1",
			"subject":       []any{map[string]any{"name": record.Archive, "digest": map[string]string{"sha256": record.ArtifactSHA256}}},
			"predicateType": candidateVerificationPredicate,
			"predicate": map[string]any{
				"version":           manifest.Version,
				"platform":          record.Platform,
				"architecture":      record.Architecture,
				"source":            map[string]any{"uri": "git+" + projectURI, "digest": map[string]string{"gitCommit": manifest.SourceCommitSHA}},
				"candidateManifest": supplyFile{"candidate/candidate-manifest.json", manifestHash},
				"verification":      supplyFile{"candidate/" + record.VerificationEvidence, record.VerificationSHA256},
				"reproduction":      supplyFile{"candidate/" + record.CompareReport, record.CompareSHA256},
			},
		}
		files[name], err = supplyJSON(statement)
		if err != nil {
			return nil, index, err
		}
		index.Targets = append(index.Targets, supplyTarget{record.Platform, record.Architecture, record.ArtifactSHA256, record.VerificationSHA256, record.CompareSHA256, supplyFile{name, supplyDigest(files[name])}})
	}
	files[supplyIndexName], err = supplyJSON(index)
	return files, index, err
}

func supplyInventory(root *os.Root, dir string, allowed map[string]bool) error {
	entries, err := fs.ReadDir(root.FS(), dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if dir != "." {
			name = dir + "/" + name
		}
		if entry.IsDir() {
			wanted := false
			for file := range allowed {
				if strings.HasPrefix(file, name+"/") {
					wanted = true
					break
				}
			}
			if !wanted {
				return errors.New("unexpected supply-chain directory")
			}
			if err := supplyInventory(root, name, allowed); err != nil {
				return err
			}
			continue
		}
		if !allowed[name] {
			return errors.New("unexpected supply-chain file")
		}
		limit := int64(maximumSupplyMetadataSize)
		if strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".tar.gz") {
			limit = maximumSupplyArchiveSize
		}
		file, err := supplyRegularOpen(root, name, limit)
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		delete(allowed, name)
	}
	return nil
}

// Trust comes from externally pinned index/manifest digests and identity, not
// from hashes supplied by the same untrusted bundle.
func verifySupply(dir, value, commit, manifestHash, indexHash string, forbidden []string) error {
	if !hashPattern.MatchString(indexHash) || !hashPattern.MatchString(manifestHash) {
		return errors.New("external evidence digest anchors required")
	}
	root, err := supplyOpen(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	data, err := supplyRead(root, supplyIndexName)
	if err != nil {
		return err
	}
	if supplyDigest(data) != indexHash {
		return errors.New("evidence index digest mismatch")
	}
	var stored supplyIndex
	if err := strictCandidateBytes(data, &stored); err != nil {
		return err
	}
	if stored.Version != value || stored.SourceCommitSHA != commit || stored.Candidate.SHA256 != manifestHash {
		return errors.New("external candidate identity mismatch")
	}
	data, err = supplyRead(root, "candidate/candidate-manifest.json")
	if err != nil {
		return err
	}
	if supplyDigest(data) != manifestHash {
		return errors.New("candidate manifest digest mismatch")
	}
	var manifest candidateManifest
	if err := strictCandidateBytes(data, &manifest); err != nil {
		return err
	}
	if manifest.Version != value || manifest.SourceCommitSHA != commit {
		return errors.New("candidate identity mismatch")
	}
	// Check every candidate input through the confined reader before reuse of
	// the BL-255 semantic verifier. Caller controls this read-only bundle.
	allowed := map[string]bool{}
	for _, name := range promotedNames(manifest) {
		name = "candidate/" + name
		if strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".tar.gz") {
			file, err := supplyRegularOpen(root, name, maximumSupplyArchiveSize)
			if err != nil {
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		} else {
			data, err := supplyRead(root, name)
			if err != nil {
				return err
			}
			report := leakReport{}
			scanBytes(name, data, forbidden, &report)
			if len(report.Findings) > 0 {
				return errors.New("carried candidate evidence leak detected")
			}
		}
		allowed[name] = true
	}
	if err := verifyCandidateManifest(filepath.Join(dir, "candidate"), manifest); err != nil {
		return err
	}
	inputs := map[string][]byte{}
	for _, name := range supplySourceNames {
		data, err := supplyRead(root, "source/"+name)
		if err != nil {
			return err
		}
		inputs[name] = data
		report := leakReport{}
		scanBytes("source/"+name, data, forbidden, &report)
		if len(report.Findings) > 0 {
			return errors.New("supply-chain source input leak detected")
		}
		allowed["source/"+name] = true
	}
	files, index, err := supplyDocuments(manifest, manifestHash, stored.SourceCommitTime, inputs)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(stored, index) {
		return errors.New("supply-chain index semantics differ")
	}
	for name, expected := range files {
		actual, err := supplyRead(root, name)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, expected) {
			return errors.New("supply-chain document semantics or deterministic bytes differ")
		}
		report := leakReport{}
		scanBytes(name, actual, forbidden, &report)
		if len(report.Findings) > 0 {
			return errors.New("supply-chain evidence leak detected")
		}
		allowed[name] = true
	}
	if err := supplyInventory(root, ".", allowed); err != nil {
		return err
	}
	if len(allowed) != 0 {
		return errors.New("incomplete supply-chain bundle")
	}
	return nil
}

func generateSupply(source, candidate, destination, commitTime string, manifest candidateManifest, canonical map[string][]byte, forbidden []string) (result error) {
	// New set only: no in-place update, including an identical regeneration.
	abs, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return err
	}
	for _, pair := range [][2]string{{candidateAbs, abs}, {abs, candidateAbs}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err != nil {
			return err
		}
		if rel == "." || (!filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return errors.New("evidence output must be separate from candidate input")
		}
	}
	for parent := filepath.Dir(abs); ; parent = filepath.Dir(parent) {
		if _, err := os.Lstat(filepath.Join(parent, "candidate-manifest.json")); err == nil {
			return errors.New("evidence output cannot modify a candidate directory")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if filepath.Dir(parent) == parent {
			break
		}
	}
	if err := rejectSymlinkParents(abs); err != nil {
		return err
	}
	if _, err := os.Lstat(abs); !errors.Is(err, os.ErrNotExist) {
		return errors.New("supply-chain output already exists or cannot be inspected")
	}
	sourceRoot, err := supplyOpen(source)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	candidateRoot, err := supplyOpen(candidate)
	if err != nil {
		return err
	}
	defer candidateRoot.Close()
	// Enforce resource bounds before the retained BL-255 hash/semantic checks.
	for _, name := range promotedNames(manifest) {
		limit := int64(maximumSupplyMetadataSize)
		if strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".tar.gz") {
			limit = maximumSupplyArchiveSize
		}
		file, err := supplyRegularOpen(candidateRoot, name, limit)
		if err != nil {
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	if err := verifyCandidateManifest(candidate, manifest); err != nil {
		return err
	}
	inputs := map[string][]byte{}
	for _, name := range supplySourceNames {
		data, err := supplyRead(sourceRoot, name)
		if err != nil {
			return err
		}
		if canonical != nil {
			committed, ok := canonical[name]
			if !ok {
				return errors.New("missing canonical source input")
			}
			if !bytes.Equal(data, committed) && !(name != "assets/branding/flashgate.ico" && bytes.Equal(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(committed, []byte("\r\n"), []byte("\n")))) {
				return errors.New("source input changed before generation")
			}
			data = committed
		}
		inputs[name] = data
	}
	manifestBytes, err := supplyRead(candidateRoot, "candidate-manifest.json")
	if err != nil {
		return err
	}
	var bound candidateManifest
	if err := strictCandidateBytes(manifestBytes, &bound); err != nil {
		return err
	}
	if !reflect.DeepEqual(bound, manifest) {
		return errors.New("candidate manifest changed")
	}
	files, _, err := supplyDocuments(manifest, supplyDigest(manifestBytes), commitTime, inputs)
	if err != nil {
		return err
	}
	parent := filepath.Dir(abs)
	stage, err := os.MkdirTemp(parent, ".supply-chain-")
	if err != nil {
		return err
	}
	promotion, err := bindStageRename(stage, abs)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, promotion.cleanup()) }()
	write := func(name string, data []byte) error {
		p := filepath.Join(stage, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return errors.Join(err, f.Close())
	}
	for name, data := range files {
		if err := write(name, data); err != nil {
			return err
		}
	}
	for name, data := range inputs {
		if err := write("source/"+name, data); err != nil {
			return err
		}
	}
	for _, name := range promotedNames(manifest) {
		limit := int64(maximumSupplyMetadataSize)
		if strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".tar.gz") {
			limit = maximumSupplyArchiveSize
		}
		target := filepath.Join(stage, "candidate", name)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := supplyCopy(candidateRoot, name, target, limit); err != nil {
			return err
		}
	}
	if err := verifySupply(stage, manifest.Version, manifest.SourceCommitSHA, supplyDigest(manifestBytes), supplyDigest(files[supplyIndexName]), forbidden); err != nil {
		return err
	}
	if err := promotion.rename(func(path string) error {
		return verifySupply(path, manifest.Version, manifest.SourceCommitSHA, supplyDigest(manifestBytes), supplyDigest(files[supplyIndexName]), forbidden)
	}); err != nil {
		return err
	}
	return verifySupply(abs, manifest.Version, manifest.SourceCommitSHA, supplyDigest(manifestBytes), supplyDigest(files[supplyIndexName]), forbidden)
}

func runSupply(arguments []string, verify bool) error {
	flags := flag.NewFlagSet("supply-chain", flag.ContinueOnError)
	root := flags.String("root", "", "source checkout (generation only)")
	dir := flags.String("dir", "", "complete candidate directory or evidence bundle")
	output := flags.String("output", "", "new evidence directory")
	value := flags.String("version", "", "expected VERSION")
	commit := flags.String("commit", "", "expected source commit")
	manifestHash := flags.String("manifest-sha256", "", "external candidate manifest anchor")
	indexHash := flags.String("index-sha256", "", "external evidence index anchor")
	var forbidden stringList
	flags.Var(&forbidden, "forbidden", "host-specific value to reject")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *dir == "" || flags.NArg() != 0 {
		return errors.New("supply-chain requires --dir and no positional arguments")
	}
	if err := validateCandidateIdentity(*value, *commit); err != nil {
		return err
	}
	if verify {
		if *root != "" || *output != "" {
			return errors.New("verify accepts no generation paths")
		}
		return verifySupply(*dir, *value, *commit, *manifestHash, *indexHash, forbidden)
	}
	if *root == "" || *output == "" || *indexHash != "" || !hashPattern.MatchString(*manifestHash) {
		return errors.New("generation requires root, new output and manifest anchor")
	}
	candidateRoot, err := supplyOpen(*dir)
	if err != nil {
		return err
	}
	defer candidateRoot.Close()
	data, err := supplyRead(candidateRoot, "candidate-manifest.json")
	if err != nil {
		return err
	}
	if supplyDigest(data) != *manifestHash {
		return errors.New("candidate manifest anchor mismatch")
	}
	var manifest candidateManifest
	if err := strictCandidateBytes(data, &manifest); err != nil {
		return err
	}
	if manifest.Version != *value || manifest.SourceCommitSHA != *commit {
		return errors.New("candidate identity mismatch")
	}
	// Do not invent provenance from a different checkout or dirty source input.
	git := func(args ...string) ([]byte, error) {
		command := exec.Command("git", append([]string{"-C", *root}, args...)...)
		return command.Output()
	}
	head, err := git("rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(head)) != *commit {
		return errors.New("source checkout commit mismatch")
	}
	canonical := map[string][]byte{}
	sourceRoot, err := supplyOpen(*root)
	if err != nil {
		return err
	}
	defer sourceRoot.Close()
	for _, name := range supplySourceNames {
		sizeBytes, err := git("cat-file", "-s", *commit+":"+name)
		if err != nil {
			return errors.New("source commit input unavailable")
		}
		size, err := strconv.ParseInt(strings.TrimSpace(string(sizeBytes)), 10, 64)
		if err != nil || size < 0 || size > maximumSupplyMetadataSize {
			return errors.New("source commit input exceeds size bound")
		}
		committed, err := git("show", *commit+":"+name)
		if err != nil {
			return errors.New("source commit input unavailable")
		}
		actual, err := supplyRead(sourceRoot, name)
		if err != nil {
			return err
		}
		// Git checkout EOL conversion is allowed only for text inputs; bind
		// exactly the committed canonical bytes in the portable bundle.
		if !bytes.Equal(actual, committed) && !(name != "assets/branding/flashgate.ico" && bytes.Equal(bytes.ReplaceAll(actual, []byte("\r\n"), []byte("\n")), bytes.ReplaceAll(committed, []byte("\r\n"), []byte("\n")))) {
			return errors.New("source input differs from candidate commit")
		}
		canonical[name] = committed
	}
	epoch, err := git("show", "-s", "--format=%ct", *commit)
	if err != nil {
		return err
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(string(epoch)), 10, 64)
	if err != nil || seconds < 0 {
		return errors.New("invalid source commit epoch")
	}
	return generateSupply(*root, *dir, *output, time.Unix(seconds, 0).UTC().Format(time.RFC3339), manifest, canonical, forbidden)
}
