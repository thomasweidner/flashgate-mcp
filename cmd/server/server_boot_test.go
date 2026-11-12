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

func TestSecurityPolicyFromConfigUsesTOML(t *testing.T) {
	cfg := decodeTestConfig(t, "[security]\nallow_hidden_files = true\nallow_unc_paths = true\nfollow_symlinks = true\n")

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

func TestFilesystemLimitsFromConfigUsesTOML(t *testing.T) {
	cfg := decodeTestConfig(t, "max_write_bytes = 111\nmax_list_entries = 22\nmax_copy_bytes = 333\nmax_delete_entries = 44\n")

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

	if limits.MaxDeleteEntries != 44 {
		t.Fatalf("expected max delete entries 44, got %d", limits.MaxDeleteEntries)
	}
}

func TestServerOptionsFromConfigUsesTOML(t *testing.T) {
	cfg := decodeTestConfig(t, "[protocol]\nmax_message_bytes = 111\nmax_argument_bytes = 222\nmax_response_bytes = 333\n[logging]\nlevel = \"debug\"\n")

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
