package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/benchmark"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/revision"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
	"github.com/thomasweidner/flashgate-mcp/internal/wiretrace"
)

const wireFileLimit = 64 * 1024

type wireSnapshot struct {
	Revision  string                                `json:"revision"`
	Profile   string                                `json:"profile"`
	Entry     json.RawMessage                       `json:"entry"`
	ToolsList json.RawMessage                       `json:"toolsList"`
	Calls     map[string]map[string]json.RawMessage `json:"calls"`
}

type serverWireCase struct {
	id        benchmark.WireIdentity
	args      string
	body      string
	want      json.RawMessage
	fields    []benchmark.WireField
	reference []benchmark.WirePayload
	encode    uint64
	copyBytes uint64
	setup     func(string) error
}

type wireGateMutation struct {
	registry func(*tools.Registry) error
	cases    func([]serverWireCase) []serverWireCase
	frame    func(benchmark.WireIdentity, []byte) []byte
}

func TestMCPConformanceWireEfficiency(t *testing.T) {
	budgets := loadServerWireBudgets(t)
	if err := runServerWireGate(t, budgets, wireGateMutation{}); err != nil {
		t.Fatal(err)
	}
}

func loadServerWireBudgets(t *testing.T) benchmark.WireBudgets {
	t.Helper()
	budgets, err := benchmark.LoadWireBudgets(filepath.Join("..", "..", "benchmarks", "wire-budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	return budgets
}

func runServerWireGate(t *testing.T, budgets benchmark.WireBudgets, mutation wireGateMutation) error {
	if err := mcpMatrixCoverage(mcpConformanceVersions, mcpConformanceProfiles); err != nil {
		return err
	}
	usedPaths := make(map[string]bool)
	var cases []serverWireCase
	for _, version := range mcpConformanceVersions {
		for _, profile := range mcpConformanceProfiles {
			base := benchmark.WireIdentity{
				Source: benchmark.WireCandidate, Revision: version, Profile: profile,
				Transport: "stdio-lf", Transfer: string(tools.InlineTransfer),
				State: string(tools.CompleteResult), Outcome: "success",
			}
			if revision.ProductionPolicy().Enabled(version) {
				base.Source = benchmark.WireProduction
			}
			path := base.BudgetPath()
			if _, ok := budgets.Paths[path]; !ok || usedPaths[path] {
				return fmt.Errorf("missing or duplicate wire path %s", path)
			}
			usedPaths[path] = true
			registry, _ := mcpRegistryWithLimit(t, profile == "read-only", wireFileLimit)
			if mutation.registry != nil {
				if err := mutation.registry(registry); err != nil {
					return err
				}
			}
			scope, err := serverWireCases(base, registry)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			if mutation.cases != nil {
				scope = mutation.cases(scope)
			}
			for _, fixture := range scope {
				if fixture.id.BudgetPath() != path {
					return fmt.Errorf("fixture has mismatched revision or effective profile")
				}
			}
			cases = append(cases, scope...)
		}
	}
	// Synthetic routes share the same fixture/budget accounting. They cannot
	// satisfy a registry variant or a real revision/profile binding.
	synthetic := syntheticWireFixtures()
	for _, fixture := range synthetic {
		path := fixture.Identity.BudgetPath()
		if _, ok := budgets.Paths[path]; !ok {
			return fmt.Errorf("missing synthetic wire path %s", path)
		}
		usedPaths[path] = true
	}
	var ids []benchmark.WireIdentity
	for _, fixture := range cases {
		ids = append(ids, fixture.id)
	}
	for _, fixture := range synthetic {
		ids = append(ids, fixture.Identity)
	}
	if len(usedPaths) != len(budgets.Paths) {
		return fmt.Errorf("unexpected or unused wire path")
	}
	if err := benchmark.CheckWireCoverage(ids, budgets); err != nil {
		return err
	}
	check := func(fixture benchmark.WireFixture) error {
		id := fixture.Identity
		budget := budgets.Classes[budgets.Cases[id.BudgetCase()].Class]
		measurement, err := benchmark.MeasureWire(fixture)
		if err == nil {
			err = benchmark.CheckWireBudget(measurement, budget, budgets.Paths[id.BudgetPath()])
		}
		if err != nil {
			return fmt.Errorf("%s/%s: %w", id.BudgetPath(), id.BudgetCase(), err)
		}
		catalog, err := benchmark.CheckWireCatalogBudget(fixture, budget)
		if err != nil {
			return fmt.Errorf("%s/%s: %w", id.BudgetPath(), id.BudgetCase(), err)
		}
		t.Logf("%s/%s: %s", id.BudgetPath(), id.BudgetCase(), wireMeasurementJSON(measurement))
		if catalog != nil {
			fields, _ := json.Marshal(catalog)
			t.Logf("%s/%s catalog-fields: %s", id.BudgetPath(), id.BudgetCase(), fields)
		}
		return nil
	}
	for _, fixture := range cases {
		captured, err := captureServerWire(t, fixture, mutation)
		if err != nil {
			return fmt.Errorf("%s: %w", fixture.id.BudgetCase(), err)
		}
		if err := check(captured); err != nil {
			return err
		}
	}
	for _, fixture := range synthetic {
		if err := check(fixture); err != nil {
			return err
		}
	}
	return nil
}

func wireMeasurementJSON(m benchmark.WireMeasurement) string {
	encoded, _ := json.Marshal(m)
	return string(encoded)
}

func serverWireCases(base benchmark.WireIdentity, registry *tools.Registry) ([]serverWireCase, error) {
	var snapshot wireSnapshot
	path := filepath.Join("testdata", "mcp", base.Revision+"-"+base.Profile+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.Revision != base.Revision || snapshot.Profile != base.Profile {
		return nil, fmt.Errorf("snapshot revision or profile mismatch")
	}
	testCases := mcpToolCases()
	if err := mcpInventory(mcpDefinitions(registry), testCases, base.Profile == "read-only"); err != nil {
		return nil, err
	}
	entry, args := "initialize", `{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"contract","version":"1"}}`
	if base.Revision == revision.Stateless {
		entry, args = "server/discover", "{}"
	}
	plan := []serverWireCase{
		wireControlCase(base, entry, "entry", args, snapshot.Entry),
		wireControlCase(base, "tools/list", base.Profile, "{}", snapshot.ToolsList),
	}
	unknown := wireControlCase(base, "tools/call", "unknown-tool", `{"name":"fixture_unregistered"}`,
		json.RawMessage(`{"code":-32602,"message":"invalid params"}`))
	unknown.id.Outcome, unknown.body = "error", "error"
	plan = append(plan, unknown)
	if base.Revision == protocol.ProtocolVersion {
		plan = append(plan, wireControlCase(base, "ping", "empty-result", "{}", json.RawMessage(`{}`)))
	}
	if len(snapshot.Calls) != len(registry.List()) {
		return nil, fmt.Errorf("snapshot and registry tool counts differ")
	}
	for _, registered := range registry.List() {
		name := registered.Name()
		test := testCases[name]
		contract := registry.ResultContract(name)
		if err := mcpVariantCoverage(contract, test.success); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		outcomes := snapshot.Calls[name]
		if len(outcomes) != len(contract)+1 || len(outcomes["denied"]) == 0 {
			return nil, fmt.Errorf("stale or missing wire fixture for %s", name)
		}
		for index, variant := range contract {
			outcome := "success_" + variant.Name
			if index == 0 {
				outcome = "success"
			}
			fixture := test.success[variant.Name]
			id := base
			id.Route, id.Tool, id.Variant, id.Fixture = "tools/call", name, variant.Name, "reference"
			id.Transfer, id.State = string(variant.Mode), string(variant.State)
			id.Contract = wireVariantBinding(variant)
			row, err := wireToolCase(id, variant, fixture, outcomes[outcome])
			if err != nil {
				return nil, fmt.Errorf("%s/%s: %w", name, variant.Name, err)
			}
			plan = append(plan, row)
			if name == "read_file" && variant.Name == "complete" {
				extras, err := readFileWireCases(id, variant, outcomes[outcome])
				if err != nil {
					return nil, err
				}
				plan = append(plan, extras...)
			}
			if name == "list_directory" && variant.Name == "complete" {
				extra, err := collectionWireCase(id, variant, outcomes[outcome])
				if err != nil {
					return nil, err
				}
				plan = append(plan, extra)
			}
		}
		id := base
		id.Route, id.Tool, id.Variant, id.Fixture, id.Outcome = "tools/call", name, "error", "denied", "error"
		errorCase := serverWireCase{id: id, args: test.denied, want: outcomes["denied"], encode: 2}
		errorCase.fields = append(revisionWireFields(errorCase.want), wireField(benchmark.WireDomain, "content", "0", "text"))
		errorCase.copyBytes, err = wireCompatibilityCopyBytes(errorCase.want)
		if err != nil {
			return nil, err
		}
		plan = append(plan, errorCase)
	}
	return plan, nil
}

func wireControlCase(id benchmark.WireIdentity, route, name, args string, want json.RawMessage) serverWireCase {
	id.Route, id.Fixture = route, name
	return serverWireCase{id: id, args: args, want: want, fields: revisionWireFields(want), encode: 1}
}

func wireVariantBinding(variant tools.ResultVariant) string {
	kinds := []string{string(variant.ObjectKind)}
	for _, part := range variant.Parts {
		kinds = append(kinds, string(part))
	}
	return fmt.Sprintf("%s/%s/%s/%d/%d", strings.Join(kinds, "+"), variant.Mode, variant.State,
		variant.MaxObjectBytes, variant.MaxPayloadBytes)
}

func wireField(role benchmark.WireRole, path ...string) benchmark.WireField {
	return benchmark.WireField{Role: role, Path: path}
}

func revisionWireFields(raw json.RawMessage) []benchmark.WireField {
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	var result []benchmark.WireField
	for _, field := range []struct {
		key  string
		role benchmark.WireRole
	}{{"_meta", benchmark.WireMeta}, {"resultType", benchmark.WireResultType},
		{"ttlMs", benchmark.WireCache}, {"cacheScope", benchmark.WireCache}} {
		if fields[field.key] != nil {
			result = append(result, wireField(field.role, field.key))
		}
	}
	return result
}

func wireCompatibilityCopyBytes(raw json.RawMessage) (uint64, error) {
	var result struct {
		Content []protocol.TextContent `json:"content"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || len(result.Content) != 1 || result.Content[0].Type != "text" {
		return 0, fmt.Errorf("fixture lacks exact compatibility or error text")
	}
	return uint64(len(result.Content[0].Text)), nil
}

func wireToolCase(id benchmark.WireIdentity, variant tools.ResultVariant, test mcpSuccessCase, want json.RawMessage) (serverWireCase, error) {
	if len(want) == 0 || variant.Mode != tools.InlineTransfer || variant.State != tools.CompleteResult {
		return serverWireCase{}, fmt.Errorf("missing fixture or unsupported tool delivery")
	}
	row := serverWireCase{id: id, args: test.arguments, want: want, fields: revisionWireFields(want), encode: 2}
	if variant.ObjectKind == tools.StructuredPayload {
		value, err := json.Marshal(test.want)
		if err != nil {
			return row, err
		}
		reference, err := benchmark.CanonicalWireJSON(value)
		if err != nil {
			return row, err
		}
		class := benchmark.WireObject
		path := []string{"structuredContent"}
		if reference[0] == '[' {
			class = benchmark.WireCollection
			path = append(path, test.field)
		} else {
			var envelope map[string]json.RawMessage
			if err := json.Unmarshal(want, &envelope); err != nil {
				return row, err
			}
			reference, err = benchmark.CanonicalWireJSON(envelope["structuredContent"])
			if err != nil {
				return row, err
			}
		}
		row.reference = []benchmark.WirePayload{{Class: class, Bytes: reference, Encoding: benchmark.WireJSON}}
		row.fields = append(row.fields, wireField(benchmark.WireUseful, path...),
			wireField(benchmark.WireCompatibility, "content", "0", "text"))
		return row, nil
	}
	row.fields = append(row.fields, wireField(benchmark.WireDomain, "structuredContent"))
	if len(variant.Parts) == 0 {
		row.fields = append(row.fields, wireField(benchmark.WireCompatibility, "content", "0", "text"))
		var err error
		row.copyBytes, err = wireCompatibilityCopyBytes(want)
		return row, err
	}
	if len(variant.Parts) != len(test.content) {
		return row, fmt.Errorf("fixture has mismatched ordered payload parts")
	}
	for index, block := range test.content {
		switch part := block.(type) {
		case protocol.TextContent:
			if variant.Parts[index] != tools.TextPayload {
				return row, fmt.Errorf("unexpected text payload")
			}
			row.reference = append(row.reference, benchmark.WirePayload{Class: benchmark.WireText, Bytes: []byte(part.Text), Encoding: benchmark.WireUTF8})
			row.fields = append(row.fields, wireField(benchmark.WireUseful, "content", strconv.Itoa(index), "text"))
			row.copyBytes += uint64(len(part.Text))
		case protocol.MediaContent:
			if variant.Parts[index] != tools.BinaryPayload || (part.Type != "image" && part.Type != "audio") {
				return row, fmt.Errorf("unexpected media payload")
			}
			class := benchmark.WireImage
			if part.Type == "audio" {
				class = benchmark.WireAudio
			}
			row.reference = append(row.reference, benchmark.WirePayload{Class: class, Bytes: part.Data, Encoding: benchmark.WireBase64})
			row.fields = append(row.fields, wireField(benchmark.WireUseful, "content", strconv.Itoa(index), "data"))
		default:
			return row, fmt.Errorf("unsupported fixture content %T", block)
		}
	}
	return row, nil
}

func readFileWireCases(id benchmark.WireIdentity, variant tools.ResultVariant, template json.RawMessage) ([]serverWireCase, error) {
	var result []serverWireCase
	for _, input := range []struct{ name, text string }{
		{"64kib", strings.Repeat("x", wireFileLimit)},
		{"escaped", strings.Repeat("é𐐷<>&\\\n\t\"", 64)},
		{"empty", ""},
	} {
		id.Fixture = input.name
		content := []protocol.ContentBlock{protocol.NewTextContent(input.text)}
		want, err := replaceWireBody(template, map[string]any{"content": content, "structuredContent": map[string]int{"size": len(input.text)}})
		if err != nil {
			return nil, err
		}
		row, err := wireToolCase(id, variant, mcpSuccessCase{arguments: `{"path":"seed.txt"}`, content: content}, want)
		if err != nil {
			return nil, err
		}
		row.setup = func(root string) error {
			return os.WriteFile(filepath.Join(root, "seed.txt"), []byte(input.text), 0o600)
		}
		result = append(result, row)
	}
	return result, nil
}

func collectionWireCase(id benchmark.WireIdentity, variant tools.ResultVariant, template json.RawMessage) (serverWireCase, error) {
	id.Fixture = "500-entries"
	entries := make([]map[string]any, 500)
	for index := range entries {
		entries[index] = map[string]any{"name": fmt.Sprintf("entry-%04d.txt", index), "isDir": false, "size": 0}
	}
	want, err := replaceWireBody(template, map[string]any{"structuredContent": map[string]any{"entries": entries}})
	if err != nil {
		return serverWireCase{}, err
	}
	row, err := wireToolCase(id, variant, mcpSuccessCase{arguments: `{"path":"collection"}`, field: "entries", want: entries}, want)
	row.setup = func(root string) error {
		dir := filepath.Join(root, "collection")
		if err := os.Mkdir(dir, 0o700); err != nil {
			return err
		}
		for _, entry := range entries {
			if err := os.WriteFile(filepath.Join(dir, entry["name"].(string)), nil, 0o600); err != nil {
				return err
			}
		}
		return nil
	}
	return row, err
}

func replaceWireBody(template json.RawMessage, fields map[string]any) (json.RawMessage, error) {
	var result map[string]json.RawMessage
	if err := json.Unmarshal(template, &result); err != nil {
		return nil, err
	}
	for key, value := range fields {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		result[key] = raw
	}
	return json.Marshal(result)
}

func captureServerWire(t *testing.T, fixture serverWireCase, mutation wireGateMutation) (benchmark.WireFixture, error) {
	id := fixture.id
	registry, root := mcpRegistryWithLimit(t, id.Profile == "read-only", wireFileLimit)
	if fixture.setup != nil {
		if err := fixture.setup(root); err != nil {
			return benchmark.WireFixture{}, err
		}
	}
	if mutation.registry != nil {
		if err := mutation.registry(registry); err != nil {
			return benchmark.WireFixture{}, err
		}
	}
	var observed *mcpObservedTool
	args := fixture.args
	if id.Tool != "" {
		registered, ok := registry.Get(id.Tool)
		if !ok {
			return benchmark.WireFixture{}, fmt.Errorf("tool disappeared from effective registry")
		}
		observed = &mcpObservedTool{Tool: registered}
		if err := registry.Register(observed); err != nil {
			return benchmark.WireFixture{}, err
		}
		args = `{"name":"` + id.Tool + `","arguments":` + args + `}`
	}
	body := fixture.body
	if body == "" {
		body = "result"
	}
	frame := benchmark.WireFrame{ID: "1", Body: body, Suffix: []byte{'\n'},
		Delivery: "response", Expected: fixture.want, Fields: fixture.fields, CopiesObserved: true}
	ctx := wiretrace.Observe(context.Background(), func(event wiretrace.Event) {
		switch event.Operation {
		case wiretrace.Encode:
			frame.EncodingPasses++
		case wiretrace.Decode:
			frame.DecodingPasses++
		case wiretrace.Copy:
			frame.PayloadCopies++
			frame.ExplicitCopyBytes += uint64(event.Bytes)
		}
	})
	wire, err := mcpStdout(ctx, mcpRouter(registry, id.Source == benchmark.WireCandidate), mcpRequest(id.Route, args, id.Revision))
	if err != nil {
		return benchmark.WireFixture{}, err
	}
	if bytes.Count(wire, []byte{'\n'}) != 1 {
		return benchmark.WireFixture{}, fmt.Errorf("expected exactly one STDIO response frame")
	}
	if observed != nil && id.Outcome == "success" && observed.variant != id.Variant {
		return benchmark.WireFixture{}, fmt.Errorf("executed variant %q, want %q", observed.variant, id.Variant)
	}
	encode, decode := fixture.encode, uint64(0)
	if id.Revision == revision.Stateless && body == "result" {
		encode, decode = encode+3, 1
	}
	copies := uint64(0)
	if fixture.copyBytes != 0 {
		copies = 1
	}
	if frame.EncodingPasses != encode || frame.DecodingPasses != decode ||
		frame.PayloadCopies != copies || frame.ExplicitCopyBytes != fixture.copyBytes {
		return benchmark.WireFixture{}, fmt.Errorf("explicit serialization or payload copy trace changed: encode=%d decode=%d copies=%d copied bytes=%d",
			frame.EncodingPasses, frame.DecodingPasses, frame.PayloadCopies, frame.ExplicitCopyBytes)
	}
	if mutation.frame != nil {
		wire = mutation.frame(id, wire)
	}
	frame.Wire = wire
	return benchmark.WireFixture{Identity: id, Reference: fixture.reference, Frames: []benchmark.WireFrame{frame}}, nil
}

func TestMCPConformanceWireEfficiencyGateMutations(t *testing.T) {
	t.Run("catalog budgets", testCatalogBudgetMutations)
	for _, test := range []struct {
		name, want string
		budget     func(*benchmark.WireBudgets)
		mutation   wireGateMutation
	}{
		{"new tool", "uncovered", nil, wireGateMutation{registry: func(r *tools.Registry) error {
			return r.Register(mcpFutureTool{Tool: r.List()[0]})
		}}},
		{"new variant", "declared variants", nil, wireGateMutation{registry: func(r *tools.Registry) error {
			tool, _ := r.Get("get_path_info")
			return r.Register(wireExtraVariant{tool})
		}}},
		{"changed variant contract", "registered variant contract", nil, wireGateMutation{registry: func(r *tools.Registry) error {
			tool, _ := r.Get("get_path_info")
			return r.Register(wireChangedLimit{tool})
		}}},
		{"missing semantic fixture", "missing fixture", nil, wireGateMutation{cases: func(rows []serverWireCase) []serverWireCase {
			return removeWireCase(rows, "read_file", "escaped")
		}}},
		{"missing budget", "missing wire budget", func(b *benchmark.WireBudgets) {
			delete(b.Cases, "tools/call/read_file/complete/reference")
		}, wireGateMutation{}},
		{"invalid budget", "invalid wire budget", func(b *benchmark.WireBudgets) {
			b.Classes["entry"] = benchmark.WireBudget{}
		}, wireGateMutation{}},
		{"stale budget", "unexpected or unused", func(b *benchmark.WireBudgets) {
			b.Classes["stale"] = b.Classes["entry"]
		}, wireGateMutation{}},
		{"missing candidate profile", "missing or duplicate wire path", func(b *benchmark.WireBudgets) {
			delete(b.Paths, "candidate/2026-07-28/read-only/stdio-lf")
		}, wireGateMutation{}},
		{"unapproved optional composition", "mismatched revision or effective profile", nil, wireGateMutation{cases: func(rows []serverWireCase) []serverWireCase {
			rows[0].id.Profile = "default+process+command"
			return rows
		}}},
		{"candidate metadata on zero-revision budget", "revision bytes", func(b *benchmark.WireBudgets) {
			b.Paths["candidate/2026-07-28/default/stdio-lf"] = benchmark.WirePathBudget{}
		}, wireGateMutation{}},
		{"extra heavy copy", "payload copy trace changed", nil, wireGateMutation{registry: func(r *tools.Registry) error {
			tool, _ := r.Get("read_file")
			return r.Register(wireHeavyCopy{tool})
		}}},
		{"relocated payload", "payload placement", nil, wireGateMutation{frame: mutateReadReference(`"text":"seed"`, `"data":"seed"`)}},
		{"duplicated payload", "payload placement", nil, wireGateMutation{frame: mutateReadReference(`"size":4`, `"size":4,"text":"seed"`)}},
		{"forged size", "payload placement", nil, wireGateMutation{frame: mutateReadReference(`"size":4`, `"size":5`)}},
		{"forged same-length content", "payload placement", nil, wireGateMutation{frame: mutateReadReference(`"text":"seed"`, `"text":"fake"`)}},
		{"malformed encoding", "invalid", nil, wireGateMutation{frame: mutateReadReference(`"text":"seed"`, `"text":"\x"`)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			budgets := loadServerWireBudgets(t)
			if test.budget != nil {
				test.budget(&budgets)
			}
			if err := runServerWireGate(t, budgets, test.mutation); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("gate returned %v, want %q", err, test.want)
			}
		})
	}
}

func removeWireCase(rows []serverWireCase, tool, name string) []serverWireCase {
	return slices.DeleteFunc(rows, func(row serverWireCase) bool {
		return row.id.Tool == tool && row.id.Fixture == name
	})
}

func mutateReadReference(before, after string) func(benchmark.WireIdentity, []byte) []byte {
	return func(id benchmark.WireIdentity, data []byte) []byte {
		if id.Tool == "read_file" && id.Fixture == "reference" {
			return bytes.Replace(data, []byte(before), []byte(after), 1)
		}
		return data
	}
}

type wireExtraVariant struct{ tools.Tool }

func (tool wireExtraVariant) ResultContract() tools.ResultContract {
	contract := tool.Tool.ResultContract()
	extra := contract[0]
	extra.Name = "new_variant"
	return append(contract, extra)
}

type wireHeavyCopy struct{ tools.Tool }

func (tool wireHeavyCopy) Execute(ctx context.Context, arguments json.RawMessage) (tools.ToolResult, error) {
	copy := wiretrace.Text(ctx, bytes.Repeat([]byte{'x'}, wireFileLimit))
	if len(copy) != wireFileLimit {
		return tools.ToolResult{}, fmt.Errorf("mutation payload was truncated")
	}
	return tool.Tool.Execute(ctx, arguments)
}

type wireChangedLimit struct{ tools.Tool }

func (tool wireChangedLimit) ResultContract() tools.ResultContract {
	contract := tool.Tool.ResultContract()
	contract[0].MaxObjectBytes--
	return contract
}

func syntheticWireFixtures() []benchmark.WireFixture {
	id := benchmark.WireIdentity{
		Source: benchmark.WireSynthetic, Revision: revision.Stateless, Profile: "fixture",
		Transport: "synthetic-lf", Transfer: "inline", State: "complete", Outcome: "success",
	}
	text := func(path ...string) benchmark.WireField { return wireField(benchmark.WireUseful, path...) }
	response := func(delivery, body string, fields ...benchmark.WireField) benchmark.WireFrame {
		return benchmark.WireFrame{
			Wire: []byte(`{"jsonrpc":"2.0","id":1,"result":` + body + "}\n"), Suffix: []byte{'\n'},
			ID: "1", Body: "result", Delivery: delivery, Expected: json.RawMessage(body), Fields: fields,
		}
	}
	makeFixture := func(route, transfer, outcome string, refs []benchmark.WirePayload, frames ...benchmark.WireFrame) benchmark.WireFixture {
		binding := id
		binding.Route, binding.Transfer, binding.Fixture, binding.Outcome = route, transfer, "reference", outcome
		return benchmark.WireFixture{Identity: binding, Reference: refs, Frames: frames}
	}
	firstRange := text("data")
	firstRange.Continue = true
	event := benchmark.WireFrame{
		Wire:   []byte("data: {\"jsonrpc\":\"2.0\",\"method\":\"fixture/event\",\"params\":{\"items\":[1,2]}}\n\n"),
		Prefix: []byte("data: "), Suffix: []byte("\n\n"), Body: "params", Method: "fixture/event",
		Delivery: "event-batch", Expected: json.RawMessage(`{"items":[1,2]}`), Fields: []benchmark.WireField{text("items")},
	}
	link := response("resource-link", `{"uri":"flashgate://result/fixture"}`, wireField(benchmark.WireDomain, "uri"))
	chunks := makeFixture("fixture/chunk", "resource", "success", []benchmark.WirePayload{{Class: benchmark.WireText, Bytes: []byte("left/right"), Encoding: benchmark.WireUTF8}},
		response("chunk", `{"data":"left/","offset":0}`, firstRange),
		response("chunk", `{"data":"right","offset":5}`, text("data")))
	chunks.Frames[0].State = "partial"
	page := makeFixture("fixture/page", "paged", "success",
		[]benchmark.WirePayload{{Class: benchmark.WireCollection, Bytes: []byte("[1,2]"), Encoding: benchmark.WireJSON}, {Class: benchmark.WireCollection, Bytes: []byte("[3]"), Encoding: benchmark.WireJSON}},
		response("page", `{"entries":[1,2],"cursor":"next"}`, text("entries"), wireField(benchmark.WireCursor, "cursor")),
		response("page", `{"entries":[3]}`, text("entries")))
	page.Frames[0].State = "partial"
	resourceLink := makeFixture("fixture/link", "resource", "success", nil, link)
	resourceLink.Identity.State = "pending"
	eventBatch := makeFixture("fixture/event", "inline", "success",
		[]benchmark.WirePayload{{Class: benchmark.WireCollection, Bytes: []byte("[1,2]"), Encoding: benchmark.WireJSON}}, event)
	eventBatch.Identity.Transport = "synthetic-event"
	return []benchmark.WireFixture{
		page,
		resourceLink,
		makeFixture("fixture/resource-read", "resource", "success",
			[]benchmark.WirePayload{{Class: benchmark.WireBinary, Bytes: []byte{0, 1, 2, 3}, Encoding: benchmark.WireBase64}},
			link, response("resource-read", `{"mimeType":"application/octet-stream","data":"AAECAw=="}`, text("data"))),
		chunks,
		eventBatch,
		makeFixture("fixture/fallback", "inline", "fallback",
			[]benchmark.WirePayload{{Class: benchmark.WireObject, Bytes: []byte(`{"x":1}`), Encoding: benchmark.WireJSON}},
			response("fallback", `{"object":{"x":1},"text":"{\"x\":1}"}`, text("object"), wireField(benchmark.WireCompatibility, "text"))),
		makeFixture("fixture/media", "inline", "success",
			[]benchmark.WirePayload{{Class: benchmark.WireText, Bytes: []byte("é<"), Encoding: benchmark.WireUTF8}, {Class: benchmark.WireImage, Bytes: []byte{1, 2, 3}, Encoding: benchmark.WireBase64}, {Class: benchmark.WireAudio, Bytes: []byte{4}, Encoding: benchmark.WireBase64}},
			response("multipart", `{"content":[{"text":"é\u003c"},{"mimeType":"image/png","data":"AQID"},{"mimeType":"audio/wav","data":"BA=="}]}`,
				text("content", "0", "text"), text("content", "1", "data"), text("content", "2", "data"))),
	}
}
