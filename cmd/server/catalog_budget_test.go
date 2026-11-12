package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/benchmark"
)

func testCatalogBudgetMutations(t *testing.T) {
	budgets := loadServerWireBudgets(t)
	base := benchmark.WireIdentity{
		Source: benchmark.WireProduction, Revision: "2025-11-25", Profile: "read-only",
		Transport: "stdio-lf", Transfer: "inline", State: "complete", Outcome: "success",
	}
	registry, _ := mcpRegistryWithLimit(t, true, wireFileLimit)
	cases, err := serverWireCases(base, registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range cases {
		if row.id.Outcome != "success" || (row.id.Route != "initialize" && row.id.Route != "tools/list") {
			continue
		}
		t.Run(row.id.Route, func(t *testing.T) {
			captured, err := captureServerWire(t, row, wireGateMutation{})
			if err != nil {
				t.Fatal(err)
			}
			budget := budgets.Classes[budgets.Cases[row.id.BudgetCase()].Class]
			if row.id.Route == "initialize" {
				testEntryLimits(t, captured, budget)
				return
			}
			testCatalogLimits(t, captured, budget, budgets.Paths[base.BudgetPath()])
		})
	}
	t.Run("duplicate budget key", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "budgets.json")
		raw := `{"schema_version":"flashgate-benchmark-wire-budgets/v1","paths":{},"groups":{},"cases":{},"classes":{
			"entry":{"max_response_bytes":400,"max_result_bytes":350,"max_overhead_bytes":400,
			"max_instructions_bytes":0,"max_instructions_bytes":64}}}`
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := benchmark.LoadWireBudgets(path); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("duplicate budget key: %v", err)
		}
	})
}

func testCatalogLimits(t *testing.T, fixture benchmark.WireFixture, budget benchmark.WireBudget, path benchmark.WirePathBudget) {
	t.Helper()
	actual, err := benchmark.CheckWireCatalogBudget(fixture, budget)
	if err != nil {
		t.Fatal(err)
	}
	boundary := benchmark.WireCatalogBudget{
		MaxTools: actual.ToolCount, MaxNameBytes: actual.Names.Bytes, MaxTitleBytes: actual.Titles.Bytes,
		MaxDescriptionBytes: actual.Descriptions.Bytes, MaxInputSchemaBytes: actual.InputSchemas.Bytes,
		MaxOutputSchemaBytes: actual.OutputSchemas.Bytes,
	}
	budget.Catalog = &boundary
	if _, err := benchmark.CheckWireCatalogBudget(fixture, budget); err != nil {
		t.Fatalf("exact field ceilings: %v", err)
	}
	for _, field := range []struct {
		name  string
		limit *uint64
	}{
		{"tool count", &boundary.MaxTools}, {"name bytes", &boundary.MaxNameBytes},
		{"title bytes", &boundary.MaxTitleBytes}, {"description bytes", &boundary.MaxDescriptionBytes},
		{"input schema bytes", &boundary.MaxInputSchemaBytes}, {"output schema bytes", &boundary.MaxOutputSchemaBytes},
	} {
		*field.limit--
		expectCatalogFailure(t, fixture, budget, field.name)
		*field.limit++
	}
	measurement, err := benchmark.MeasureWire(fixture)
	if err != nil {
		t.Fatal(err)
	}
	wireLimit := budget
	wireLimit.MaxResponseBytes = measurement.Counts.ResponseBytes - 1
	if err := benchmark.CheckWireBudget(measurement, wireLimit, path); err == nil || !strings.Contains(err.Error(), "response bytes") {
		t.Fatalf("whole frame ceiling: %v", err)
	}

	var result struct {
		Tools []map[string]json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(fixture.Frames[0].Expected, &result); err != nil {
		t.Fatal(err)
	}
	original := result.Tools[0]["description"]
	var description string
	if err := json.Unmarshal(original, &description); err != nil {
		t.Fatal(err)
	}
	result.Tools[0]["description"], err = json.Marshal(description + "é")
	if err != nil {
		t.Fatal(err)
	}
	changed := replaceCatalogFixtureResult(t, fixture, result)
	expectCatalogFailure(t, changed, budget, "description bytes")
	result.Tools[0]["description"] = original
	result.Tools[0]["inputSchema"] = json.RawMessage(`{"type":"object","description":"` +
		strings.Repeat("a", int(boundary.MaxInputSchemaBytes)) + `"}`)
	changed = replaceCatalogFixtureResult(t, fixture, result)
	expectCatalogFailure(t, changed, budget, "input schema bytes")
	result.Tools[1]["name"] = result.Tools[0]["name"]
	changed = replaceCatalogFixtureResult(t, fixture, result)
	expectCatalogFailure(t, changed, budget, "duplicate catalog tool")

	budget.Catalog = nil
	expectCatalogFailure(t, fixture, budget, "budget binding")
	budget.Catalog = &boundary
	fixture.Identity.Source = benchmark.WireSynthetic
	expectCatalogFailure(t, fixture, budget, "executable result frame")
}

func testEntryLimits(t *testing.T, fixture benchmark.WireFixture, budget benchmark.WireBudget) {
	t.Helper()
	var result map[string]json.RawMessage
	if err := json.Unmarshal(fixture.Frames[0].Expected, &result); err != nil {
		t.Fatal(err)
	}
	result["instructions"] = json.RawMessage(`"é\u003c"`)
	changed := replaceCatalogFixtureResult(t, fixture, result)
	expectCatalogFailure(t, changed, budget, "instructions bytes")
	result["instructions"] = json.RawMessage(`""`)
	expectCatalogFailure(t, replaceCatalogFixtureResult(t, fixture, result), budget, "zero requires absence")
	limit := uint64(3)
	budget.Instructions = &limit
	actual, err := benchmark.CheckWireCatalogBudget(changed, budget)
	if err != nil || !actual.InstructionsPresent || actual.Instructions.Bytes != 3 || actual.Instructions.ApproxTokens != 1 {
		t.Fatalf("decoded UTF-8 instruction boundary: %+v, %v", actual, err)
	}
	result["instructions"] = json.RawMessage("null")
	expectCatalogFailure(t, replaceCatalogFixtureResult(t, fixture, result), budget, "instructions string")
	budget.Instructions = nil
	expectCatalogFailure(t, fixture, budget, "budget binding")
}

func replaceCatalogFixtureResult(t *testing.T, fixture benchmark.WireFixture, result any) benchmark.WireFixture {
	t.Helper()
	frame := fixture.Frames[0]
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	frame.Expected = raw
	frame.Wire = append(append([]byte(`{"jsonrpc":"2.0","id":1,"result":`), raw...), '}', '\n')
	fixture.Frames = []benchmark.WireFrame{frame}
	if _, err := benchmark.MeasureWire(fixture); err != nil {
		t.Fatalf("invalid mutation fixture: %v", err)
	}
	return fixture
}

func expectCatalogFailure(t *testing.T, fixture benchmark.WireFixture, budget benchmark.WireBudget, want string) {
	t.Helper()
	if _, err := benchmark.CheckWireCatalogBudget(fixture, budget); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("catalog gate returned %v, want %q", err, want)
	}
}
