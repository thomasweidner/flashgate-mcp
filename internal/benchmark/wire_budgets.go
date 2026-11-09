package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"math/bits"
	"os"
	"slices"
)

const wireBudgetVersion = "flashgate-benchmark-wire-budgets/v1"

type WireBudget struct {
	MaxResponseBytes uint64             `json:"max_response_bytes"`
	MaxResultBytes   uint64             `json:"max_result_bytes"`
	MaxOverheadBytes uint64             `json:"max_overhead_bytes"`
	MaxFrameBytes    uint64             `json:"max_frame_bytes,omitempty"`
	MaxAmplification uint64             `json:"max_amplification_milli,omitempty"`
	MaxTokenCost     uint64             `json:"max_token_cost_milli,omitempty"`
	Catalog          *WireCatalogBudget `json:"catalog,omitempty"`
	Instructions     *uint64            `json:"max_instructions_bytes,omitempty"`
}

type WireCatalogBudget struct {
	MaxTools             uint64 `json:"max_tools"`
	MaxNameBytes         uint64 `json:"max_name_bytes"`
	MaxTitleBytes        uint64 `json:"max_title_bytes"`
	MaxDescriptionBytes  uint64 `json:"max_description_bytes"`
	MaxInputSchemaBytes  uint64 `json:"max_input_schema_bytes"`
	MaxOutputSchemaBytes uint64 `json:"max_output_schema_bytes"`
}

type wireCatalogCost struct {
	Bytes        uint64
	ApproxTokens uint64
}

type WireCatalogMeasurement struct {
	ToolCount                                                              uint64
	Names, Titles, Descriptions, InputSchemas, OutputSchemas, Instructions wireCatalogCost
	InstructionsPresent                                                    bool
}

type WirePathBudget struct {
	MaxRevisionBytes uint64 `json:"max_revision_bytes,omitempty"`
}

type WireBudgetBinding struct {
	Class    string `json:"class"`
	Group    string `json:"group"`
	Contract string `json:"contract,omitempty"`
}

type WireBudgets struct {
	Version string                       `json:"schema_version"`
	Paths   map[string]WirePathBudget    `json:"paths"`
	Groups  map[string][]string          `json:"groups"`
	Cases   map[string]WireBudgetBinding `json:"cases"`
	Classes map[string]WireBudget        `json:"classes"`
}

func LoadWireBudgets(path string) (WireBudgets, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return WireBudgets{}, benchmarkReadError("wire budgets", path, err)
	}
	var budgets WireBudgets
	if err := decodeStrictJSON(raw, &budgets); err != nil {
		return WireBudgets{}, fmt.Errorf("decode wire budgets: %w", err)
	}
	if budgets.Version != wireBudgetVersion {
		return WireBudgets{}, fmt.Errorf("unsupported wire budget schema %q", budgets.Version)
	}
	return budgets, nil
}

func (id WireIdentity) BudgetPath() string {
	return string(id.Source) + "/" + id.Revision + "/" + id.Profile + "/" + id.Transport
}

func (id WireIdentity) BudgetCase() string {
	return id.Route + "/" + id.Tool + "/" + id.Variant + "/" + id.Fixture
}

// CheckWireCoverage checks exact per-path fixture/budget parity. Groups let
// cases share applicable paths without requiring a theoretical cross product.
func CheckWireCoverage(ids []WireIdentity, budgets WireBudgets) error {
	type key struct{ path, name string }
	required := make(map[key]string)
	usedGroups, usedClasses := map[string]bool{}, map[string]bool{}
	usedPaths := make(map[string]bool)
	for _, name := range slices.Sorted(maps.Keys(budgets.Cases)) {
		binding := budgets.Cases[name]
		paths := budgets.Groups[binding.Group]
		if _, ok := budgets.Classes[binding.Class]; !ok || len(paths) == 0 || name == "" {
			return fmt.Errorf("missing wire budget or path group for %s", name)
		}
		usedGroups[binding.Group], usedClasses[binding.Class] = true, true
		for _, path := range paths {
			k := key{path, name}
			_, duplicate := required[k]
			if _, ok := budgets.Paths[path]; !ok || duplicate {
				return fmt.Errorf("unknown or duplicate wire path %s for %s", path, name)
			}
			required[k] = binding.Contract
			usedPaths[path] = true
		}
	}
	for _, id := range ids {
		k := key{id.BudgetPath(), id.BudgetCase()}
		contract, ok := required[k]
		if !ok {
			return fmt.Errorf("duplicate fixture or missing wire budget for %s/%s", k.path, k.name)
		}
		if id.Contract != contract {
			return fmt.Errorf("registered variant contract does not match the wire budget for %s/%s", k.path, k.name)
		}
		delete(required, k)
	}
	if len(required) != 0 {
		var missing []string
		for k := range required {
			missing = append(missing, k.path+"/"+k.name)
		}
		slices.Sort(missing)
		return fmt.Errorf("missing fixture for %s", missing[0])
	}
	if len(usedGroups) != len(budgets.Groups) || len(usedClasses) != len(budgets.Classes) || len(usedPaths) != len(budgets.Paths) {
		return fmt.Errorf("unexpected or unused wire budget class, path or path group")
	}
	return nil
}

// CheckWireBudget applies deterministic limits to the whole logical delivery.
// The gate also compares explicit operation traces at the producer.
func CheckWireBudget(m WireMeasurement, budget WireBudget, path WirePathBudget) error {
	if !consistentWireTotals(m) {
		return fmt.Errorf("inconsistent wire measurement")
	}
	c := m.Counts
	if budget.MaxResponseBytes == 0 || budget.MaxResultBytes == 0 || budget.MaxOverheadBytes == 0 ||
		(budget.MaxAmplification == 0) != (budget.MaxTokenCost == 0) ||
		(c.UsefulBytes == 0) != (budget.MaxAmplification == 0) {
		return fmt.Errorf("invalid wire budget for useful byte count %d", c.UsefulBytes)
	}
	revisionBytes, carry := bits.Add64(c.RevisionMetaBytes, c.ResultTypeBytes, 0)
	revisionBytes, cacheCarry := bits.Add64(revisionBytes, c.CacheBytes, 0)
	if c.UsefulBytes > c.ResponseBytes || c.ResultBytes > c.ResponseBytes || carry|cacheCarry != 0 {
		return fmt.Errorf("inconsistent wire measurement")
	}
	for _, limit := range []struct {
		name        string
		actual, max uint64
	}{
		{"response bytes", c.ResponseBytes, budget.MaxResponseBytes},
		{"result bytes", c.ResultBytes, budget.MaxResultBytes},
		{"overhead bytes", c.ResponseBytes - c.UsefulBytes, budget.MaxOverheadBytes},
		{"revision bytes", revisionBytes, path.MaxRevisionBytes},
	} {
		if limit.actual > limit.max {
			return fmt.Errorf("%s %d exceeds %d", limit.name, limit.actual, limit.max)
		}
	}
	if c.UsefulBytes != 0 &&
		(!wireRatioFits(c.ResponseBytes, c.UsefulBytes, budget.MaxAmplification) ||
			!wireRatioFits(m.ApproxTokensBytes4, c.UsefulBytes, budget.MaxTokenCost)) {
		return fmt.Errorf("wire amplification or token cost exceeds budget")
	}
	for _, frame := range m.Frames {
		if budget.MaxFrameBytes != 0 && frame.Counts.ResponseBytes > budget.MaxFrameBytes {
			return fmt.Errorf("%s frame bytes %d exceed %d", frame.Delivery, frame.Counts.ResponseBytes, budget.MaxFrameBytes)
		}
	}
	return nil
}

// CheckWireCatalogBudget reads field costs from a response already verified
// by MeasureWire. The existing wire budget still caps the complete response.
func CheckWireCatalogBudget(fixture WireFixture, budget WireBudget) (*WireCatalogMeasurement, error) {
	id := fixture.Identity
	catalog := id.Route == "tools/list" && id.Outcome == "success"
	entry := (id.Route == "initialize" || id.Route == "server/discover") && id.Outcome == "success"
	if catalog != (budget.Catalog != nil) || entry != (budget.Instructions != nil) {
		return nil, fmt.Errorf("missing or unexpected catalog or instructions budget binding")
	}
	if !catalog && !entry {
		return nil, nil
	}
	if id.Source == WireSynthetic || len(fixture.Frames) != 1 || fixture.Frames[0].Body != "result" {
		return nil, fmt.Errorf("catalog and entry require one executable result frame")
	}
	frame := fixture.Frames[0]
	body, prefixOK := bytes.CutPrefix(frame.Wire, frame.Prefix)
	body, suffixOK := bytes.CutSuffix(body, frame.Suffix)
	if !prefixOK || !suffixOK {
		return nil, fmt.Errorf("invalid catalog or entry framing")
	}
	var envelope struct {
		Result struct {
			Tools []struct {
				Name         string          `json:"name"`
				Title        string          `json:"title"`
				Description  string          `json:"description"`
				InputSchema  json.RawMessage `json:"inputSchema"`
				OutputSchema json.RawMessage `json:"outputSchema"`
			} `json:"tools"`
			Instructions json.RawMessage `json:"instructions"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode catalog or entry fields: %w", err)
	}
	result := envelope.Result
	measurement := &WireCatalogMeasurement{}
	if entry {
		if result.Instructions != nil {
			var instructions *string
			if err := json.Unmarshal(result.Instructions, &instructions); err != nil || instructions == nil {
				return nil, fmt.Errorf("invalid entry instructions string")
			}
			measurement.InstructionsPresent = true
			measurement.Instructions = catalogCost(uint64(len(*instructions)))
			if *budget.Instructions == 0 || measurement.Instructions.Bytes > *budget.Instructions {
				return nil, fmt.Errorf("instructions bytes %d exceed binding %d (zero requires absence)",
					measurement.Instructions.Bytes, *budget.Instructions)
			}
		}
		return measurement, nil
	}
	if result.Tools == nil {
		return nil, fmt.Errorf("missing catalog tool array")
	}
	limit := *budget.Catalog
	seen := make(map[string]bool, len(result.Tools))
	var names, titles, descriptions, inputSchemas, outputSchemas uint64
	for _, tool := range result.Tools {
		if tool.Name == "" || seen[tool.Name] {
			return nil, fmt.Errorf("empty or duplicate catalog tool name %q", tool.Name)
		}
		seen[tool.Name] = true
		names += uint64(len(tool.Name))
		titles += uint64(len(tool.Title))
		descriptions += uint64(len(tool.Description))
		inputSchemas += uint64(len(tool.InputSchema))
		outputSchemas += uint64(len(tool.OutputSchema))
	}
	measurement.ToolCount = uint64(len(result.Tools))
	measurement.Names, measurement.Titles = catalogCost(names), catalogCost(titles)
	measurement.Descriptions = catalogCost(descriptions)
	measurement.InputSchemas, measurement.OutputSchemas = catalogCost(inputSchemas), catalogCost(outputSchemas)
	for _, field := range []struct {
		name        string
		actual, max uint64
	}{
		{"tool count", measurement.ToolCount, limit.MaxTools},
		{"name bytes", names, limit.MaxNameBytes},
		{"title bytes", titles, limit.MaxTitleBytes},
		{"description bytes", descriptions, limit.MaxDescriptionBytes},
		{"input schema bytes", inputSchemas, limit.MaxInputSchemaBytes},
		{"output schema bytes", outputSchemas, limit.MaxOutputSchemaBytes},
	} {
		if field.max == 0 {
			return nil, fmt.Errorf("invalid zero catalog limit for %s", field.name)
		}
		if field.actual > field.max {
			return nil, fmt.Errorf("catalog %s %d exceeds %d", field.name, field.actual, field.max)
		}
	}
	return measurement, nil
}

func catalogCost(bytes uint64) wireCatalogCost {
	return wireCatalogCost{Bytes: bytes, ApproxTokens: approxTokensBytes4(bytes)}
}

func consistentWireTotals(m WireMeasurement) bool {
	if m.SchemaVersion != WireMeasurementVersion || len(m.Frames) == 0 {
		return false
	}
	var total WireCounts
	observed := true
	for _, frame := range m.Frames {
		if frame.ApproxTokensBytes4 != approxTokensBytes4(frame.Counts.ResponseBytes) ||
			addWireCounts(&total, frame.Counts) != nil {
			return false
		}
		observed = observed && frame.CopiesObserved
	}
	return total == m.Counts && observed == m.CopiesObserved &&
		m.ApproxTokensBytes4 == approxTokensBytes4(total.ResponseBytes)
}

func wireRatioFits(numerator, denominator, limitMilli uint64) bool {
	actualHi, actualLo := bits.Mul64(numerator, 1000)
	limitHi, limitLo := bits.Mul64(denominator, limitMilli)
	return actualHi < limitHi || actualHi == limitHi && actualLo <= limitLo
}
