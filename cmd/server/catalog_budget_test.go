package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	mcpserver "github.com/thomasweidner/flashgate-mcp/internal/mcp/server"
	mcptools "github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const catalogBudgetSchemaVersion = "flashgate-catalog-budgets/v1"

type catalogBudgetFile struct {
	SchemaVersion string                 `json:"schema_version"`
	Profiles      []catalogProfileBudget `json:"profiles"`
}

type catalogProfileBudget struct {
	Name                        string `json:"name"`
	ToolCount                   int    `json:"tool_count"`
	MaxSchemaBytes              int    `json:"max_schema_bytes"`
	MaxSchemaApproxTokens       int    `json:"max_schema_approx_tokens"`
	MaxToolsListBytes           int    `json:"max_tools_list_bytes"`
	MaxToolsListApproxTokens    int    `json:"max_tools_list_approx_tokens"`
	CatalogFingerprintSHA256    string `json:"catalog_fingerprint_sha256"`
	MaxInstructionsBytes        int    `json:"max_instructions_bytes"`
	MaxInstructionsApproxTokens int    `json:"max_instructions_approx_tokens"`
	MaxInitializeBytes          int    `json:"max_initialize_bytes"`
	MaxInitializeApproxTokens   int    `json:"max_initialize_approx_tokens"`
}

type catalogMeasurement struct {
	toolCount                int
	schemaBytes              int
	schemaApproxTokens       int
	toolsListBytes           int
	toolsListApproxTokens    int
	catalogFingerprintSHA256 string
	instructionsBytes        int
	instructionsApproxTokens int
	initializeBytes          int
	initializeApproxTokens   int
}

func TestProfileCatalogAndInitializationBudgets(t *testing.T) {
	budgets := loadCatalogBudgets(t)
	wantProfiles := []string{"read_only", "default"}
	if len(budgets.Profiles) != len(wantProfiles) {
		t.Fatalf("catalog budget profiles=%d, want %d", len(budgets.Profiles), len(wantProfiles))
	}

	for index, budget := range budgets.Profiles {
		if budget.Name != wantProfiles[index] {
			t.Fatalf("catalog budget profile[%d]=%q, want %q", index, budget.Name, wantProfiles[index])
		}
		capabilities := capabilitiesFromReadOnly(budget.Name == "read_only")
		t.Run(budget.Name, func(t *testing.T) {
			measurement := measureCatalogProfile(t, capabilities)
			t.Logf("catalog measurement: %+v", measurement)
			if measurement.toolCount != budget.ToolCount {
				t.Errorf("tool count=%d, want %d", measurement.toolCount, budget.ToolCount)
			}
			checkCatalogMaximum(t, "schema bytes", measurement.schemaBytes, budget.MaxSchemaBytes)
			checkCatalogMaximum(t, "schema approximate tokens", measurement.schemaApproxTokens, budget.MaxSchemaApproxTokens)
			checkCatalogMaximum(t, "tools/list bytes", measurement.toolsListBytes, budget.MaxToolsListBytes)
			checkCatalogMaximum(t, "tools/list approximate tokens", measurement.toolsListApproxTokens, budget.MaxToolsListApproxTokens)
			if measurement.catalogFingerprintSHA256 != budget.CatalogFingerprintSHA256 {
				t.Errorf("catalog fingerprint=%s, want %s", measurement.catalogFingerprintSHA256, budget.CatalogFingerprintSHA256)
			}
			checkCatalogMaximum(t, "instructions bytes", measurement.instructionsBytes, budget.MaxInstructionsBytes)
			checkCatalogMaximum(t, "instructions approximate tokens", measurement.instructionsApproxTokens, budget.MaxInstructionsApproxTokens)
			checkCatalogMaximum(t, "initialize bytes", measurement.initializeBytes, budget.MaxInitializeBytes)
			checkCatalogMaximum(t, "initialize approximate tokens", measurement.initializeApproxTokens, budget.MaxInitializeApproxTokens)
		})
	}
}

func loadCatalogBudgets(t *testing.T) catalogBudgetFile {
	t.Helper()
	path := filepath.Join("..", "..", "benchmarks", "catalog-budgets.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var budgets catalogBudgetFile
	if err := decoder.Decode(&budgets); err != nil {
		t.Fatalf("decode catalog budgets: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatalf("catalog budgets must contain exactly one JSON value: %v", err)
	}
	if budgets.SchemaVersion != catalogBudgetSchemaVersion {
		t.Fatalf("catalog budget schema=%q, want %q", budgets.SchemaVersion, catalogBudgetSchemaVersion)
	}
	for _, profile := range budgets.Profiles {
		if profile.Name == "" || profile.ToolCount <= 0 || profile.MaxSchemaBytes <= 0 || profile.MaxToolsListBytes <= 0 || profile.MaxInitializeBytes <= 0 {
			t.Fatalf("catalog budget profile %q has an empty or non-positive required value", profile.Name)
		}
		fingerprint, err := hex.DecodeString(profile.CatalogFingerprintSHA256)
		if err != nil || len(fingerprint) != sha256.Size {
			t.Fatalf("catalog budget profile %q has an invalid SHA-256 fingerprint", profile.Name)
		}
	}
	return budgets
}

func measureCatalogProfile(t *testing.T, capabilities toolCapabilities) catalogMeasurement {
	t.Helper()
	registry := createToolRegistry(noopFileSystem{}, 1024, capabilities)
	toolsList := runCatalogRequest(t, registry, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	initialize := runCatalogRequest(t, registry, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"budget-test","version":"1"}}}`)

	var listed struct {
		Result struct {
			Tools []protocol.Tool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(toolsList, &listed); err != nil {
		t.Fatalf("decode tools/list response: %v", err)
	}
	if len(listed.Result.Tools) != len(registry.List()) {
		t.Fatalf("tools/list returned %d tools for %d registry entries", len(listed.Result.Tools), len(registry.List()))
	}
	for index, tool := range listed.Result.Tools {
		if tool.Name != registry.List()[index].Name() {
			t.Fatalf("tools/list order[%d]=%q, want %q", index, tool.Name, registry.List()[index].Name())
		}
	}

	schemaBytes := 0
	for _, tool := range listed.Result.Tools {
		input, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		output, err := json.Marshal(tool.OutputSchema)
		if err != nil {
			t.Fatal(err)
		}
		schemaBytes += len(input) + len(output)
	}
	catalog, err := json.Marshal(listed.Result)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := sha256.Sum256(catalog)

	var initialized struct {
		Result struct {
			Instructions string `json:"instructions,omitempty"`
		} `json:"result"`
	}
	if err := json.Unmarshal(initialize, &initialized); err != nil {
		t.Fatalf("decode initialize response: %v", err)
	}
	instructionsBytes := len([]byte(initialized.Result.Instructions))

	return catalogMeasurement{
		toolCount:                len(listed.Result.Tools),
		schemaBytes:              schemaBytes,
		schemaApproxTokens:       approximateTokens(schemaBytes),
		toolsListBytes:           len(toolsList),
		toolsListApproxTokens:    approximateTokens(len(toolsList)),
		catalogFingerprintSHA256: hex.EncodeToString(fingerprint[:]),
		instructionsBytes:        instructionsBytes,
		instructionsApproxTokens: approximateTokens(instructionsBytes),
		initializeBytes:          len(initialize),
		initializeApproxTokens:   approximateTokens(len(initialize)),
	}
}

func runCatalogRequest(t *testing.T, registry *mcptools.Registry, request string) []byte {
	t.Helper()
	output := &bytes.Buffer{}
	server := mcpserver.New(
		bytes.NewBufferString(request+"\n"),
		output,
		createRouter("flashgate", "budget-test", registry),
	)
	if err := server.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSuffix(output.Bytes(), []byte("\n"))
}

func approximateTokens(bytes int) int {
	return (bytes + 3) / 4
}

func checkCatalogMaximum(t *testing.T, name string, got, maximum int) {
	t.Helper()
	if maximum < 0 {
		t.Fatalf("%s budget must not be negative", name)
	}
	if got > maximum {
		t.Errorf("%s=%d exceeds %d", name, got, maximum)
	}
}
