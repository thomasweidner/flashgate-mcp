package main

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

type securityIdentity struct {
	Version  string `json:"version"`
	Commit   string `json:"commit"`
	Manifest string `json:"manifestSha256"`
	Index    string `json:"indexSha256"`
	SBOM     string `json:"sbomSha256"`
	GoMod    string `json:"goModSha256"`
}

type securityModule struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Dependency string `json:"dependency"`
	Exposure   string `json:"exposure"`
}

type securitySPDX struct {
	Version  string `json:"spdxVersion"`
	Packages []struct {
		ID      string `json:"SPDXID"`
		Name    string `json:"name"`
		Version string `json:"versionInfo"`
	} `json:"packages"`
	Relationships []struct {
		From string `json:"spdxElementId"`
		Type string `json:"relationshipType"`
		To   string `json:"relatedSpdxElement"`
	} `json:"relationships"`
}

func readSecurityInventory(dir string, identity securityIdentity) (securityIdentity, []securityModule, error) {
	// This remains the only complete supply-chain verifier. No query happens
	// before all candidate artifacts, source inputs and evidence pass it.
	if err := verifySupply(dir, identity.Version, identity.Commit, identity.Manifest, identity.Index, nil); err != nil {
		return identity, nil, errors.New("INVENTORY_VERIFICATION_FAILED")
	}
	root, err := supplyOpen(dir)
	if err != nil {
		return identity, nil, errors.New("INVENTORY_UNAVAILABLE")
	}
	defer root.Close()
	data, err := supplyRead(root, supplyIndexName)
	if err != nil || supplyDigest(data) != identity.Index {
		return identity, nil, errors.New("INDEX_CHANGED")
	}
	var index supplyIndex
	if strictCandidateBytes(data, &index) != nil {
		return identity, nil, errors.New("INDEX_INVALID")
	}
	sbom, err := supplyRead(root, index.SBOM.Path)
	if err != nil || supplyDigest(sbom) != index.SBOM.SHA256 {
		return identity, nil, errors.New("SBOM_CHANGED")
	}
	mod, err := supplyRead(root, "source/go.mod")
	if err != nil {
		return identity, nil, errors.New("SOURCE_UNAVAILABLE")
	}
	bound := false
	for _, input := range index.SourceInputs {
		if input.Path == "source/go.mod" && input.SHA256 == supplyDigest(mod) {
			bound = true
		}
	}
	if !bound {
		return identity, nil, errors.New("SOURCE_CHANGED")
	}
	identity.SBOM, identity.GoMod = supplyDigest(sbom), supplyDigest(mod)
	modules, err := classifySecurityModules(sbom, mod)
	return identity, modules, err
}

func classifySecurityModules(sbom, mod []byte) ([]securityModule, error) {
	var doc securitySPDX
	if json.Unmarshal(sbom, &doc) != nil || doc.Version != "SPDX-2.3" {
		return nil, errors.New("SPDX_INVALID")
	}
	modules := map[string]securityModule{}
	inRequire := false
	for _, line := range strings.Split(string(mod), "\n") {
		code, comment, _ := strings.Cut(line, "//")
		f := strings.Fields(code)
		if len(f) == 0 {
			continue
		}
		if f[0] == "require" {
			if len(f) == 2 && f[1] == "(" {
				inRequire = true
				continue
			}
			f = f[1:]
		} else if f[0] == ")" {
			inRequire = false
			continue
		} else if !inRequire {
			continue
		}
		if len(f) != 2 {
			return nil, errors.New("SOURCE_REQUIRE_INVALID")
		}
		dep := "DIRECT"
		if strings.TrimSpace(comment) == "indirect" {
			dep = "INDIRECT"
		} else if strings.TrimSpace(comment) != "" {
			dep = "UNKNOWN"
		}
		if _, duplicate := modules[f[0]]; duplicate {
			return nil, errors.New("SOURCE_REQUIRE_DUPLICATE")
		}
		modules[f[0]] = securityModule{f[0], f[1], dep, "UNKNOWN"}
	}
	ids := map[string]string{}
	for _, p := range doc.Packages {
		m, ok := modules[p.Name]
		if !ok {
			continue
		}
		if m.Version != p.Version || ids[p.ID] != "" {
			return nil, errors.New("SPDX_MODULE_MISMATCH")
		}
		for _, name := range ids {
			if name == p.Name {
				return nil, errors.New("SPDX_MODULE_DUPLICATE")
			}
		}
		ids[p.ID] = p.Name
	}
	if len(ids) != len(modules) || len(modules) == 0 {
		return nil, errors.New("SPDX_MODULE_MISSING")
	}
	edges := map[string][]string{}
	build, test, runtime := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, r := range doc.Relationships {
		from, to := ids[r.From], ids[r.To]
		if r.Type == "DEPENDS_ON" && from != "" && to != "" {
			edges[from] = append(edges[from], to)
		}
		if r.To == "SPDXRef-FlashGate" && from != "" {
			if r.Type == buildDependency {
				build[from] = true
			}
			if r.Type == testDependency {
				test[from] = true
			}
			if r.Type == "RUNTIME_DEPENDENCY_OF" || r.Type == "DEPENDENCY_OF" {
				runtime[from] = true
			}
		}
		if r.From == "SPDXRef-FlashGate" && to != "" && r.Type == "DEPENDS_ON" {
			runtime[to] = true
		}
	}
	reached := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		for _, child := range edges[name] {
			if !reached[child] {
				reached[child] = true
				visit(child)
			}
		}
	}
	for name, m := range modules {
		if m.Dependency == "DIRECT" {
			visit(name)
		}
	}
	var result []securityModule
	for name, m := range modules {
		if m.Dependency == "INDIRECT" && !reached[name] || m.Dependency == "DIRECT" && reached[name] {
			return nil, errors.New("SOURCE_RELATIONSHIP_CONFLICT")
		}
		switch {
		case runtime[name]:
			m.Exposure = "RUNTIME"
		case build[name]:
			m.Exposure = "BUILD"
		case test[name]:
			m.Exposure = "TEST"
		}
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
