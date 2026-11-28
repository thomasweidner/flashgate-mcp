package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestProductionAuditFailurePolicy(t *testing.T) {
	for _, tc := range []struct {
		mode     string
		required bool
		healthy  bool
	}{
		{"operation_first", false, true},
		{"operation_first", false, false},
		{"audit_required", true, true},
		{"audit_required", true, false},
	} {
		t.Run(fmt.Sprintf("%s/healthy=%v", tc.mode, tc.healthy), func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "flashgate_config.toml")
			store := filepath.Join(root, "audit")
			if !tc.healthy {
				testconfig.Write(t, store, []byte("owned fixture of the wrong type"))
			}
			settings := "work.access = ['files.read', 'files.write']\n" + profileSettings(false) +
				"[audit]\nfailure_policy = '" + tc.mode + "'\n"
			if tc.required {
				settings += "required_operations = ['filesystem.write']\nrequired_intent_timeout = '5 s'\n"
			}
			testconfig.Write(t, source, []byte(configDocument(t, root, settings)))
			deps := defaultBootstrapDependencies(source)
			cfg, err := deps.loadConfig()
			if err != nil {
				t.Fatal(err)
			}
			roots, err := deps.newRoots(cfg)
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			registry, lifecycle, err := createAuditedToolRegistry(roots, cfg, serverOptionsFromConfig(cfg, &stderr).Diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			defer lifecycle.stop()
			if err := admitProfile(registry); err != nil {
				t.Fatal(err)
			}
			lifecycle.start()
			response, raw := runCallToolWireRequest(t, registry,
				`{"name":"write_file","arguments":{"root_id":"work","path":"write.txt","content":"product-payload-sentinel"}}`)
			result, err := mcptest.DecodeCallToolResult(response.Result)
			denied := tc.required && !tc.healthy
			if err != nil || result.IsError != denied {
				t.Fatalf("unexpected execution result: %s (%v)", raw, err)
			}
			written, err := os.ReadFile(filepath.Join(root, "write.txt"))
			if denied && !os.IsNotExist(err) || !denied && (err != nil || string(written) != "product-payload-sentinel") {
				t.Fatalf("unexpected effect: %q (%v)", written, err)
			}
			lifecycle.stop()
			if !tc.healthy {
				if !bytes.Contains(stderr.Bytes(), []byte("audit storage unavailable")) {
					t.Fatal("degraded audit was not observable")
				}
				return
			}
			assertWrittenAudit(t, store)
		})
	}
}

func TestProductionMetadataAuditAdmission(t *testing.T) {
	for _, mode := range []string{"operation_first", "audit_required"} {
		for _, healthy := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/healthy=%v", mode, healthy), func(t *testing.T) {
				root := t.TempDir()
				source := filepath.Join(root, "flashgate_config.toml")
				if !healthy {
					testconfig.Write(t, filepath.Join(root, "audit"), []byte("unavailable store"))
				}
				if err := os.WriteFile(filepath.Join(root, "present.txt"), nil, 0o600); err != nil {
					t.Fatal(err)
				}
				settings := "[audit]\nfailure_policy = '" + mode + "'\n"
				if mode == "audit_required" {
					settings += "required_operations = ['filesystem.stat']\n"
				}
				testconfig.Write(t, source, []byte(configDocument(t, root, settings)))
				deps := defaultBootstrapDependencies(source)
				cfg, err := deps.loadConfig()
				if err != nil {
					t.Fatal(err)
				}
				roots, err := deps.newRoots(cfg)
				if err != nil {
					t.Fatal(err)
				}
				registry, lifecycle, err := createRuntimeToolRegistry(roots, cfg, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer lifecycle.stop()
				if err := admitProfile(registry); err != nil {
					t.Fatal(err)
				}
				lifecycle.start()
				for _, path := range []string{"present.txt", "missing.txt", "missing/child.txt"} {
					params := fmt.Sprintf(`{"name":"get_path_info","arguments":{"root_id":"work","path":%q}}`, path)
					response, raw := runCallToolWireRequest(t, registry, params)
					got, err := mcptest.DecodeCallToolResult(response.Result)
					denied := mode == "audit_required" && !healthy
					if err != nil || got.IsError != denied || got.HasStructuredContent == denied {
						t.Fatalf("%s: %s (%v)", path, raw, err)
					}
					if denied {
						payload := got.TextContent.(map[string]any)
						if payload["category"] != "access_denied" {
							t.Fatalf("unexpected denial for %s: %s", path, raw)
						}
					} else {
						metadata, ok := got.StructuredContent.(map[string]any)
						if !ok || metadata["exists"] != (path == "present.txt") {
							t.Fatalf("wrong metadata for %s: %s", path, raw)
						}
					}
				}
			})
		}
	}
}

func assertWrittenAudit(t *testing.T, store string) {
	t.Helper()
	type recordedEvent struct {
		ID        string          `json:"id"`
		Parent    string          `json:"parent"`
		Caller    string          `json:"caller"`
		Effective string          `json:"effective"`
		Phase     string          `json:"phase"`
		Operation string          `json:"operation"`
		Request   json.RawMessage `json:"correlation"`
	}
	var events []recordedEvent
	err := filepath.WalkDir(store, func(path string, entry os.DirEntry, err error) error {
		if err != nil || filepath.Ext(path) != ".jsonl" {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Bytes()
			if bytes.Contains(line, []byte("product-payload-sentinel")) {
				t.Fatal("payload leaked into audit")
			}
			var event recordedEvent
			if err := json.Unmarshal(line, &event); err != nil {
				return err
			}
			events = append(events, event)
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 || events[0].Phase != "segment" ||
		events[1].Phase != "decision" || events[2].Phase != "intent" || events[3].Phase != "outcome" {
		t.Fatalf("unexpected event lifecycle: %+v", events)
	}
	decision, intent, outcome := events[1], events[2], events[3]
	if decision.Effective != "" || decision.Caller == "" || intent.Effective == "" || intent.Operation != "filesystem.write" ||
		intent.Parent != decision.ID || outcome.Parent != intent.ID ||
		!bytes.Equal(intent.Request, decision.Request) || !bytes.Equal(outcome.Request, decision.Request) {
		t.Fatalf("unbound audit evidence: %+v", events)
	}
}
