package benchmark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBaselineSchemaNumericAndExitStatusContractMatchesGo(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "baseline.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}

	defs := schema["$defs"].(map[string]any)
	assertIntegerContract(t, property(t, schema, "repetitions"), 1, float64(benchmarkJSONMaxInt))
	for _, definition := range []string{"startMeasurement", "workflowMeasurement"} {
		exitStatuses := property(t, defs[definition].(map[string]any), "exit_statuses")
		if got := exitStatuses["propertyNames"].(map[string]any)["pattern"]; got != `^-?(0|[1-9][0-9]*)$` {
			t.Fatalf("%s exit-status key pattern=%v", definition, got)
		}
		assertIntegerContract(t, exitStatuses["additionalProperties"].(map[string]any), 0, float64(benchmarkJSONMaxInt))
	}

	// This inventory pins every reusable nested object to a closed, required
	// representation so a schema-only nested field cannot drift from strict Go decoding.
	for _, definition := range []string{"metricSummary", "resources", "startMeasurement", "toolsListMeasurement", "workflowMeasurement"} {
		object := defs[definition].(map[string]any)
		if object["type"] != "object" || object["additionalProperties"] != false {
			t.Fatalf("$defs.%s must remain a closed object", definition)
		}
		properties := object["properties"].(map[string]any)
		required := object["required"].([]any)
		if len(required) != len(properties) && definition != "resources" {
			t.Fatalf("$defs.%s required=%d properties=%d", definition, len(required), len(properties))
		}
	}
}

func property(t *testing.T, object map[string]any, name string) map[string]any {
	t.Helper()
	return object["properties"].(map[string]any)[name].(map[string]any)
}

func assertIntegerContract(t *testing.T, schema map[string]any, minimum, maximum float64) {
	t.Helper()
	if schema["type"] != "integer" || schema["minimum"] != minimum || schema["maximum"] != maximum {
		t.Fatalf("integer contract=%v, want minimum=%v maximum=%v", schema, minimum, maximum)
	}
}
