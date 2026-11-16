package main

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/diagnostics"
	"github.com/thomasweidner/flashgate-mcp/internal/fs"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/router"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/server"
	"github.com/thomasweidner/flashgate-mcp/internal/mcp/tools"
	"github.com/thomasweidner/flashgate-mcp/internal/roots"
	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

var errInvalidBootstrapDependencies = errors.New("invalid bootstrap dependencies")

type runnableServer interface {
	Run(context.Context) error
}

type bootstrapDependencies struct {
	loadConfig      func() (config.Config, error)
	newRoots        func(config.Config) (*roots.Registry, error)
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
		newRoots:        roots.New,
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

	rootRegistry, err := dependencies.newRoots(cfg)
	if err != nil {
		return categorizeRootError(err)
	}
	if rootRegistry == nil {
		return config.NewError(config.CategoryStartupFailed, errInvalidBootstrapDependencies)
	}
	filesystem, err := rootRegistry.Lookup(rootRegistry.DefaultID())
	if err != nil {
		return config.NewError(config.CategoryStartupFailed, err)
	}

	toolRegistry, err := dependencies.newToolRegistry(
		filesystem,
		cfg.Files().MaxFileSize(),
		capabilitiesFromReadOnly(cfg.Files().ReadOnly()),
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

	return mcpServer.Run(ctx)
}

func validateBootstrapDependencies(dependencies bootstrapDependencies) error {
	if dependencies.loadConfig == nil ||
		dependencies.newRoots == nil ||
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
