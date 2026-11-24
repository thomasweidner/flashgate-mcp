package fsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/execution"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/resource"
)

type observedPairFilesystem struct {
	fs.FileSystem
	lookups int
}

func (f *observedPairFilesystem) LimitsForRoot(id config.RootID) (resource.Limits, error) {
	f.lookups++
	return f.FileSystem.LimitsForRoot(id)
}

func TestPairToolAuthorizationPrecedesRootLookup(t *testing.T) {
	for _, profileName := range []string{"read", "write"} {
		t.Run(profileName, func(t *testing.T) {
			roots, profile, _ := directFixture(t, profileName)
			runtime, err := execution.NewRuntime("pair-authorization-test")
			if err != nil {
				t.Fatal(err)
			}
			session, identity, err := execution.BindDirectSTDIO(runtime)
			if err != nil {
				t.Fatal(err)
			}
			operations, err := fs.BindOperations(profile.Catalog())
			if err != nil {
				t.Fatal(err)
			}
			routes, err := bindRoutes(roots, operations)
			if err != nil {
				t.Fatal(err)
			}
			resolutions, backendCalls, fallbackCalls := 0, 0, 0
			resolve := routes.Resolve
			routes.Resolve = func(ctx context.Context, c command) ([]execution.Resource, error) {
				resolutions++
				return resolve(ctx, c)
			}
			backend := simulatedBackend(func(context.Context, execution.Invocation[command]) (execution.Completion[result], error) {
				backendCalls++
				return execution.Completion[result]{}, nil
			})
			fallback := simulatedBackend(func(context.Context, execution.Invocation[command]) (execution.Completion[result], error) {
				fallbackCalls++
				return execution.Completion[result]{}, nil
			})
			dispatcher, err := execution.NewDispatcher(runtime, profile, execution.CurrentProcess, routes,
				execution.Registration[command, result]{ID: execution.CurrentProcess, Identity: identity, Backend: backend},
				execution.Registration[command, result]{ID: execution.UserWorker, Identity: identity, Backend: fallback})
			if err != nil {
				t.Fatal(err)
			}
			app := Service{roots: roots, session: session, dispatcher: dispatcher}
			filesystem, err := app.ForRequest(t.Context(), execution.MCP20251125)
			if err != nil {
				t.Fatal(err)
			}
			observed := &observedPairFilesystem{FileSystem: filesystem}
			registry := tools.NewRegistry()
			for _, tool := range []tools.Tool{tools.NewCopyPathTool(observed), tools.NewMovePathTool(observed)} {
				if err := registry.RegisterBound(tool, func(ctx handlers.Context, arguments json.RawMessage) (tools.ToolResult, error) {
					return tool.Execute(ctx.Context, arguments)
				}); err != nil {
					t.Fatal(err)
				}
			}
			registry.Seal()
			handler := tools.NewCallHandler(registry)
			for _, operation := range []string{"copy_path", "move_path"} {
				for _, pair := range []struct {
					source, target, allowedCategory string
				}{
					{"missing", "work", "invalid_arguments"},
					{"work", "missing", "invalid_arguments"},
					{"work", "reader", "unsupported_operation"},
				} {
					t.Run(operation+"/"+pair.source+"-"+pair.target, func(t *testing.T) {
						resolutions, backendCalls, fallbackCalls, observed.lookups = 0, 0, 0, 0
						category, wantResolutions := pair.allowedCategory, 1
						if profileName == "read" {
							category, wantResolutions = "access_denied", 0
						}
						params := fmt.Appendf(nil, `{"name":%q,"arguments":{"source_root_id":%q,"source":"seed.txt","target_root_id":%q,"target":"target.txt"}}`,
							operation, pair.source, pair.target)
						value, rpcErr := handler.Handle(handlers.Context{Context: t.Context()}, params)
						if rpcErr != nil {
							t.Fatalf("unexpected protocol error: %v", rpcErr)
						}
						raw, err := json.Marshal(value)
						if err != nil {
							t.Fatal(err)
						}
						decoded, err := mcptest.DecodeCallToolResult(raw)
						if err != nil || !decoded.IsError || decoded.HasStructuredContent {
							t.Fatalf("unexpected error result: %s (%v)", raw, err)
						}
						body, ok := decoded.TextContent.(map[string]any)
						if !ok || body["category"] != category || resolutions != wantResolutions ||
							observed.lookups != 0 || backendCalls != 0 || fallbackCalls != 0 {
							t.Fatalf("wanted %s and %d resolutions, got %s, resolutions=%d, adapter lookups=%d, backend=%d, fallback=%d",
								category, wantResolutions, raw, resolutions, observed.lookups, backendCalls, fallbackCalls)
						}
					})
				}
			}
		})
	}
}
