package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/diagnostics"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/server"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

const developmentCWDWarning = "flashgate-mcp: warning: development CWD root enabled"

var errInvalidBootstrapDependencies = errors.New("invalid bootstrap dependencies")

type runnableServer interface {
	Run(context.Context) error
}

type bootstrapDependencies struct {
	loadConfig      func() (config.Config, error)
	newFilesystem   func(config.Config) (fs.FileSystem, error)
	newToolRegistry func(fs.FileSystem, int64, toolCapabilities) (*tools.Registry, error)
	newRouter       func(string, string, *tools.Registry) *router.Router
	newServer       func(io.Reader, io.Writer, *router.Router, server.Options) runnableServer
}

func run(ctx context.Context, configPath string) error {
	return runWithIO(ctx, os.Stdin, os.Stdout, os.Stderr, defaultBootstrapDependencies(configPath))
}

func defaultBootstrapDependencies(configPath string) bootstrapDependencies {
	return bootstrapDependencies{
		loadConfig: func() (config.Config, error) {
			return config.Load(config.Source{Role: config.Direct, Path: configPath})
		},
		newFilesystem: func(cfg config.Config) (fs.FileSystem, error) {
			guard, err := security.NewPathGuardWithPolicy(cfg.Filesystem().RootPath(), securityPolicyFromConfig(cfg))
			if err != nil {
				return nil, err
			}
			if err := cfg.ValidateDataRoot(guard); err != nil {
				return nil, err
			}
			return fs.NewLocalFileSystemWithGuard(guard, filesystemLimitsFromConfig(cfg))
		},
		newToolRegistry: createToolRegistry,
		newRouter:       createRouter,
		newServer: func(in io.Reader, out io.Writer, mcpRouter *router.Router, options server.Options) runnableServer {
			return server.NewWithOptions(in, out, mcpRouter, options)
		},
	}
}

func runWithIO(
	ctx context.Context,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	dependencies bootstrapDependencies,
) error {
	if err := validateBootstrapDependencies(dependencies); err != nil {
		return err
	}

	cfg, err := dependencies.loadConfig()
	if err != nil {
		return err
	}

	filesystem, err := dependencies.newFilesystem(cfg)
	if err != nil {
		return categorizeRootError(err)
	}
	if isNilBootstrapValue(filesystem) {
		return config.NewError(config.CategoryStartupFailed, errInvalidBootstrapDependencies)
	}

	toolRegistry, err := dependencies.newToolRegistry(
		filesystem,
		cfg.Filesystem().MaxFileSize(),
		capabilitiesFromReadOnly(cfg.Filesystem().ReadOnly()),
	)
	if err != nil {
		return config.NewError(config.CategoryStartupFailed, err)
	}
	if toolRegistry == nil {
		return config.NewError(config.CategoryStartupFailed, errInvalidBootstrapDependencies)
	}
	mcpRouter := dependencies.newRouter(cfg.Server().Name(), cfg.Server().Version(), toolRegistry)
	if mcpRouter == nil {
		return config.NewError(config.CategoryStartupFailed, errInvalidBootstrapDependencies)
	}
	mcpServer := dependencies.newServer(stdin, stdout, mcpRouter, serverOptionsFromConfig(cfg, stderr))
	if isNilBootstrapValue(mcpServer) {
		return config.NewError(config.CategoryStartupFailed, errInvalidBootstrapDependencies)
	}

	if cfg.Filesystem().RootPath() == "." && cfg.Filesystem().AllowCWDRoot() {
		_, _ = fmt.Fprintln(stderr, developmentCWDWarning)
	}

	return mcpServer.Run(ctx)
}

func validateBootstrapDependencies(dependencies bootstrapDependencies) error {
	if dependencies.loadConfig == nil ||
		dependencies.newFilesystem == nil ||
		dependencies.newToolRegistry == nil ||
		dependencies.newRouter == nil ||
		dependencies.newServer == nil {
		return config.NewError(config.CategoryStartupFailed, errInvalidBootstrapDependencies)
	}

	return nil
}

func isNilBootstrapValue(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func categorizeRootError(err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return config.NewError(config.CategoryRootNotFound, err)
	case errors.Is(err, security.ErrRootNotDirectory):
		return config.NewError(config.CategoryRootNotDirectory, err)
	case errors.Is(err, os.ErrPermission),
		errors.Is(err, security.ErrHiddenPathDenied),
		errors.Is(err, security.ErrUNCPathDenied),
		errors.Is(err, security.ErrSymlinkDenied),
		errors.Is(err, security.ErrReparsePointDenied),
		errors.Is(err, security.ErrProtectedPath),
		errors.Is(err, security.ErrOutsideRoot):
		return config.NewError(config.CategoryRootNotAllowed, err)
	default:
		return config.NewError(config.CategoryStartupFailed, err)
	}
}

func filesystemLimitsFromConfig(cfg config.Config) fs.Limits {
	return fs.Limits{
		MaxWriteBytes:    cfg.Filesystem().MaxWriteBytes(),
		MaxListEntries:   cfg.Filesystem().MaxListEntries(),
		MaxCopyBytes:     cfg.Filesystem().MaxCopyBytes(),
		MaxDeleteEntries: cfg.Filesystem().MaxDeleteEntries(),
	}
}

func securityPolicyFromConfig(cfg config.Config) security.Policy {
	return security.Policy{
		AllowHiddenFiles: cfg.Security().AllowHiddenFiles(),
		AllowUNCPaths:    cfg.Security().AllowUNCPaths(),
		FollowSymlinks:   cfg.Security().FollowSymlinks(),
	}
}

func serverOptionsFromConfig(cfg config.Config, stderr io.Writer) server.Options {
	logging := cfg.Logging()
	return server.Options{
		MaxMessageBytes:  cfg.Server().MaxMessageBytes(),
		MaxArgumentBytes: cfg.Server().MaxArgumentBytes(),
		MaxResponseBytes: cfg.Server().MaxResponseBytes(),
		Diagnostics: diagnostics.NewBoundedLogger(
			logging.Level == config.LogDebug, logging.Format == config.LogJSON,
			logging.MaxRecordBytes, logging.MaxRecords, stderr,
		),
	}
}
