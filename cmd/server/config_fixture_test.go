package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
)

func configDocument(t *testing.T, root, settings string) string {
	t.Helper()
	encoded, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return "[filesystem]\nroot = " + string(encoded) + "\n" + settings
}

func writeConfig(t *testing.T, document string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "flashgate_config.toml")
	testconfig.Write(t, path, []byte(document))
	return path
}

func decodeTestConfig(t *testing.T, settings string) config.Config {
	t.Helper()
	cfg, err := config.Decode([]byte(configDocument(t, t.TempDir(), settings)), config.Direct)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
