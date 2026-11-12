package benchmark

import (
	"os"
	"path/filepath"

	"github.com/thomasweidner/flashgate-mcp/internal/config"
	"github.com/thomasweidner/flashgate-mcp/internal/configfixture"
)

func makeConfigFixtureDir() (string, error) {
	var dir string
	err := config.WithTrustedUserConfigParent(filepath.Join(os.TempDir(), "fixture.toml"), func(path string) error {
		var err error
		dir, err = os.MkdirTemp(filepath.Dir(path), "flashgate-bench-config-")
		if err != nil {
			return config.NewError(config.CategoryUntrustedConfig, nil)
		}
		return nil
	})
	return dir, err
}

func writeConfigFixture(path string, data []byte) error {
	return config.WithTrustedUserConfigParent(path, func(checked string) error {
		return configfixture.Write(checked, data)
	})
}
