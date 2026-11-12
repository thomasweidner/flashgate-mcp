package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunCLIHelp(t *testing.T) {
	var stdout bytes.Buffer

	exitCode, err := runCLI(context.Background(), []string{"--help"}, &stdout, failIfServerRuns(t))

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := stdout.String()
	expectedParts := []string{
		"flashgate-mcp",
		"Usage:",
		"flashgate-mcp --version",
		"flashgate-mcp --version --verbose",
		"flashgate-mcp --help",
		"flashgate-mcp --config <absolute-path>",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Fatalf("expected help output to contain %q, got %q", part, output)
		}
	}
}

func TestRunCLIShortHelp(t *testing.T) {
	var stdout bytes.Buffer

	exitCode, err := runCLI(context.Background(), []string{"-h"}, &stdout, failIfServerRuns(t))

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("expected help output, got %q", stdout.String())
	}
}

func TestRunCLIVersion(t *testing.T) {
	var stdout bytes.Buffer

	exitCode, err := runCLI(context.Background(), []string{"--version"}, &stdout, failIfServerRuns(t))

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if output := stdout.String(); output != "flashgate-mcp 0.0.0-dev\n" {
		t.Fatalf("unexpected version output %q", output)
	}
}

func TestRunCLIVerboseVersion(t *testing.T) {
	var stdout bytes.Buffer

	exitCode, err := runCLI(context.Background(), []string{"--version", "--verbose"}, &stdout, failIfServerRuns(t))

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	output := stdout.String()
	expectedParts := []string{
		"Product:",
		"Version:",
		"File version:",
		"Commit:",
		"Source time:",
		"Modified:",
		"Go version:",
		"Platform:",
		"Go target:",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Fatalf("expected verbose output to contain %q, got %q", part, output)
		}
	}
}

func TestRunCLIUnknownArgument(t *testing.T) {
	var stdout bytes.Buffer

	exitCode, err := runCLI(context.Background(), []string{"--unknown"}, &stdout, failIfServerRuns(t))

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, errInvalidCLIArguments) {
		t.Fatalf("expected errInvalidCLIArguments, got %v", err)
	}

	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}

	if !strings.Contains(err.Error(), "unknown argument") {
		t.Fatalf("expected unknown argument error, got %v", err)
	}
	if strings.Contains(err.Error(), "--unknown") {
		t.Fatalf("expected argument value to be omitted, got %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}

func TestRunCLIUnsupportedArgumentCombination(t *testing.T) {
	var stdout bytes.Buffer

	exitCode, err := runCLI(context.Background(), []string{"--verbose", "--version"}, &stdout, failIfServerRuns(t))

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, errInvalidCLIArguments) {
		t.Fatalf("expected errInvalidCLIArguments, got %v", err)
	}

	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}

	if !strings.Contains(err.Error(), "unsupported argument combination") {
		t.Fatalf("expected unsupported combination error, got %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}

func TestRunCLITooManyArguments(t *testing.T) {
	var stdout bytes.Buffer

	exitCode, err := runCLI(context.Background(), []string{"--help", "--version", "--verbose"}, &stdout, failIfServerRuns(t))

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, errInvalidCLIArguments) {
		t.Fatalf("expected errInvalidCLIArguments, got %v", err)
	}

	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}

	if !strings.Contains(err.Error(), "too many arguments") {
		t.Fatalf("expected too many arguments error, got %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}

func TestRunCLIWithoutArgumentsRunsServer(t *testing.T) {
	var stdout bytes.Buffer
	called := false

	exitCode, err := runCLI(
		context.Background(),
		nil,
		&stdout,
		func(context.Context, string) error {
			called = true
			return nil
		},
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}

	if !called {
		t.Fatal("expected server runner to be called")
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}

func TestRunCLIWithoutArgumentsReturnsServerError(t *testing.T) {
	var stdout bytes.Buffer
	expectedErr := errors.New("server failed")

	exitCode, err := runCLI(
		context.Background(),
		nil,
		&stdout,
		func(context.Context, string) error {
			return expectedErr
		},
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected server error, got %v", err)
	}

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}

func TestRunCLIContextCancellationIsNormalShutdown(t *testing.T) {
	var stdout bytes.Buffer

	exitCode, err := runCLI(
		context.Background(),
		nil,
		&stdout,
		func(context.Context, string) error { return context.Canceled },
	)

	if err != nil {
		t.Fatalf("expected context cancellation to be treated as normal shutdown, got %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", stdout.String())
	}
}

func failIfServerRuns(t *testing.T) serverRunner {
	t.Helper()

	return func(context.Context, string) error {
		t.Fatal("server runner must not be called")
		return nil
	}
}

func TestRunCLIForwardsOneExplicitPath(t *testing.T) {
	var stdout bytes.Buffer
	path := "test absolute path.toml"
	exit, err := runCLI(context.Background(), []string{"--config", path}, &stdout, func(_ context.Context, got string) error {
		if got != path {
			t.Fatalf("expected original path, got %q", got)
		}
		return nil
	})
	if err != nil || exit != 0 || stdout.Len() != 0 {
		t.Fatalf("unexpected launch behavior: exit=%d error=%v stdout=%q", exit, err, stdout.String())
	}
	for _, args := range [][]string{{"--config"}, {"--config", ""}, {"--config", path, "--help"}} {
		exit, err := runCLI(context.Background(), args, &stdout, failIfServerRuns(t))
		if exit != 2 || !errors.Is(err, errInvalidCLIArguments) {
			t.Fatalf("malformed override accepted: exit=%d error=%v", exit, err)
		}
	}
}
