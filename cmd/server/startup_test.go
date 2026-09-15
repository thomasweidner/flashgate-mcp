package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/server"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func TestRunMainMapsSafeStartupCategoriesAndExitCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		category config.ErrorCategory
		exitCode int
	}{
		{config.CategoryMissingRoot, 3},
		{config.CategoryInvalidRoot, 3},
		{config.CategoryRootNotFound, 3},
		{config.CategoryRootNotDirectory, 3},
		{config.CategoryRootNotAllowed, 3},
		{config.CategoryInvalidProfile, 3},
		{config.CategoryInvalidDevelopmentOption, 3},
		{config.CategoryStartupFailed, 1},
	}

	for _, test := range tests {
		t.Run(string(test.category), func(t *testing.T) {
			t.Parallel()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			runner := func(context.Context) error {
				return config.NewError(test.category, errors.New(`C:\private\root: raw failure`))
			}

			exitCode := runMain(context.Background(), nil, &stdout, &stderr, runner)
			if exitCode != test.exitCode {
				t.Fatalf("expected exit code %d, got %d", test.exitCode, exitCode)
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected empty stdout, got %q", stdout.String())
			}
			expected := "flashgate-mcp: startup failed (" + string(test.category) + ")\n"
			if stderr.String() != expected {
				t.Fatalf("expected stderr %q, got %q", expected, stderr.String())
			}
			assertNoHostPathLeak(t, stderr.String())
		})
	}
}

func TestRunMainMapsUnexpectedErrorToStartupFailed(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runMain(context.Background(), nil, &stdout, &stderr, func(context.Context) error {
		return errors.New(`/private/root: raw startup failure`)
	})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if stderr.String() != "flashgate-mcp: startup failed (startup_failed)\n" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
	assertNoHostPathLeak(t, stderr.String())
}

func TestRunMainKeepsCLIExitCodeTwo(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runMain(context.Background(), []string{"--unknown"}, &stdout, &stderr, failIfServerRuns(t))
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "invalid CLI arguments") {
		t.Fatalf("expected safe CLI error, got %q", stderr.String())
	}
}

func TestRunMainDoesNotEchoHostPathCLIArgument(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runMain(context.Background(), []string{`C:\private\root`}, &stdout, &stderr, failIfServerRuns(t))
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	assertNoHostPathLeak(t, stderr.String())
}

func TestRunMainSuccessfulStartHasNoBanner(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runMain(context.Background(), nil, &stdout, &stderr, func(context.Context) error { return nil })
	if exitCode != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("expected silent successful start, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestRunMainContextCancellationIsSilentSuccess(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runMain(context.Background(), nil, &stdout, &stderr, func(context.Context) error {
		return context.Canceled
	})
	if exitCode != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("expected silent successful cancellation, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestRunMainActualConfigurationFailuresAreSafe(t *testing.T) {
	tests := []struct {
		name     string
		prepare  func(*testing.T) string
		category config.ErrorCategory
	}{
		{
			name: "missing root",
			prepare: func(t *testing.T) string {
				unsetEnvForTest(t, "MCP_ROOT")
				return ""
			},
			category: config.CategoryMissingRoot,
		},
		{
			name: "empty root",
			prepare: func(t *testing.T) string {
				t.Setenv("MCP_ROOT", "")
				return ""
			},
			category: config.CategoryInvalidRoot,
		},
		{
			name: "missing path",
			prepare: func(t *testing.T) string {
				root := filepath.Join(t.TempDir(), "missing-root")
				t.Setenv("MCP_ROOT", root)
				return root
			},
			category: config.CategoryRootNotFound,
		},
		{
			name: "file root",
			prepare: func(t *testing.T) string {
				root := filepath.Join(t.TempDir(), "root.txt")
				if err := os.WriteFile(root, []byte("file"), 0o600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("MCP_ROOT", root)
				return root
			},
			category: config.CategoryRootNotDirectory,
		},
		{
			name: "invalid profile",
			prepare: func(t *testing.T) string {
				root := t.TempDir()
				t.Setenv("MCP_ROOT", root)
				t.Setenv("MCP_READ_ONLY", "invalid")
				return root
			},
			category: config.CategoryInvalidProfile,
		},
		{
			name: "invalid development option",
			prepare: func(t *testing.T) string {
				root := t.TempDir()
				t.Setenv("MCP_ROOT", root)
				t.Setenv("MCP_ALLOW_CWD_ROOT", "TRUE")
				return root
			},
			category: config.CategoryInvalidDevelopmentOption,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := test.prepare(t)
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			runner := func(ctx context.Context) error {
				return runWithIO(ctx, strings.NewReader(""), &stdout, &stderr, defaultBootstrapDependencies())
			}

			exitCode := runMain(context.Background(), nil, &stdout, &stderr, runner)
			if exitCode != 3 {
				t.Fatalf("expected exit code 3, got %d", exitCode)
			}
			if stdout.Len() != 0 {
				t.Fatalf("expected empty stdout, got %q", stdout.String())
			}
			expected := "flashgate-mcp: startup failed (" + string(test.category) + ")\n"
			if stderr.String() != expected {
				t.Fatalf("expected stderr %q, got %q", expected, stderr.String())
			}
			if root != "" && strings.Contains(stderr.String(), root) {
				t.Fatalf("stderr leaked root %q: %q", root, stderr.String())
			}
			assertNoHostPathLeak(t, stderr.String())
		})
	}
}

func TestRunWithIODevelopmentWarningExactlyOnce(t *testing.T) {
	t.Setenv("MCP_ROOT", ".")
	t.Setenv("MCP_ALLOW_CWD_ROOT", "true")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies := defaultBootstrapDependencies()
	dependencies.newServer = func(_ io.Reader, _ io.Writer, _ *router.Router, _ server.Options) runnableServer {
		return stubRunnableServer{}
	}

	err := runWithIO(context.Background(), strings.NewReader(""), &stdout, &stderr, dependencies)
	if err != nil {
		t.Fatalf("expected development start to succeed, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if stderr.String() != developmentCWDWarning+"\n" {
		t.Fatalf("expected exactly one warning, got %q", stderr.String())
	}
	assertNoHostPathLeak(t, stderr.String())
}

func TestRunWithIORootFailureStopsBeforeRegistryRouterAndServer(t *testing.T) {
	t.Setenv("MCP_ROOT", t.TempDir())
	dependencies := defaultBootstrapDependencies()
	dependencies.newFilesystem = func(config.Config) (fs.FileSystem, error) {
		return nil, security.ErrUNCPathDenied
	}
	registryCalls := 0
	dependencies.newToolRegistry = func(fs.FileSystem, int64, toolCapabilities) *tools.Registry {
		registryCalls++
		return tools.NewRegistry()
	}
	routerCalls := 0
	originalRouterFactory := dependencies.newRouter
	dependencies.newRouter = func(name string, version string, registry *tools.Registry, capabilities toolCapabilities) *router.Router {
		routerCalls++
		return originalRouterFactory(name, version, registry, capabilities)
	}
	serverCalls := 0
	dependencies.newServer = func(io.Reader, io.Writer, *router.Router, server.Options) runnableServer {
		serverCalls++
		return stubRunnableServer{}
	}

	err := runWithIO(context.Background(), strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, dependencies)
	category, ok := config.CategoryOf(err)
	if !ok || category != config.CategoryRootNotAllowed {
		t.Fatalf("expected root_not_allowed, got %v", err)
	}
	if registryCalls != 0 || routerCalls != 0 || serverCalls != 0 {
		t.Fatalf("expected no post-root factories, registry=%d router=%d server=%d", registryCalls, routerCalls, serverCalls)
	}
}

func TestRunWithIORejectsInvalidBootstrapDependenciesWithoutPanic(t *testing.T) {
	t.Setenv("MCP_ROOT", t.TempDir())

	tests := map[string]func(bootstrapDependencies) bootstrapDependencies{
		"missing config loader": func(dependencies bootstrapDependencies) bootstrapDependencies {
			dependencies.loadConfig = nil
			return dependencies
		},
		"missing filesystem factory": func(dependencies bootstrapDependencies) bootstrapDependencies {
			dependencies.newFilesystem = nil
			return dependencies
		},
		"missing registry factory": func(dependencies bootstrapDependencies) bootstrapDependencies {
			dependencies.newToolRegistry = nil
			return dependencies
		},
		"missing router factory": func(dependencies bootstrapDependencies) bootstrapDependencies {
			dependencies.newRouter = nil
			return dependencies
		},
		"missing server factory": func(dependencies bootstrapDependencies) bootstrapDependencies {
			dependencies.newServer = nil
			return dependencies
		},
		"nil filesystem": func(dependencies bootstrapDependencies) bootstrapDependencies {
			dependencies.newFilesystem = func(config.Config) (fs.FileSystem, error) { return nil, nil }
			return dependencies
		},
		"nil registry": func(dependencies bootstrapDependencies) bootstrapDependencies {
			dependencies.newToolRegistry = func(fs.FileSystem, int64, toolCapabilities) *tools.Registry { return nil }
			return dependencies
		},
		"nil router": func(dependencies bootstrapDependencies) bootstrapDependencies {
			dependencies.newRouter = func(string, string, *tools.Registry, toolCapabilities) *router.Router { return nil }
			return dependencies
		},
		"nil server": func(dependencies bootstrapDependencies) bootstrapDependencies {
			dependencies.newServer = func(io.Reader, io.Writer, *router.Router, server.Options) runnableServer { return nil }
			return dependencies
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			err := runWithIO(
				context.Background(),
				strings.NewReader(""),
				io.Discard,
				io.Discard,
				mutate(defaultBootstrapDependencies()),
			)
			category, ok := config.CategoryOf(err)
			if !ok || category != config.CategoryStartupFailed {
				t.Fatalf("expected startup_failed, got %v", err)
			}
		})
	}
}

func TestCategorizeRootError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err      error
		category config.ErrorCategory
	}{
		{os.ErrNotExist, config.CategoryRootNotFound},
		{security.ErrRootNotDirectory, config.CategoryRootNotDirectory},
		{security.ErrUNCPathDenied, config.CategoryRootNotAllowed},
		{security.ErrSymlinkDenied, config.CategoryRootNotAllowed},
		{os.ErrPermission, config.CategoryRootNotAllowed},
		{errors.New("unexpected"), config.CategoryStartupFailed},
	}
	for _, test := range tests {
		category, ok := config.CategoryOf(categorizeRootError(test.err))
		if !ok || category != test.category {
			t.Fatalf("error %v: expected %s, got %s", test.err, test.category, category)
		}
	}
}

type stubRunnableServer struct {
	err error
}

func (s stubRunnableServer) Run(context.Context) error { return s.err }

func assertNoHostPathLeak(t *testing.T, output string) {
	t.Helper()
	for _, forbidden := range []string{`C:\`, `\\server\share`, `/private/`, `/home/`, `/etc/`} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("output contains host path %q: %q", forbidden, output)
		}
	}
}

func unsetEnvForTest(t *testing.T, name string) {
	t.Helper()
	previous, existed := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(name, previous)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}
