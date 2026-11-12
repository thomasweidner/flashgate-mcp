package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/version"
)

var errInvalidCLIArguments = errors.New("invalid CLI arguments")

type serverRunner func(context.Context, string) error

func runCLI(ctx context.Context, args []string, stdout io.Writer, runServer serverRunner) (int, error) {
	switch len(args) {
	case 0:
		return runServerCLI(ctx, "", runServer)
	case 1:
		switch args[0] {
		case "--version":
			_, _ = fmt.Fprintln(stdout, version.Get().String())
			return 0, nil
		case "--help", "-h":
			_, _ = fmt.Fprint(stdout, helpText())
			return 0, nil
		default:
			return 2, fmt.Errorf("%w: unknown argument\nUse --help for usage.", errInvalidCLIArguments)
		}
	case 2:
		if args[0] == "--config" && args[1] != "" {
			return runServerCLI(ctx, args[1], runServer)
		}
		if args[0] == "--version" && args[1] == "--verbose" {
			_, _ = fmt.Fprintln(stdout, version.Get().VerboseString())
			return 0, nil
		}
		return 2, fmt.Errorf("%w: unsupported argument combination\nUse --help for usage.", errInvalidCLIArguments)
	default:
		return 2, fmt.Errorf("%w: too many arguments\nUse --help for usage.", errInvalidCLIArguments)
	}
}

func runServerCLI(ctx context.Context, path string, runServer serverRunner) (int, error) {
	err := runServer(ctx, path)
	if err == nil || errors.Is(err, context.Canceled) {
		return 0, nil
	}
	if category, ok := config.CategoryOf(err); ok && category != config.CategoryStartupFailed {
		return 3, err
	}
	return 1, err
}

func helpText() string {
	return `flashgate-mcp

Usage:
  flashgate-mcp
  flashgate-mcp --config <absolute-path>
  flashgate-mcp --version
  flashgate-mcp --version --verbose
  flashgate-mcp --help

Configuration:
  The executing user's .flashgate/flashgate_config.toml is the default.
  --config replaces that default with one trusted absolute TOML file.
  The file must declare a filesystem root before the server can start.
`
}
