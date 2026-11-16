package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/catalogbudget"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

//go:embed testdata/mcp/*-read.json testdata/mcp/*-write.json
var profileWireFixtures embed.FS

//go:embed testdata/wire-budgets.json
var wireBudgetManifest string

// The existing wire gate measures read_file against a 64 KiB reference file
// limit. The actual instance cap remains independently configured and enforced.
const wireFileLimit = 64 * 1024

type wireSnapshot struct {
	Revision  string                                `json:"revision"`
	Profile   string                                `json:"profile"`
	Entry     json.RawMessage                       `json:"entry"`
	ToolsList json.RawMessage                       `json:"toolsList"`
	Calls     map[string]map[string]json.RawMessage `json:"calls"`
}

// Each entry binds a complete grant set and ceiling to one measured view.
// Named roots share the same contract because discovery excludes root data.
var measuredProfiles = []struct {
	grants  []capability.ID
	ceiling capability.Risk
	view    string
}{
	{[]capability.ID{capability.FilesRead}, capability.RiskRead, "read"},
	{[]capability.ID{capability.FilesRead}, capability.RiskMutation, "read"},
	{[]capability.ID{capability.FilesRead}, capability.RiskHigh, "read"},
	{[]capability.ID{capability.FilesRead, capability.FilesWrite}, capability.RiskRead, "read"},
	{[]capability.ID{capability.FilesRead, capability.FilesWrite}, capability.RiskMutation, "write"},
	{[]capability.ID{capability.FilesRead, capability.FilesWrite}, capability.RiskHigh, "write"},
}

func measuredProfile(policy capability.Policy) (string, error) {
	grants := policy.Grants().IDs()
	for _, binding := range measuredProfiles {
		if policy.Ceiling() == binding.ceiling && slices.Equal(grants, binding.grants) {
			return binding.view, nil
		}
	}
	return "", errors.New("unmeasured effective profile")
}

// activationBudgets projects the fields needed to bind startup to BL-214/215.
// The executable BL-205/214/215 gate consumes the full manifest and fixtures.
type activationBudgets struct {
	Version string                     `json:"schema_version"`
	Paths   map[string]json.RawMessage `json:"paths"`
	Groups  map[string][]string        `json:"groups"`
	Cases   map[string]struct {
		Class    string `json:"class"`
		Group    string `json:"group"`
		Contract string `json:"contract"`
	} `json:"cases"`
	Classes map[string]struct {
		MaxResponse  uint64                `json:"max_response_bytes"`
		MaxResult    uint64                `json:"max_result_bytes"`
		MaxOverhead  uint64                `json:"max_overhead_bytes"`
		Catalog      *catalogbudget.Limits `json:"catalog"`
		Instructions *uint64               `json:"max_instructions_bytes"`
	} `json:"classes"`
}

func admitProfile(registry *tools.Registry, policy capability.Policy) error {
	profile, err := measuredProfile(policy)
	if err != nil {
		return err
	}
	for _, version := range revision.ProductionPolicy().Supported() {
		data, err := profileWireFixtures.ReadFile("testdata/mcp/" + version + "-" + profile + ".json")
		if err != nil {
			return err
		}
		if err := checkProfileWire(registry, version, profile, data, []byte(wireBudgetManifest)); err != nil {
			return err
		}
	}
	return nil
}

func checkProfileWire(registry *tools.Registry, version, profile string, fixture, manifest []byte) error {
	var snapshot wireSnapshot
	var budgets activationBudgets
	if json.Unmarshal(fixture, &snapshot) != nil || snapshot.Revision != version || snapshot.Profile != profile ||
		json.Unmarshal(manifest, &budgets) != nil || budgets.Version != "flashgate-benchmark-wire-budgets/v1" {
		return errors.New("invalid profile wire binding")
	}
	path := "production/" + version + "/" + profile + "/stdio-lf"
	if budgets.Paths[path] == nil {
		return errors.New("missing profile wire path")
	}
	check := func(name, contract string, result json.RawMessage) error {
		binding := budgets.Cases[name]
		budget := budgets.Classes[binding.Class]
		if binding.Contract != contract || !slices.Contains(budgets.Groups[binding.Group], path) ||
			budget.MaxOverhead == 0 || budget.MaxResult == 0 || budget.MaxResponse == 0 || len(result) == 0 {
			return fmt.Errorf("missing profile wire case %s", name)
		}
		var compact bytes.Buffer
		if json.Compact(&compact, result) != nil || compact.Len() == 0 || compact.Bytes()[0] != '{' {
			return fmt.Errorf("invalid profile result fixture %s", name)
		}
		const framing = uint64(len("{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":}\n"))
		if size := uint64(compact.Len()); size > budget.MaxResult || size+framing > budget.MaxResponse || framing > budget.MaxOverhead {
			return fmt.Errorf("over-budget profile wire case %s", name)
		}
		return nil
	}
	if err := check("initialize///entry", "", snapshot.Entry); err != nil {
		return err
	}
	entry := budgets.Classes[budgets.Cases["initialize///entry"].Class]
	if entry.Instructions == nil || *entry.Instructions != 0 {
		return errors.New("missing binding for instruction-free initialization")
	}
	catalogCase := "tools/list///" + profile
	if err := check(catalogCase, "", snapshot.ToolsList); err != nil {
		return err
	}
	catalog := budgets.Classes[budgets.Cases[catalogCase].Class]
	if catalog.Catalog == nil {
		return errors.New("missing catalog field limits")
	}
	var compactList bytes.Buffer
	if err := json.Compact(&compactList, snapshot.ToolsList); err != nil {
		return err
	}
	if _, err := catalogbudget.Check(compactList.Bytes(), *catalog.Catalog); err != nil {
		return err
	}
	var expected struct{ Tools []protocol.Tool }
	registered := registry.List()
	if json.Unmarshal(snapshot.ToolsList, &expected) != nil || len(expected.Tools) != len(registered) || len(snapshot.Calls) != len(registered) {
		return errors.New("profile catalog or fixture inventory differs from registered tools")
	}
	for i, tool := range registered {
		name := tool.Name()
		actual, err := json.Marshal(tool.Definition())
		want, wantErr := json.Marshal(expected.Tools[i])
		if err != nil || wantErr != nil || name != expected.Tools[i].Name || !bytes.Equal(actual, want) {
			return fmt.Errorf("unmeasured tool definition %s", name)
		}
		outcomes := snapshot.Calls[name]
		variants := registry.ResultContract(name)
		if len(outcomes) != len(variants)+1 {
			return fmt.Errorf("incomplete profile result fixtures for %s", name)
		}
		for j, variant := range variants {
			outcome := "success"
			if j > 0 {
				outcome += "_" + variant.Name
			}
			if name == "read_file" && variant.Name == "complete" {
				variant.MaxPayloadBytes = wireFileLimit
			}
			if err := check("tools/call/"+name+"/"+variant.Name+"/reference", wireVariantBinding(variant), outcomes[outcome]); err != nil {
				return err
			}
		}
		if err := check("tools/call/"+name+"/error/denied", "result/invalid_path", outcomes["denied"]); err != nil {
			return err
		}
	}
	return nil
}

func wireVariantBinding(variant tools.ResultVariant) string {
	kinds := []string{string(variant.ObjectKind)}
	for _, part := range variant.Parts {
		kinds = append(kinds, string(part))
	}
	return fmt.Sprintf("%s/%s/%s/%d/%d", strings.Join(kinds, "+"), variant.Mode, variant.State,
		variant.MaxObjectBytes, variant.MaxPayloadBytes)
}
