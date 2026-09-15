package main

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcp/handlers"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

var (
	readCatalogTools  = []string{"list_directory", "read_file", "get_path_info"}
	writeCatalogTools = []string{"write_file", "create_directory", "delete_path", "copy_path", "move_path"}
)

func TestEffectiveCapabilityCatalogVisibilityAndCallDenial(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		capabilities toolCapabilities
		want         []string
	}{
		{name: "no capabilities", capabilities: toolCapabilities{}, want: []string{}},
		{name: "safe read profile", capabilities: capabilitiesFromReadOnly(true), want: readCatalogTools},
		{name: "write only", capabilities: toolCapabilities{filesystemWrite: true}, want: writeCatalogTools},
		{
			name:         "read write profile",
			capabilities: capabilitiesFromReadOnly(false),
			want:         append(append([]string{}, readCatalogTools...), writeCatalogTools...),
		},
	}
	allTools := append(append([]string{}, readCatalogTools...), writeCatalogTools...)

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			registry := createToolRegistry(noopFileSystem{}, 1024, tc.capabilities)
			router := createRouter("test-server", "test-version", registry)
			got, encoded := listedToolNames(t, router)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("effective catalog\nwant: %v\n got: %v", tc.want, got)
			}

			// Repeated listing of the same effective catalog must be byte-for-byte stable.
			for attempt := 0; attempt < 3; attempt++ {
				repeated, repeatedEncoded := listedToolNames(t, router)
				if !reflect.DeepEqual(repeated, tc.want) || string(repeatedEncoded) != string(encoded) {
					t.Fatalf("tools/list changed on attempt %d\nfirst: %s\n next: %s", attempt+1, encoded, repeatedEncoded)
				}
			}

			visible := make(map[string]bool, len(tc.want))
			for _, name := range tc.want {
				visible[name] = true
			}
			for _, name := range allTools {
				_, registered := registry.Get(name)
				if registered != visible[name] {
					t.Fatalf("registry visibility for %q = %t, want %t", name, registered, visible[name])
				}
				if visible[name] {
					continue
				}

				params, err := json.Marshal(map[string]any{"name": name, "arguments": map[string]any{}})
				if err != nil {
					t.Fatal(err)
				}
				result, rpcErr := router.Dispatch(
					"tools/call",
					handlers.Context{Context: context.Background()},
					params,
				)
				if result != nil || rpcErr == nil || rpcErr.Code != protocol.ErrInvalidParams {
					t.Fatalf("call to absent tool %q = (%#v, %#v), want generic invalid params", name, result, rpcErr)
				}
			}
		})
	}
}

func listedToolNames(t *testing.T, router interface {
	Dispatch(string, handlers.Context, []byte) (any, *protocol.Error)
}) ([]string, []byte) {
	t.Helper()

	result, rpcErr := router.Dispatch("tools/list", handlers.Context{Context: context.Background()}, json.RawMessage(`{}`))
	if rpcErr != nil {
		t.Fatalf("tools/list error: %#v", rpcErr)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal tools/list result: %v", err)
	}
	var response struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatalf("decode tools/list result: %v", err)
	}
	names := make([]string, 0, len(response.Tools))
	for _, tool := range response.Tools {
		names = append(names, tool.Name)
	}
	return names, encoded
}
