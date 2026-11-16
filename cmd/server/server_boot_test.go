package main

import (
	"io"
	"testing"
)

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
