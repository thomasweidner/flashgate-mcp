package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/thomasweidner/flashgate-mcp/internal/capability"
	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/catalogbudget"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
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
	Revision       string                                `json:"revision"`
	Profile        string                                `json:"profile"`
	Entry          json.RawMessage                       `json:"entry"`
	ToolsList      json.RawMessage                       `json:"toolsList"`
	Calls          map[string]map[string]json.RawMessage `json:"calls"`
	OperationCases map[string][]wireOperationCase        `json:"operationCases"`
}

// Each operation must identify an executable fixture and its result contract.
// Additional fixtures preserve their separately observed response bytes.
type wireOperationCase struct {
	Operation string          `json:"operation"`
	Fixture   string          `json:"fixture"`
	Variant   string          `json:"resultVariant"`
	Response  json.RawMessage `json:"response,omitempty"`
}

func checkOperationCases(required []tools.CatalogOutcome, cases []wireOperationCase, check func(wireOperationCase) error) error {
	type outcome struct{ operation, result string }
	expected := make(map[outcome]bool, len(required))
	for _, row := range required {
		pair := outcome{wireOperationKey(row.Operation), row.Result}
		if _, duplicate := expected[pair]; duplicate {
			return fmt.Errorf("ambiguous declared operation outcome %s/%s", pair.operation, pair.result)
		}
		expected[pair] = false
	}
	fixtures := make(map[[2]string]bool, len(cases))
	for _, row := range cases {
		pair := outcome{row.Operation, row.Variant}
		id := [2]string{row.Variant, row.Fixture}
		if _, known := expected[pair]; !known || row.Fixture == "" || strings.Contains(row.Fixture, "/") || fixtures[id] {
			return fmt.Errorf("invalid or duplicate operation fixture %s/%s/%s", row.Operation, row.Variant, row.Fixture)
		}
		if err := check(row); err != nil {
			return err
		}
		expected[pair], fixtures[id] = true, true
	}
	for _, row := range required {
		if !expected[outcome{wireOperationKey(row.Operation), row.Result}] {
			return fmt.Errorf("missing executable operation fixture %s/%s", wireOperationKey(row.Operation), row.Result)
		}
	}
	return nil
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

func admitProfile(registry *tools.Registry) error {
	return admitProfileForRevisions(registry, revision.ProductionPolicy())
}

func admitProfileForRevisions(registry *tools.Registry, revisions revision.Policy) error {
	var contexts []handlers.Context
	for _, version := range revisions.Supported() {
		contexts = append(contexts, handlers.Context{Context: context.Background(), ProtocolVersion: version})
	}
	return registry.Activate(contexts...)
}

func validateProfileView(view *tools.CatalogView) error {
	composition := view.Composition()
	version := string(composition.Protocol())
	profile, err := measuredProfile(composition.Policy())
	if err != nil {
		return err
	}
	data, err := profileWireFixtures.ReadFile("testdata/mcp/" + version + "-" + profile + ".json")
	if err != nil {
		return err
	}
	return checkProfileWire(view, version, profile, data, []byte(wireBudgetManifest))
}

func (budgets activationBudgets) check(path, name, contract string, result json.RawMessage) error {
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

func checkProfileWire(registry *tools.CatalogView, version, profile string, fixture, manifest []byte) error {
	var snapshot wireSnapshot
	var budgets activationBudgets
	composition := registry.Composition()
	route, current := composition.Route("")
	measured, err := measuredProfile(composition.Policy())
	if err != nil || profile != measured || version != string(composition.Protocol()) ||
		!current || route.Backend() != execution.CurrentProcess || composition.Origin() != execution.InheritedSTDIO ||
		route.Representation() != "" {
		return errors.New("wire binding differs from the effective composition")
	}
	if json.Unmarshal(fixture, &snapshot) != nil || snapshot.Revision != version || snapshot.Profile != profile ||
		json.Unmarshal(manifest, &budgets) != nil || budgets.Version != "flashgate-benchmark-wire-budgets/v1" {
		return errors.New("invalid profile wire binding")
	}
	path := "production/" + version + "/" + profile + "/stdio-lf"
	entryCase := "initialize///entry"
	if version == revision.Stateless {
		path = "candidate/" + version + "/" + profile + "/stdio-lf"
		entryCase = "server/discover///entry"
	}
	if budgets.Paths[path] == nil {
		return errors.New("missing profile wire path")
	}
	check := func(name, contract string, result json.RawMessage) error {
		return budgets.check(path, name, contract, result)
	}
	if err := check(entryCase, "", snapshot.Entry); err != nil {
		return err
	}
	entry := budgets.Classes[budgets.Cases[entryCase].Class]
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
	const catalogFraming = len("{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":}\n")
	const readCatalogPlanningTokens = 750
	if profile == "read" && (compactList.Len()+catalogFraming+3)/4 > readCatalogPlanningTokens {
		return errors.New("read catalog exceeds the framed token planning ceiling")
	}
	if _, err := catalogbudget.Check(compactList.Bytes(), *catalog.Catalog); err != nil {
		return err
	}
	var expected struct{ Tools []protocol.Tool }
	registered := registry.List()
	if json.Unmarshal(snapshot.ToolsList, &expected) != nil || len(expected.Tools) != len(registered) || len(snapshot.Calls) != len(registered) {
		return errors.New("profile catalog or fixture inventory differs from registered tools")
	}
	if len(snapshot.OperationCases) != len(registered) {
		return errors.New("profile operation fixture inventory differs from effective tools")
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
		operationCases := snapshot.OperationCases[name]
		references := 0
		err = checkOperationCases(registry.Outcomes(name), operationCases, func(operation wireOperationCase) error {
			index := slices.IndexFunc(variants, func(variant tools.ResultVariant) bool { return variant.Name == operation.Variant })
			response := operation.Response
			if operation.Fixture == "reference" {
				if len(operation.Response) != 0 {
					return fmt.Errorf("duplicate reference response for %s", operation.Operation)
				}
				references++
				response = outcomes[wireResultOutcome(index, operation.Variant)]
			}
			variant := variants[index]
			if name == "read_file" && variant.Name == "complete" {
				variant.MaxPayloadBytes = wireFileLimit
			}
			return check("tools/call/"+name+"/"+variant.Name+"/"+operation.Fixture, wireVariantBinding(variant), response)
		})
		if err != nil {
			return err
		}
		if len(outcomes) != references+1 {
			return fmt.Errorf("incomplete profile result fixtures for %s", name)
		}
		if err := check("tools/call/"+name+"/error/denied", "result/invalid_path", outcomes["denied"]); err != nil {
			return err
		}
	}
	return nil
}

func wireResultOutcome(index int, variant string) string {
	if index == 0 {
		return "success"
	}
	return "success_" + variant
}

func wireOperationKey(key execution.OperationKey) string {
	return string(key.Name) + "/" + string(key.Variant)
}

func wireVariantBinding(variant tools.ResultVariant) string {
	kinds := []string{string(variant.ObjectKind)}
	for _, part := range variant.Parts {
		kinds = append(kinds, string(part))
	}
	return fmt.Sprintf("%s/%s/%s/%d/%d", strings.Join(kinds, "+"), variant.Mode, variant.State,
		variant.MaxObjectBytes, variant.MaxPayloadBytes)
}
