package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	benchmarkrunner "github.com/thomasweidner/flashgate-mcp/internal/benchmark"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

var benchmarkResultSink []byte
var benchmarkHandlerResultSink any

type textOnlyCallToolResult struct {
	Content []protocol.TextContent `json:"content"`
}

func BenchmarkCallToolResultSerialization(b *testing.B) {
	fixtures := callToolResultBenchmarkFixtures()
	variants := []struct {
		name      string
		serialize func(any) ([]byte, error)
	}{
		{"historical_direct", json.Marshal},
		{"text_only", serializeTextOnlyCallToolResult},
		{"text_plus_structured", serializeStructuredCallToolResult},
	}

	for _, fixture := range fixtures {
		fixture := fixture
		b.Run(fixture.name, func(b *testing.B) {
			for _, variant := range variants {
				variant := variant
				b.Run(variant.name, func(b *testing.B) {
					sample, err := variant.serialize(fixture.value)
					if err != nil {
						b.Fatal(err)
					}
					response, err := serializeBenchmarkResponse(sample)
					if err != nil {
						b.Fatal(err)
					}
					b.ReportAllocs()
					b.SetBytes(int64(len(sample)))
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						benchmarkResultSink, err = variant.serialize(fixture.value)
						if err != nil {
							b.Fatal(err)
						}
					}
					b.StopTimer()
					b.ReportMetric(float64(len(sample)), "payload-bytes")
					b.ReportMetric(float64(len(response)), "response-bytes")
				})
			}
		})
	}
}

func BenchmarkCallToolHandlerProcessing(b *testing.B) {
	for _, fixture := range callToolHandlerBenchmarkFixtures() {
		fixture := fixture
		b.Run(fixture.name, func(b *testing.B) {
			sample, rpcErr := fixture.handler.Handle(
				handlers.Context{Context: context.Background()},
				fixture.params,
			)
			if rpcErr != nil {
				b.Fatalf("sample tools/call failed: %s", rpcErr.Message)
			}
			encoded, err := json.Marshal(sample)
			if err != nil {
				b.Fatal(err)
			}
			response, err := serializeBenchmarkResponse(encoded)
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(encoded)))
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				benchmarkHandlerResultSink, rpcErr = fixture.handler.Handle(
					handlers.Context{Context: context.Background()},
					fixture.params,
				)
				if rpcErr != nil {
					b.Fatalf("tools/call failed: %s", rpcErr.Message)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(len(encoded)), "result-bytes")
			b.ReportMetric(float64(len(response)), "response-bytes")
		})
	}
}

func BenchmarkCallToolHandlerProcessingParallel(b *testing.B) {
	registry := NewRegistry()
	registry.Register(NewGetPathInfoTool(benchmarkParallelFileSystem{}))
	handler := NewCallHandler(registry)
	params := json.RawMessage(`{"name":"get_path_info","arguments":{"path":"existing.txt"}}`)
	sample, rpcErr := handler.Handle(handlers.Context{Context: context.Background()}, params)
	if rpcErr != nil {
		b.Fatalf("sample tools/call failed: %s", rpcErr.Message)
	}
	encoded, err := json.Marshal(sample)
	if err != nil {
		b.Fatal(err)
	}
	response, err := serializeBenchmarkResponse(encoded)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(encoded)))
	var failureOnce sync.Once
	var failureMessage string
	b.ResetTimer()
	b.RunParallel(func(parallel *testing.PB) {
		for parallel.Next() {
			result, rpcErr := handler.Handle(handlers.Context{Context: context.Background()}, params)
			if rpcErr != nil {
				failureOnce.Do(func() { failureMessage = rpcErr.Message })
				continue
			}
			runtime.KeepAlive(result)
		}
	})
	if failureMessage != "" {
		b.Fatalf("tools/call failed: %s", failureMessage)
	}
	b.StopTimer()
	b.ReportMetric(float64(len(encoded)), "result-bytes")
	b.ReportMetric(float64(len(response)), "response-bytes")
}

type benchmarkParallelFileSystem struct{}

func (benchmarkParallelFileSystem) List(string) ([]fs.Entry, error)    { return nil, nil }
func (benchmarkParallelFileSystem) Read(string, int64) ([]byte, error) { return nil, nil }
func (benchmarkParallelFileSystem) Stat(string) (fs.Metadata, error) {
	return fs.Metadata{Name: "existing.txt", Size: 26}, nil
}
func (benchmarkParallelFileSystem) Write(string, []byte, bool) error { return nil }
func (benchmarkParallelFileSystem) Mkdir(string) (bool, error)       { return false, nil }
func (benchmarkParallelFileSystem) Delete(string, bool) error        { return nil }
func (benchmarkParallelFileSystem) Move(string, string, bool) error  { return nil }
func (benchmarkParallelFileSystem) Copy(string, string, bool) error  { return nil }

type callToolHandlerBenchmarkFixture struct {
	name    string
	handler *CallHandler
	params  json.RawMessage
}

func callToolHandlerBenchmarkFixtures() []callToolHandlerBenchmarkFixture {
	result := make([]callToolHandlerBenchmarkFixture, 0, len(callToolResultBenchmarkFixtures()))
	for _, fixture := range callToolResultBenchmarkFixtures() {
		filesystem := newFakeFileSystem()
		registry := NewRegistry()
		var params json.RawMessage

		switch value := fixture.value.(type) {
		case getPathInfoMissingResult:
			filesystem.statErr = fs.ErrNotFound
			registry.Register(NewGetPathInfoTool(filesystem))
			params = json.RawMessage(`{"name":"get_path_info","arguments":{"path":"missing.txt"}}`)
		case getPathInfoExistingResult:
			filesystem.statMetadata = fs.Metadata{Name: value.Name, IsDir: value.IsDir, Size: value.Size}
			registry.Register(NewGetPathInfoTool(filesystem))
			params = json.RawMessage(`{"name":"get_path_info","arguments":{"path":"docs/readme.txt"}}`)
		case listDirectoryResult:
			filesystem.entries = make([]fs.Entry, 0, len(value.Entries))
			for _, entry := range value.Entries {
				filesystem.entries = append(filesystem.entries, fs.Entry{Name: *entry.Name, IsDir: *entry.IsDir, Size: *entry.Size})
			}
			registry.Register(NewListDirectoryTool(filesystem))
			params = json.RawMessage(`{"name":"list_directory","arguments":{"path":"fixtures"}}`)
		case readFileResult:
			filesystem.readContent = []byte(value.Content)
			registry.Register(NewReadFileTool(filesystem, 64*1024))
			params = json.RawMessage(`{"name":"read_file","arguments":{"path":"fixture.txt"}}`)
		default:
			panic(fmt.Sprintf("unsupported benchmark fixture %T", fixture.value))
		}

		result = append(result, callToolHandlerBenchmarkFixture{
			name:    fixture.name,
			handler: NewCallHandler(registry),
			params:  params,
		})
	}
	return result
}

func serializeTextOnlyCallToolResult(value any) ([]byte, error) {
	structured, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(textOnlyCallToolResult{
		Content: []protocol.TextContent{protocol.NewTextContent(string(structured))},
	})
}

func serializeStructuredCallToolResult(value any) ([]byte, error) {
	wrapped, rpcErr := wrapSuccessfulToolResult(value)
	if rpcErr != nil {
		return nil, fmt.Errorf("wrap result: %s", rpcErr.Message)
	}
	return json.Marshal(wrapped)
}

func serializeBenchmarkResponse(result []byte) ([]byte, error) {
	response, err := json.Marshal(protocol.Response{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`1`),
		Result:  json.RawMessage(result),
	})
	if err != nil {
		return nil, err
	}
	return append(response, '\n'), nil
}

type callToolResultBenchmarkFixture struct {
	name  string
	value any
}

func callToolResultBenchmarkFixtures() []callToolResultBenchmarkFixture {
	largeEntries := make([]fs.Entry, 500)
	for index := range largeEntries {
		largeEntries[index] = fs.Entry{
			Name:  fmt.Sprintf("entry-%04d.txt", index),
			Size:  int64(index * 17),
			IsDir: index%10 == 0,
		}
	}

	return []callToolResultBenchmarkFixture{
		{"path_info_missing", getPathInfoMissingResult{Path: "does-not-exist.txt", Exists: false}},
		{"path_info_existing", getPathInfoExistingResult{Path: "docs/readme.txt", Exists: true, Name: "readme.txt", Size: 128}},
		{"directory_small", listDirectoryResult{Entries: projectListDirectoryEntries([]fs.Entry{{Name: "a.txt", Size: 12}, {Name: "sub", IsDir: true}}, listDirectoryFields{name: true, isDir: true, size: true})}},
		{"directory_500_entries", listDirectoryResult{Entries: projectListDirectoryEntries(largeEntries, listDirectoryFields{name: true, isDir: true, size: true})}},
		{"text_file_small", readFileResult{Content: "FlashGate benchmark text.\n", Size: 26}},
		{"text_file_64kib", readFileResult{Content: strings.Repeat("x", 64*1024), Size: 64 * 1024}},
	}
}

func TestCallToolResultSerializationPayloadSizes(t *testing.T) {
	expected := map[string][3]int{
		"path_info_missing":     {44, 89, 154},
		"path_info_existing":    {85, 138, 244},
		"directory_small":       {91, 148, 260},
		"directory_500_entries": {25897, 29938, 55856},
		"text_file_small":       {51, 97, 169},
		"text_file_64kib":       {65563, 65608, 131192},
	}
	variants := []struct {
		name      string
		serialize func(any) ([]byte, error)
	}{
		{"historical_direct", json.Marshal},
		{"text_only", serializeTextOnlyCallToolResult},
		{"text_plus_structured", serializeStructuredCallToolResult},
	}

	for _, fixture := range callToolResultBenchmarkFixtures() {
		for variantIndex, variant := range variants {
			fixture := fixture
			variant := variant
			t.Run(fixture.name+"/"+variant.name, func(t *testing.T) {
				result, err := variant.serialize(fixture.value)
				if err != nil {
					t.Fatal(err)
				}
				wantResult := expected[fixture.name][variantIndex]
				if len(result) != wantResult {
					t.Fatalf("result bytes=%d, want %d", len(result), wantResult)
				}
				response, err := serializeBenchmarkResponse(result)
				if err != nil {
					t.Fatal(err)
				}
				if len(response) != wantResult+35 {
					t.Fatalf("response bytes=%d, want %d", len(response), wantResult+35)
				}
			})
		}
	}
}

func TestCallToolResultSerializationBudgets(t *testing.T) {
	budgetPath := filepath.Join("..", "..", "..", "benchmarks", "budgets.json")
	budgets, err := benchmarkrunner.LoadSerializationBudgets(budgetPath)
	if err != nil {
		t.Fatal(err)
	}

	fixtures := callToolResultBenchmarkFixtures()
	seenFixtures := make(map[string]struct{}, len(fixtures))
	usedBudgets := make(map[string]struct{}, len(fixtures))
	for _, fixture := range fixtures {
		if fixture.name == "" {
			t.Fatal("serialization fixture has empty name")
		}
		if _, ok := seenFixtures[fixture.name]; ok {
			t.Fatalf("duplicate serialization fixture %q", fixture.name)
		}
		seenFixtures[fixture.name] = struct{}{}

		budgetName := fixture.name + "_text_plus_structured"
		budget, ok := budgets[budgetName]
		if !ok {
			t.Fatalf("serialization fixture %q has no budget", fixture.name)
		}
		usedBudgets[budgetName] = struct{}{}
		if budget.MaxPayloadBytes == 0 || budget.MaxAllocsPerOp == 0 {
			t.Fatalf("serialization budget %q is structurally incomplete", budgetName)
		}

		payload, err := serializeStructuredCallToolResult(fixture.value)
		if err != nil {
			t.Fatal(err)
		}
		if uint64(len(payload)) > budget.MaxPayloadBytes {
			t.Fatalf("serialization fixture %q payload=%d exceeds budget %d", fixture.name, len(payload), budget.MaxPayloadBytes)
		}

	}
	for budgetName := range budgets {
		if _, ok := usedBudgets[budgetName]; !ok {
			t.Fatalf("unknown serialization budget fixture %q", budgetName)
		}
	}
	if len(usedBudgets) != len(budgets) {
		t.Fatalf("serialization fixtures=%d budgets=%d", len(usedBudgets), len(budgets))
	}
}

func TestCallToolResultSerializationAllocationBudgets(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("Allocation budgets are evaluated only without race instrumentation because the race detector changes allocation behavior. Functional serialization coverage remains active under race.")
	}

	budgetPath := filepath.Join("..", "..", "..", "benchmarks", "budgets.json")
	budgets, err := benchmarkrunner.LoadSerializationBudgets(budgetPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, fixture := range callToolResultBenchmarkFixtures() {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			budgetName := fixture.name + "_text_plus_structured"
			budget, ok := budgets[budgetName]
			if !ok {
				t.Fatalf("serialization fixture %q has no budget", fixture.name)
			}

			var allocationErr error
			allocations := testing.AllocsPerRun(100, func() {
				result, serializeErr := serializeStructuredCallToolResult(fixture.value)
				if serializeErr != nil {
					allocationErr = serializeErr
				}
				runtime.KeepAlive(result)
			})
			if allocationErr != nil {
				t.Fatal(allocationErr)
			}
			t.Logf("allocations/op=%.0f budget=%d", allocations, budget.MaxAllocsPerOp)
			if allocations > float64(budget.MaxAllocsPerOp) {
				t.Fatalf("serialization fixture %q allocations/op=%.0f exceeds budget %d", fixture.name, allocations, budget.MaxAllocsPerOp)
			}
		})
	}
}
