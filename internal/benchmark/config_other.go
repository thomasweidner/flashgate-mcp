//go:build !windows

package benchmark

import (
	"errors"
	"os"

	"github.com/thomasweidner/flashgate-mcp/internal/configfixture"
)

func makeConfigFixtureDir() (string, error) {
	dir, err := os.MkdirTemp("", "flashgate-bench-config-")
	if err != nil {
		return "", errors.New("configuration fixture directory creation failed")
	}
	return dir, nil
}

func writeConfigFixture(path string, data []byte) error {
	return configfixture.Write(path, data)
}
