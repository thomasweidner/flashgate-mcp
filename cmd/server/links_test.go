package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func TestLinkPolicyOnMCPWire(t *testing.T) {
	for _, follow := range []bool{false, true} {
		for _, tc := range []struct{ name, args string }{
			{"read_file", `"root_id":"work","path":"alias.txt"`},
			{"get_path_info", `"root_id":"work","path":"alias.txt"`},
			{"list_directory", `"root_id":"work","path":"dir-alias"`},
			{"write_file", `"root_id":"work","path":"alias.txt","content":"changed","overwrite":true`},
			{"create_directory", `"root_id":"work","path":"dir-alias/new"`},
			{"delete_path", `"root_id":"work","path":"alias.txt"`},
			{"copy_path", `"source_root_id":"work","source":"alias.txt","target_root_id":"work","target":"copy.txt"`},
			{"move_path", `"source_root_id":"work","source":"alias.txt","target_root_id":"work","target":"renamed.txt"`},
		} {
			t.Run(tc.name+map[bool]string{false: "-deny", true: "-allow"}[follow], func(t *testing.T) {
				root := t.TempDir()
				testconfig.Write(t, filepath.Join(root, "target.txt"), []byte("ordinary-marker"))
				if err := os.Mkdir(filepath.Join(root, "directory"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("target.txt", filepath.Join(root, "alias.txt")); err != nil {
					t.Skipf("native symlink unavailable: %v", err)
				}
				if err := os.Symlink("directory", filepath.Join(root, "dir-alias")); err != nil {
					t.Fatal(err)
				}
				value := "false"
				if follow {
					value = "true"
				}
				source := filepath.Join(t.TempDir(), "config.toml")
				testconfig.Write(t, source, []byte(configDocument(t, root, "work.policy='p'\n[policy.p]\naccess=['files.read','files.write']\nallowed_extensions=['.txt']\nfollow_symlinks="+value+"\n"+profileSettings(false))))
				deps := defaultBootstrapDependencies(source)
				cfg, err := deps.loadConfig()
				if err != nil {
					t.Fatal(err)
				}
				roots, err := deps.newRoots(cfg)
				if err != nil {
					t.Fatal(err)
				}
				registry, err := createToolRegistry(roots, cfg.Files().MaxFileSize(), cfg.Profile())
				if err != nil {
					t.Fatal(err)
				}
				if err := admitProfile(registry, cfg.Profile().Policy()); err != nil {
					t.Fatal(err)
				}
				response, raw := runCallToolWireRequest(t, registry, `{"name":"`+tc.name+`","arguments":{`+tc.args+`}}`)
				var envelope struct {
					IsError bool `json:"isError"`
				}
				if err := json.Unmarshal(response.Result, &envelope); err != nil || response.Error != nil || envelope.IsError == follow || strings.Contains(raw, root) {
					t.Fatal("wire parity", raw, err)
				}
				if !follow {
					decoded, err := mcptest.DecodeCallToolResult(response.Result)
					if err != nil || decoded.HasStructuredContent || decoded.TextContent.(map[string]any)["category"] != "invalid_path" || strings.Contains(raw, "ordinary-marker") {
						t.Fatal("unsafe denial", raw)
					}
					b, err := os.ReadFile(filepath.Join(root, "target.txt"))
					if err != nil || string(b) != "ordinary-marker" {
						t.Fatal("denied effect", err)
					}
				}
			})
		}
	}
}
