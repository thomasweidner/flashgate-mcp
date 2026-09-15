package version

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestReleaseWorkflowRemainsTagGated(t *testing.T) {
	t.Parallel()

	workflowPath := filepath.Join("..", "..", ".github", "workflows", "release-build.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	required := map[string]string{
		"tag-gated release entry points": `(?m)^on:\n  push:\n    tags:\n      - 'v\*\.\*\.\*'\n  workflow_dispatch:\n    inputs:\n      version:\n        description: Semantic release version without leading v\n        required: true\n        type: string\n\npermissions:$`,
		"tag-derived version":          `(?m)^          if \[\[ "\$GITHUB_REF_TYPE" == "tag" \]\]; then\n            version="\$\{GITHUB_REF_NAME#v\}"$`,
		"exact matching source tag":    `(?m)^          exact_tag="\$\(git describe --tags --exact-match HEAD\)"\n          \[\[ "\$exact_tag" == "v\$version" \]\]$`,
		"read-only repository access":  `(?m)^permissions:\n  contents: read$`,
	}

	for name, pattern := range required {
		t.Run(name, func(t *testing.T) {
			matched, err := regexp.Match(pattern, workflow)
			if err != nil {
				t.Fatalf("compile workflow assertion: %v", err)
			}
			if !matched {
				t.Fatalf("release workflow no longer satisfies %s", name)
			}
		})
	}
}
