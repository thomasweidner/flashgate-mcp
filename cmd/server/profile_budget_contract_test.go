package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	mcpserver "github.com/thomasweidner/flashgate-mcp/internal/mcp/server"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

const profileBudgetSchemaVersion = "flashgate-mcp-profile-budgets/v1"

type profileBudgetFile struct {
	SchemaVersion string                   `json:"schemaVersion"`
	Profiles      map[string]profileBudget `json:"profiles"`
}

type profileBudget struct {
	MaxToolCount                       int `json:"maxToolCount"`
	MaxToolsListResponseBytes          int `json:"maxToolsListResponseBytes"`
	MaxSchemaBytes                     int `json:"maxSchemaBytes"`
	MaxDescriptionBytesPerTool         int `json:"maxDescriptionBytesPerTool"`
	MaxDescriptionBytesTotal           int `json:"maxDescriptionBytesTotal"`
	MaxInitializationInstructionsBytes int `json:"maxInitializationInstructionsBytes"`
}

func TestMCPProfileBudgets(t *testing.T) {
	budgets := loadProfileBudgets(t)
	profiles := map[string]toolCapabilities{
		"read_only": capabilitiesFromReadOnly(true),
		"default":   {filesystemWrite: true},
	}
	if len(budgets.Profiles) != len(profiles) {
		t.Fatalf("budget profile count=%d, want %d", len(budgets.Profiles), len(profiles))
	}

	for name, capabilities := range profiles {
		budget, ok := budgets.Profiles[name]
		if !ok {
			t.Fatalf("profile %q has no budget", name)
		}
		validatePositiveProfileBudget(t, name, budget)
		t.Run(name, func(t *testing.T) {
			registry := createToolRegistry(noopFileSystem{}, 1024, capabilities)
			response := runProfileRequest(t, registry, "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\",\"params\":{}}\n")
			if len(response) > budget.MaxToolsListResponseBytes {
				t.Fatalf("tools/list response bytes=%d, maximum=%d", len(response), budget.MaxToolsListResponseBytes)
			}

			var envelope struct {
				Result struct {
					Tools []protocol.Tool `json:"tools"`
				} `json:"result"`
			}
			if err := json.Unmarshal(response, &envelope); err != nil {
				t.Fatal(err)
			}
			if len(envelope.Result.Tools) > budget.MaxToolCount {
				t.Fatalf("tool count=%d, maximum=%d", len(envelope.Result.Tools), budget.MaxToolCount)
			}

			schemaBytes, descriptionBytes := 0, 0
			for _, tool := range envelope.Result.Tools {
				input, err := json.Marshal(tool.InputSchema)
				if err != nil {
					t.Fatalf("marshal %s input schema: %v", tool.Name, err)
				}
				output, err := json.Marshal(tool.OutputSchema)
				if err != nil {
					t.Fatalf("marshal %s output schema: %v", tool.Name, err)
				}
				schemaBytes += len(input) + len(output)
				descriptionBytes += len([]byte(tool.Description))
				if len([]byte(tool.Description)) > budget.MaxDescriptionBytesPerTool {
					t.Fatalf("%s description bytes=%d, maximum=%d", tool.Name, len([]byte(tool.Description)), budget.MaxDescriptionBytesPerTool)
				}
			}
			if schemaBytes > budget.MaxSchemaBytes {
				t.Fatalf("schema bytes=%d, maximum=%d", schemaBytes, budget.MaxSchemaBytes)
			}
			if descriptionBytes > budget.MaxDescriptionBytesTotal {
				t.Fatalf("description bytes=%d, maximum=%d", descriptionBytes, budget.MaxDescriptionBytesTotal)
			}

			initializeResponse := runProfileRequest(t, registry, "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-11-25\",\"capabilities\":{},\"clientInfo\":{\"name\":\"budget-test\",\"version\":\"1\"}}}\n")
			var initialized struct {
				Result struct {
					Instructions string `json:"instructions"`
				} `json:"result"`
			}
			if err := json.Unmarshal(initializeResponse, &initialized); err != nil {
				t.Fatal(err)
			}
			if len([]byte(initialized.Result.Instructions)) > budget.MaxInitializationInstructionsBytes {
				t.Fatalf("initialization instructions bytes=%d, maximum=%d", len([]byte(initialized.Result.Instructions)), budget.MaxInitializationInstructionsBytes)
			}
		})
	}
}

func loadProfileBudgets(t *testing.T) profileBudgetFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "mcp-profile-budgets.json"))
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var budgets profileBudgetFile
	if err := decoder.Decode(&budgets); err != nil {
		t.Fatalf("decode profile budgets: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatalf("profile budgets trailing data: %v", err)
	}
	if budgets.SchemaVersion != profileBudgetSchemaVersion {
		t.Fatalf("profile budget schema=%q, want %q", budgets.SchemaVersion, profileBudgetSchemaVersion)
	}
	return budgets
}

func validatePositiveProfileBudget(t *testing.T, name string, budget profileBudget) {
	t.Helper()
	limits := map[string]int{
		"maxToolCount":                       budget.MaxToolCount,
		"maxToolsListResponseBytes":          budget.MaxToolsListResponseBytes,
		"maxSchemaBytes":                     budget.MaxSchemaBytes,
		"maxDescriptionBytesPerTool":         budget.MaxDescriptionBytesPerTool,
		"maxDescriptionBytesTotal":           budget.MaxDescriptionBytesTotal,
		"maxInitializationInstructionsBytes": budget.MaxInitializationInstructionsBytes,
	}
	for field, value := range limits {
		if value <= 0 {
			t.Fatalf("profile %q %s=%d, want positive", name, field, value)
		}
	}
}

func runProfileRequest(t *testing.T, registry *tools.Registry, request string) []byte {
	t.Helper()
	output := &bytes.Buffer{}
	server := mcpserver.New(bytes.NewBufferString(request), output, createRouter("test-server", "test-version", registry))
	if err := server.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
