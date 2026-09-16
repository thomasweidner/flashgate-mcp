// Package releaseevidence creates deterministic, machine-readable release
// supply-chain evidence without requiring an external generator.
package releaseevidence

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Module struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Sum     string `json:"sum,omitempty"`
}

type Inventory struct {
	SchemaVersion string   `json:"schema_version"`
	MainModule    string   `json:"main_module"`
	GoVersion     string   `json:"go_version"`
	Modules       []Module `json:"modules"`
}

type Options struct {
	Artifact, Checksum, GoMod, GoSum, OutputDirectory   string
	Version, Commit, SourceTime, Platform, Architecture string
}

func Generate(o Options) ([]string, error) {
	if o.Artifact == "" || o.Checksum == "" || o.GoMod == "" || o.GoSum == "" || o.OutputDirectory == "" {
		return nil, fmt.Errorf("artifact, checksum, go.mod, go.sum, and output directory are required")
	}
	if o.Version == "" || o.Commit == "" || o.SourceTime == "" || o.Platform == "" || o.Architecture == "" {
		return nil, fmt.Errorf("version, commit, source time, platform, and architecture are required")
	}
	digest, err := verifiedDigest(o.Artifact, o.Checksum)
	if err != nil {
		return nil, err
	}
	inv, err := readModules(o.GoMod, o.GoSum)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(filepath.Base(o.Artifact), filepath.Ext(o.Artifact))
	if strings.HasSuffix(o.Artifact, ".tar.gz") {
		base = strings.TrimSuffix(filepath.Base(o.Artifact), ".tar.gz")
	}
	if err := os.MkdirAll(o.OutputDirectory, 0o755); err != nil {
		return nil, err
	}

	spdxPackages := make([]map[string]any, 0, len(inv.Modules)+1)
	spdxPackages = append(spdxPackages, map[string]any{"SPDXID": "SPDXRef-Package-flashgate-mcp", "name": inv.MainModule, "versionInfo": o.Version, "downloadLocation": "NOASSERTION", "filesAnalyzed": false})
	for i, m := range inv.Modules {
		spdxPackages = append(spdxPackages, map[string]any{"SPDXID": fmt.Sprintf("SPDXRef-Package-dependency-%d", i+1), "name": m.Path, "versionInfo": m.Version, "downloadLocation": "NOASSERTION", "filesAnalyzed": false, "checksums": moduleChecksums(m)})
	}
	spdx := map[string]any{"spdxVersion": "SPDX-2.3", "dataLicense": "CC0-1.0", "SPDXID": "SPDXRef-DOCUMENT", "name": base, "documentNamespace": "https://github.com/thomasweidner/flashgate-mcp/sbom/" + o.Commit + "/" + base, "creationInfo": map[string]any{"created": o.SourceTime, "creators": []string{"Tool: flashgate-releaseevidence"}}, "packages": spdxPackages}
	provenance := map[string]any{"_type": "https://in-toto.io/Statement/v1", "subject": []any{map[string]any{"name": filepath.Base(o.Artifact), "digest": map[string]string{"sha256": digest}}}, "predicateType": "https://slsa.dev/provenance/v1", "predicate": map[string]any{"buildDefinition": map[string]any{"buildType": "https://github.com/thomasweidner/flashgate-mcp/.github/workflows/release-build.yml", "externalParameters": map[string]string{"version": o.Version, "platform": o.Platform, "architecture": o.Architecture}, "resolvedDependencies": []any{map[string]any{"uri": "git+https://github.com/thomasweidner/flashgate-mcp@" + o.Commit, "digest": map[string]string{"gitCommit": o.Commit}}}}, "runDetails": map[string]any{"builder": map[string]string{"id": "https://github.com/actions/runner"}, "metadata": map[string]string{"startedOn": o.SourceTime}}}}

	paths := []string{filepath.Join(o.OutputDirectory, base+".dependencies.json"), filepath.Join(o.OutputDirectory, base+".sbom.spdx.json"), filepath.Join(o.OutputDirectory, base+".provenance.intoto.json")}
	for i, v := range []any{inv, spdx, provenance} {
		if err := writeJSON(paths[i], v); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func moduleChecksums(m Module) []map[string]string {
	if !strings.HasPrefix(m.Sum, "h1:") {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(m.Sum, "h1:"))
	if err != nil || len(raw) != sha256.Size {
		return nil
	}
	return []map[string]string{{"algorithm": "SHA256", "checksumValue": hex.EncodeToString(raw)}}
}

func readModules(goMod, goSum string) (Inventory, error) {
	b, err := os.ReadFile(goMod)
	if err != nil {
		return Inventory{}, err
	}
	var inv Inventory
	inv.SchemaVersion = "1"
	lines := strings.Split(string(b), "\n")
	inRequire := false
	for _, raw := range lines {
		line := strings.TrimSpace(strings.Split(raw, "//")[0])
		if strings.HasPrefix(line, "module ") {
			inv.MainModule = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			continue
		}
		if strings.HasPrefix(line, "go ") {
			inv.GoVersion = strings.TrimSpace(strings.TrimPrefix(line, "go "))
			continue
		}
		if line == "require (" {
			inRequire = true
			continue
		}
		if inRequire && line == ")" {
			inRequire = false
			continue
		}
		if strings.HasPrefix(line, "require ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		} else if !inRequire {
			continue
		}
		f := strings.Fields(line)
		if len(f) >= 2 {
			inv.Modules = append(inv.Modules, Module{Path: f[0], Version: f[1]})
		}
	}
	sums, err := os.Open(goSum)
	if err != nil {
		return Inventory{}, err
	}
	defer sums.Close()
	sumMap := map[string]string{}
	scanner := bufio.NewScanner(sums)
	for scanner.Scan() {
		f := strings.Fields(scanner.Text())
		if len(f) == 3 && !strings.HasSuffix(f[1], "/go.mod") {
			sumMap[f[0]+" "+f[1]] = f[2]
		}
	}
	if err := scanner.Err(); err != nil {
		return Inventory{}, err
	}
	for i := range inv.Modules {
		inv.Modules[i].Sum = sumMap[inv.Modules[i].Path+" "+inv.Modules[i].Version]
	}
	sort.Slice(inv.Modules, func(i, j int) bool { return inv.Modules[i].Path < inv.Modules[j].Path })
	if inv.MainModule == "" || inv.GoVersion == "" {
		return Inventory{}, fmt.Errorf("go.mod is missing module or Go version")
	}
	return inv, nil
}

func verifiedDigest(artifact, checksum string) (string, error) {
	b, err := os.ReadFile(artifact)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	got := hex.EncodeToString(sum[:])
	c, err := os.ReadFile(checksum)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(c))
	if len(fields) < 1 || !strings.EqualFold(fields[0], got) {
		return "", fmt.Errorf("artifact checksum does not match %s", checksum)
	}
	return got, nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}
