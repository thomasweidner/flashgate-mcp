package main

import (
	"io"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
)

func TestSecurityPolicyFromConfigUsesSecureDefaults(t *testing.T) {
	t.Parallel()

	policy := securityPolicyFromConfig(config.DefaultConfig())

	if policy.AllowHiddenFiles {
		t.Fatal("expected hidden files to be denied by default")
	}

	if policy.AllowUNCPaths {
		t.Fatal("expected UNC paths to be denied by default")
	}

	if policy.FollowSymlinks {
		t.Fatal("expected symlink following to be denied by default")
	}
}

func TestSecurityPolicyFromConfigUsesEnvironment(t *testing.T) {
	t.Setenv("MCP_ROOT", t.TempDir())
	t.Setenv("MCP_ALLOW_HIDDEN_FILES", "true")
	t.Setenv("MCP_ALLOW_UNC_PATHS", "true")
	t.Setenv("MCP_FOLLOW_SYMLINKS", "true")

	cfg, err := config.LoadFromEnvironment()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	policy := securityPolicyFromConfig(cfg)

	if !policy.AllowHiddenFiles {
		t.Fatal("expected hidden files to be allowed")
	}

	if !policy.AllowUNCPaths {
		t.Fatal("expected UNC paths to be allowed")
	}

	if !policy.FollowSymlinks {
		t.Fatal("expected symlink following to be allowed")
	}
}

func TestFilesystemLimitsFromConfigUsesEnvironment(t *testing.T) {
	t.Setenv("MCP_ROOT", t.TempDir())
	t.Setenv("MCP_MAX_WRITE_BYTES", "111")
	t.Setenv("MCP_MAX_LIST_ENTRIES", "22")
	t.Setenv("MCP_MAX_COPY_BYTES", "333")
	t.Setenv("MCP_MAX_COPY_ENTRIES", "33")
	t.Setenv("MCP_MAX_DELETE_ENTRIES", "44")

	cfg, err := config.LoadFromEnvironment()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	limits := filesystemLimitsFromConfig(cfg)

	if limits.MaxWriteBytes != 111 {
		t.Fatalf("expected max write bytes 111, got %d", limits.MaxWriteBytes)
	}

	if limits.MaxListEntries != 22 {
		t.Fatalf("expected max list entries 22, got %d", limits.MaxListEntries)
	}

	if limits.MaxCopyBytes != 333 {
		t.Fatalf("expected max copy bytes 333, got %d", limits.MaxCopyBytes)
	}
	if limits.MaxCopyEntries != 33 {
		t.Fatalf("expected max copy entries 33, got %d", limits.MaxCopyEntries)
	}

	if limits.MaxDeleteEntries != 44 {
		t.Fatalf("expected max delete entries 44, got %d", limits.MaxDeleteEntries)
	}
}

func TestServerOptionsFromConfigUsesEnvironment(t *testing.T) {
	t.Setenv("MCP_ROOT", t.TempDir())
	t.Setenv("MCP_DEBUG", "true")
	t.Setenv("MCP_MAX_JSONRPC_MESSAGE_BYTES", "111")
	t.Setenv("MCP_MAX_TOOL_ARGUMENT_BYTES", "222")
	t.Setenv("MCP_MAX_RESPONSE_BYTES", "333")

	cfg, err := config.LoadFromEnvironment()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	options := serverOptionsFromConfig(cfg, io.Discard)

	if options.MaxMessageBytes != 111 {
		t.Fatalf("expected max message bytes 111, got %d", options.MaxMessageBytes)
	}

	if options.MaxArgumentBytes != 222 {
		t.Fatalf("expected max argument bytes 222, got %d", options.MaxArgumentBytes)
	}

	if options.MaxResponseBytes != 333 {
		t.Fatalf("expected max response bytes 333, got %d", options.MaxResponseBytes)
	}

	if options.Diagnostics == nil {
		t.Fatal("expected diagnostics logger")
	}
}
