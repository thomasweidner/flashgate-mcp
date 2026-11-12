package diagnostics

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactRemovesSecretsAndHostPaths(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		`Authorization: Bearer abc123`,
		`Authorization: Basic abc123`,
		`password=secret`,
		`"api_key":"secret"`,
		`-----BEGIN RSA PRIVATE KEY-----`,
		`postgres://user:pass@example.test/db`,
		`C:\Users\Example\secret.txt`,
		`\\server\share\secret.txt`,
		`/home/example/secret.txt`,
	}, "\n")

	redacted := Redact(input)

	for _, forbidden := range []string{
		"abc123",
		"password=secret",
		`"api_key":"secret"`,
		"BEGIN RSA PRIVATE KEY",
		"user:pass@",
		`C:\Users\Example`,
		`\\server\share`,
		"/home/example",
	} {
		if strings.Contains(redacted, forbidden) {
			t.Fatalf("expected %q to be redacted from %q", forbidden, redacted)
		}
	}

	if !strings.Contains(redacted, "[REDACTED]") && !strings.Contains(redacted, "[REDACTED_PATH]") {
		t.Fatalf("expected redaction markers, got %q", redacted)
	}
}

func TestLoggerWritesOnlyWhenDebugEnabled(t *testing.T) {
	t.Parallel()

	disabledOutput := &bytes.Buffer{}
	NewLogger(false, disabledOutput).Debugf("password=secret")
	if disabledOutput.Len() != 0 {
		t.Fatalf("expected no output when debug is disabled, got %q", disabledOutput.String())
	}

	enabledOutput := &bytes.Buffer{}
	NewLogger(true, enabledOutput).Debugf("password=secret")
	output := enabledOutput.String()
	if output == "" {
		t.Fatal("expected debug output")
	}

	if strings.Contains(output, "secret") {
		t.Fatalf("expected secret to be redacted, got %q", output)
	}
}

func TestBoundedDiagnosticsRejectLogInjection(t *testing.T) {
	var out bytes.Buffer
	logger := NewBoundedLogger(true, true, 128, 2, &out)
	logger.Debugf("token=secret\nforged\rline")
	logger.Debugf("%s", strings.Repeat("x", 1000))
	logger.Debugf("third")
	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 2 || strings.Contains(out.String(), "secret") || strings.Contains(out.String(), "third") {
		t.Fatalf("diagnostics exceeded policy: %q", out.String())
	}
	for _, line := range lines {
		if len(line)+1 > 128 || !json.Valid([]byte(line)) || strings.ContainsRune(line, '\r') {
			t.Fatalf("unbounded or malformed record: %q", line)
		}
	}
	if !strings.Contains(lines[1], "[TRUNCATED]") {
		t.Fatal("oversized record was not replaced")
	}
}
