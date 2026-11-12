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
	"github.com/thomasweidner/flashgate-mcp/internal/testconfig"
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
			runner := func(context.Context, string) error {
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
	exitCode := runMain(context.Background(), nil, &stdout, &stderr, func(context.Context, string) error {
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
	exitCode := runMain(context.Background(), nil, &stdout, &stderr, func(context.Context, string) error { return nil })
	if exitCode != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("expected silent successful start, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestRunMainContextCancellationIsSilentSuccess(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := runMain(context.Background(), nil, &stdout, &stderr, func(context.Context, string) error {
		return context.Canceled
	})
	if exitCode != 0 || stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("expected silent successful cancellation, exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestRunMainActualConfigurationFailuresAreSafe(t *testing.T) {
	root := t.TempDir()
	rootFile := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(rootFile, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		path     string
		category config.ErrorCategory
	}{
		{"missing config", filepath.Join(root, "absent.toml"), config.CategoryMissingConfig},
		{"relative override", "private.toml", config.CategoryUntrustedConfig},
		{"missing root", writeConfig(t, "[filesystem]\nread_only = true\n"), config.CategoryMissingRoot},
		{"empty root", writeConfig(t, configDocument(t, "", "")), config.CategoryInvalidRoot},
		{"missing filesystem root", writeConfig(t, configDocument(t, filepath.Join(root, "missing"), "")), config.CategoryRootNotFound},
		{"file root", writeConfig(t, configDocument(t, rootFile, "")), config.CategoryRootNotDirectory},
		{"invalid settings", writeConfig(t, configDocument(t, root, "allow_cwd_root = \"raw-secret\"\n")), config.CategoryInvalidConfig},
		{"unsupported mode", writeConfig(t, configDocument(t, root, "[runtime]\nmode = \"proxy\"\n")), config.CategoryUnsupportedRuntime},
	}
	t.Setenv("MCP_ROOT", root)
	t.Setenv("MCP_READ_ONLY", "true")
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			runner := func(ctx context.Context, configPath string) error {
				dependencies := defaultBootstrapDependencies(configPath)
				if test.category != config.CategoryRootNotFound && test.category != config.CategoryRootNotDirectory {
					dependencies.newFilesystem = func(config.Config) (fs.FileSystem, error) {
						t.Fatal("filesystem initialized before config validation")
						return nil, nil
					}
				}
				return runWithIO(ctx, strings.NewReader(""), &stdout, &stderr, dependencies)
			}
			exitCode := runMain(context.Background(), []string{"--config", test.path}, &stdout, &stderr, runner)
			expected := "flashgate-mcp: startup failed (" + string(test.category) + ")\n"
			if exitCode != 3 || stdout.Len() != 0 || stderr.String() != expected {
				t.Fatalf("unexpected startup result: exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
			}
			assertNoHostPathLeak(t, stderr.String())
		})
	}
}

func TestRunWithIODevelopmentWarningExactlyOnce(t *testing.T) {
	path := writeConfig(t, configDocument(t, ".", "allow_cwd_root = true\n"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies := defaultBootstrapDependencies(path)
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
	path := writeConfig(t, configDocument(t, t.TempDir(), ""))
	dependencies := defaultBootstrapDependencies(path)
	dependencies.newFilesystem = func(config.Config) (fs.FileSystem, error) {
		return nil, security.ErrUNCPathDenied
	}
	registryCalls := 0
	dependencies.newToolRegistry = func(fs.FileSystem, int64, toolCapabilities) (*tools.Registry, error) {
		registryCalls++
		return tools.NewRegistry(), nil
	}
	routerCalls := 0
	originalRouterFactory := dependencies.newRouter
	dependencies.newRouter = func(name string, version string, registry *tools.Registry) *router.Router {
		routerCalls++
		return originalRouterFactory(name, version, registry)
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

func TestRunWithIOPropagatesToolRegistrationFailure(t *testing.T) {
	path := writeConfig(t, configDocument(t, t.TempDir(), ""))
	dependencies := defaultBootstrapDependencies(path)
	var registrationErr error
	dependencies.newToolRegistry = func(filesystem fs.FileSystem, _ int64, capabilities toolCapabilities) (*tools.Registry, error) {
		registry, err := createToolRegistry(filesystem, 0, capabilities)
		if registry != nil || err == nil || !strings.Contains(err.Error(), "register read_file") {
			t.Fatalf("expected read_file contract registration to fail, registry=%v error=%v", registry, err)
		}
		registrationErr = err
		return registry, err
	}
	dependencies.newRouter = func(string, string, *tools.Registry) *router.Router {
		t.Fatal("router created after invalid tool registration")
		return nil
	}
	dependencies.newServer = func(io.Reader, io.Writer, *router.Router, server.Options) runnableServer {
		t.Fatal("server created after invalid tool registration")
		return nil
	}
	var stdout, stderr bytes.Buffer
	err := runWithIO(context.Background(), strings.NewReader(""), &stdout, &stderr, dependencies)
	if registrationErr == nil || !errors.Is(err, registrationErr) || err.Error() != string(config.CategoryStartupFailed) {
		t.Fatalf("expected safe startup error wrapping registration cause, got %v", err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatal("failed bootstrap wrote protocol output or internal diagnostics")
	}
}

func TestRunWithIORejectsInvalidBootstrapDependenciesWithoutPanic(t *testing.T) {
	path := writeConfig(t, configDocument(t, t.TempDir(), ""))

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
			dependencies.newToolRegistry = func(fs.FileSystem, int64, toolCapabilities) (*tools.Registry, error) { return nil, nil }
			return dependencies
		},
		"nil router": func(dependencies bootstrapDependencies) bootstrapDependencies {
			dependencies.newRouter = func(string, string, *tools.Registry) *router.Router { return nil }
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
				mutate(defaultBootstrapDependencies(path)),
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

func TestStartupRejectsConfigInsideMCPRoot(t *testing.T) {
	for _, profile := range []string{"write", "read-only"} {
		t.Run(profile, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "private-config.toml")
			settings := "read_only = false\n"
			if profile == "read-only" {
				settings = "read_only = true\n"
			}
			document := configDocument(t, root, settings)
			testconfig.Write(t, path, []byte(document))
			var stdout, stderr bytes.Buffer
			runner := func(ctx context.Context, selectedPath string) error {
				deps := defaultBootstrapDependencies(selectedPath)
				deps.newToolRegistry = func(fs.FileSystem, int64, toolCapabilities) (*tools.Registry, error) {
					t.Error("overlapping configuration reached tool registration")
					return nil, nil
				}
				return runWithIO(ctx, strings.NewReader(""), &stdout, &stderr, deps)
			}
			code := runMain(context.Background(), []string{"--config", path}, &stdout, &stderr, runner)
			if code != 3 || stdout.Len() != 0 ||
				stderr.String() != "flashgate-mcp: startup failed (root_not_allowed)\n" {
				t.Errorf("startup result: code=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
			}
			assertNoHostPathLeak(t, stderr.String())
			after, err := os.ReadFile(path)
			if err != nil || string(after) != document {
				t.Fatal("startup changed the configuration")
			}
		})
	}
}

func TestStartupRejectsConfigHardlinkAddedAfterLoad(t *testing.T) {
	for _, profile := range []string{"writable", "read-only"} {
		t.Run(profile, func(t *testing.T) {
			root := t.TempDir()
			settings := "read_only = false\n"
			if profile == "read-only" {
				settings = "read_only = true\n"
			}
			document := configDocument(t, root, settings)
			path := writeConfig(t, document)
			alias := filepath.Join(root, "linked-config.toml")
			var stdout, stderr bytes.Buffer
			runner := func(ctx context.Context, selectedPath string) error {
				deps := defaultBootstrapDependencies(selectedPath)
				load := deps.loadConfig
				deps.loadConfig = func() (config.Config, error) {
					cfg, err := load()
					if err != nil {
						t.Fatalf("single-link source failed to load: %v", err)
					}
					if err := os.Link(path, alias); err != nil {
						t.Fatalf("creating native hardlink fixture: %v", err)
					}
					return cfg, nil
				}
				deps.newToolRegistry = func(fs.FileSystem, int64, toolCapabilities) (*tools.Registry, error) {
					t.Error("source hardlink reached tool registration")
					return &tools.Registry{}, nil
				}
				deps.newRouter = func(string, string, *tools.Registry) *router.Router {
					t.Error("source hardlink reached router creation")
					return &router.Router{}
				}
				deps.newServer = func(io.Reader, io.Writer, *router.Router, server.Options) runnableServer {
					t.Error("source hardlink reached MCP initialization")
					return nil
				}
				return runWithIO(ctx, strings.NewReader(""), &stdout, &stderr, deps)
			}
			code := runMain(context.Background(), []string{"--config", path}, &stdout, &stderr, runner)
			if code != 3 || stdout.Len() != 0 ||
				stderr.String() != "flashgate-mcp: startup failed (root_not_allowed)\n" {
				t.Errorf("startup result: code=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
			}
			for _, name := range []string{path, alias} {
				after, err := os.ReadFile(name)
				if err != nil || string(after) != document {
					t.Fatal("startup changed the source or fixture alias")
				}
			}
		})
	}
}

func TestStartupRejectsConfigInsideEffectiveSymlinkRoot(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "data-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("native directory symlinks unavailable: %v", err)
	}
	path := filepath.Join(root, "private-config.toml")
	document := configDocument(t, alias, "[security]\nfollow_symlinks = true\n")
	testconfig.Write(t, path, []byte(document))
	var stdout, stderr bytes.Buffer
	deps := defaultBootstrapDependencies(path)
	deps.newToolRegistry = func(fs.FileSystem, int64, toolCapabilities) (*tools.Registry, error) {
		t.Error("aliased configuration reached tool registration")
		return nil, nil
	}
	err := runWithIO(context.Background(), strings.NewReader(""), &stdout, &stderr, deps)
	if category, _ := config.CategoryOf(err); category != config.CategoryRootNotAllowed ||
		stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("alias denial: error=%v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
}

func TestDocumentedTemplateStartsServer(t *testing.T) {
	template, err := os.ReadFile("../../docs/flashgate-config.toml")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	boundRoot := strings.TrimSuffix(strings.TrimPrefix(configDocument(t, root, ""), "[filesystem]\n"), "\n")
	document := strings.Replace(string(template), "root = \"\"", boundRoot, 1)
	path := writeConfig(t, document)
	var stdout, stderr bytes.Buffer
	input := strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\",\"params\":{\"protocolVersion\":\"2025-11-25\",\"capabilities\":{},\"clientInfo\":{\"name\":\"template\",\"version\":\"1\"}}}\n")
	deps := defaultBootstrapDependencies(path)
	cfg, err := deps.loadConfig()
	if err != nil || cfg.Filesystem().RootPath() != root || !cfg.Filesystem().ReadOnly() {
		t.Fatalf("template did not validate with a bound root: %v", err)
	}
	if err := runWithIO(context.Background(), input, &stdout, &stderr, deps); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 || !strings.Contains(stdout.String(), "\"protocolVersion\":\"2025-11-25\"") {
		t.Fatalf("template launch failed: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != document {
		t.Fatal("normal startup modified the template instance")
	}
}
