//go:build !linux && !windows

package config

import (
	"os"

	"github.com/thomasweidner/flashgate-mcp/internal/security"
)

func defaultPath(Role) (string, error) { return "", os.ErrPermission }

func readTrustedConfig(string, Role) (trustedDocument, error) {
	return trustedDocument{}, invalidSourcePath()
}

func withTrustedConfig(string, Role, func(*os.File) error) error { return invalidSourcePath() }

func checkRootAliases(string, *security.PathGuard) error { return security.ErrProtectedPath }
