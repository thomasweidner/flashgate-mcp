package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func configDocument(t testing.TB, root, settings string) string {
	t.Helper()
	return "[files]\ndefault_id = 'work'\nroots.work.path = " + quotedConfigPath(t, root) + "\n" + settings
}

func quotedConfigPath(t testing.TB, root string) string {
	t.Helper()
	encoded, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func writeConfig(t *testing.T, document string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "flashgate_config.toml")
	testconfig.Write(t, path, []byte(document))
	return path
}

func decodeTestConfig(t testing.TB, settings string) config.Config {
	t.Helper()
	cfg, err := config.Decode([]byte(configDocument(t, t.TempDir(), settings)), config.Direct)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func profileSettings(readOnly bool) string {
	if readOnly {
		return ""
	}
	return "[profiles]\nactive = 'write'\n"
}

func testProfile(t testing.TB, readOnly bool) config.EffectiveProfile {
	t.Helper()
	return decodeTestConfig(t, profileSettings(readOnly)).Profile()
}

func testRootRegistry(t *testing.T, root string, limits fs.Limits) *roots.Registry {
	t.Helper()
	settings := fmt.Sprintf("roots.work.access = ['read', 'write']\nmax_write_bytes = %d\nmax_list_entries = %d\nmax_copy_bytes = %d\nmax_delete_entries = %d\n", limits.MaxWriteBytes, limits.MaxListEntries, limits.MaxCopyBytes, limits.MaxDeleteEntries)
	path := writeConfig(t, configDocument(t, root, settings))
	cfg, err := config.Load(config.Source{Role: config.Direct, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := roots.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}
