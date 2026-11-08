package diagnostics

import (
	"bytes"
	"reflect"
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

func TestExecutionRedactorRemovesSensitiveEnvironmentValues(t *testing.T) {
	t.Parallel()

	environment := map[string]string{
		"LANG":                 "en_US.UTF-8",
		"API_TOKEN":            "short-token",
		"DATABASE_PASSWORD_V2": "longer-password",
	}
	redactor := NewExecutionRedactor(environment, "application-specific-secret")

	output := redactor.Redact(strings.Join([]string{
		"token=short-token",
		"password longer-password",
		"custom application-specific-secret",
		"locale en_US.UTF-8",
	}, "\n"))

	for _, forbidden := range []string{"short-token", "longer-password", "application-specific-secret"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("expected %q to be redacted from %q", forbidden, output)
		}
	}
	if !strings.Contains(output, "en_US.UTF-8") {
		t.Fatalf("expected a non-sensitive environment value to remain, got %q", output)
	}
}

func TestExecutionRedactorReplacesLongestSensitiveValueFirst(t *testing.T) {
	t.Parallel()

	redactor := NewExecutionRedactor(nil, "token", "token-suffix", "", "[REDACTED]")
	if got := redactor.Redact("token-suffix token"); got != "[REDACTED] [REDACTED]" {
		t.Fatalf("unexpected redaction result %q", got)
	}
}

func TestRedactedEnvironmentDoesNotExposeOrMutateValues(t *testing.T) {
	t.Parallel()

	environment := map[string]string{"PATH": "/approved/bin", "API_TOKEN": "secret-value"}
	wantInput := map[string]string{"PATH": "/approved/bin", "API_TOKEN": "secret-value"}
	got := RedactedEnvironment(environment)

	if !reflect.DeepEqual(environment, wantInput) {
		t.Fatalf("input environment was mutated: %#v", environment)
	}
	want := map[string]string{"PATH": "[REDACTED]", "API_TOKEN": "[REDACTED]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected redacted environment: got %#v, want %#v", got, want)
	}
}
