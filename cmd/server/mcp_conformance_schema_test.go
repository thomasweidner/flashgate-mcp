package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/thomasweidner/flashgate-mcp/internal/mcptest"
	"github.com/thomasweidner/flashgate-mcp/internal/protocol"
)

type mcpToolSchemas struct {
	input, output *jsonschema.Schema
}

func mcpCompileSchemas(definitions []protocol.Tool) (map[string]mcpToolSchemas, error) {
	schemas := make(map[string]mcpToolSchemas, len(definitions))
	for _, definition := range definitions {
		input, err := mcptest.CompileToolSchema(definition.InputSchema, true)
		if err != nil {
			return nil, fmt.Errorf("%s/inputSchema: %w", definition.Name, err)
		}
		output, err := mcptest.CompileToolSchema(definition.OutputSchema, false)
		if err != nil {
			return nil, fmt.Errorf("%s/outputSchema: %w", definition.Name, err)
		}
		schemas[definition.Name] = mcpToolSchemas{input, output}
	}
	return schemas, nil
}

func mcpRequireSchemaInstance(t *testing.T, label string, schema *jsonschema.Schema, raw []byte) {
	t.Helper()
	value, err := mcptest.SchemaInstance(raw)
	if err == nil {
		err = schema.Validate(value)
	}
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
}

func mcpCheckCatalog(t *testing.T, definitions []protocol.Tool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "mcp-tool-catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := mcpCatalogParity(t, raw, definitions); err != nil {
		t.Fatal(err)
	}
}

func mcpCatalogParity(t *testing.T, raw []byte, definitions []protocol.Tool) error {
	t.Helper()
	var catalog struct {
		Tools []map[string]json.RawMessage `json:"tools"`
	}
	if err := json.Unmarshal(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}), &catalog); err != nil {
		return err
	}
	if len(catalog.Tools) != len(definitions) {
		return fmt.Errorf("catalog has %d tools, registry has %d", len(catalog.Tools), len(definitions))
	}
	for i, definition := range definitions {
		entry := catalog.Tools[i]
		// Compare the full public tool contract after adapting the catalog's
		// resultSchema field to the wire name. Other catalog fields are prose.
		contract := map[string]json.RawMessage{
			"name": entry["name"], "title": entry["title"], "description": entry["description"],
			"inputSchema": entry["inputSchema"], "outputSchema": entry["resultSchema"], "annotations": entry["annotations"],
		}
		if len(entry["title"]) == 0 {
			delete(contract, "title")
		}
		if err := mcpAnnotations(canonicalMCP(t, contract)); err != nil {
			return fmt.Errorf("%s catalog annotations: %w", definition.Name, err)
		}
		if !bytes.Equal(canonicalMCP(t, definition), canonicalMCP(t, contract)) {
			return fmt.Errorf("%s runtime definition differs from catalog entry %d", definition.Name, i)
		}
	}
	return nil
}

func TestMCPConformanceSchema202012(t *testing.T) {
	const object = `{"type":"object","additionalProperties":false}`
	for _, test := range []struct {
		name, input, output, want string
	}{
		{"missing-input", "null", object, "/inputSchema"},
		{"missing-output", object, "null", "/outputSchema"},
		{"open-input", `{"type":"object"}`, object, "/additionalProperties"},
		{"nonobject-output", object, `{"type":"string"}`, "/outputSchema"},
		{"bad-input-metaschema", `{"type":"object","additionalProperties":false,"required":["x","x"]}`, object, "/required"},
		{"bad-output-metaschema", object, `{"type":"object","properties":{"x":{"type":"number","minimum":"1"}}}`, "/properties/x/minimum"},
		{"legacy-dialect", object, `{"type":"object","$schema":"http://json-schema.org/draft-07/schema#"}`, "/$schema"},
		{"unknown-dialect", object, `{"type":"object","$schema":"https://unregistered.invalid/schema"}`, "/$schema"},
		{"nested-dialect", object, `{"type":"object","$defs":{"x":{"$id":"x","$schema":"https://json-schema.org/draft/2019-09/schema"}}}`, "/$defs/x/$schema"},
		{"remote-reference", object, `{"type":"object","$ref":"https://unregistered.invalid/schema"}`, "not registered"},
		{"file-reference", object, `{"type":"object","$ref":"file:///unregistered.json"}`, "not registered"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output map[string]any
			if err := json.Unmarshal([]byte(test.output), &output); err != nil {
				t.Fatal(err)
			}
			_, err := mcpCompileSchemas([]protocol.Tool{{Name: "synthetic", InputSchema: json.RawMessage(test.input), OutputSchema: output}})
			if err == nil || !strings.Contains(err.Error(), "synthetic/") || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected synthetic schema error containing %q, got %v", test.want, err)
			}
		})
	}
	t.Run("draft-keywords-and-format-annotation", func(t *testing.T) {
		raw := json.RawMessage(`{
			"$schema":"https://json-schema.org/draft/2020-12/schema",
			"type":"object","additionalProperties":false,"required":["values","label"],
			"$defs":{"positive":{"type":"integer","minimum":1}},
			"properties":{
				"values":{"type":"array","prefixItems":[{"$ref":"#/$defs/positive"}],"items":false},
				"label":{"type":"string","format":"email"}
			}
		}`)
		schema, err := mcptest.CompileToolSchema(raw, true)
		if err != nil {
			t.Fatal(err)
		}
		mcpRequireSchemaInstance(t, "2020-12 instance", schema, []byte(`{"values":[9007199254740993],"label":"annotation"}`))
		for _, raw := range []string{`{"values":[0],"label":"ok"}`, `{"values":[1,2],"label":"ok"}`} {
			value, err := mcptest.SchemaInstance([]byte(raw))
			if err != nil || schema.Validate(value) == nil {
				t.Fatalf("expected Draft 2020-12 keyword to reject %s (decode: %v)", raw, err)
			}
		}
	})
	t.Run("instance-and-strict-decoder-negatives", func(t *testing.T) {
		registry, _ := mcpRegistry(t, false)
		tool, ok := registry.Get("read_file")
		if !ok {
			t.Fatal("read_file fixture missing")
		}
		schemas, err := mcpCompileSchemas([]protocol.Tool{tool.Definition()})
		if err != nil {
			t.Fatal(err)
		}
		schema := schemas[tool.Name()]
		for _, raw := range []string{`{"path":42}`, `{}`, `{"path":"seed.txt","extra":true}`, `{"path":"seed.txt","maxBytes":0}`, `{"path":null}`, `null`, `{"path":"seed.txt"} {}`} {
			value, err := mcptest.SchemaInstance([]byte(raw))
			if err == nil && schema.input.Validate(value) == nil {
				t.Errorf("input schema accepted %s", raw)
			}
			if _, err := tool.Execute(context.Background(), json.RawMessage(raw)); err == nil {
				t.Errorf("strict decoder accepted %s", raw)
			}
		}
		for _, raw := range []string{`{"content":["seed"],"size":4}`, `{"content":"seed"}`, `{"content":"seed","size":4,"extra":true}`, `null`} {
			value, err := mcptest.SchemaInstance([]byte(raw))
			if err != nil || schema.output.Validate(value) == nil {
				t.Errorf("expected output schema to reject %s (decode: %v)", raw, err)
			}
		}
	})
	t.Run("catalog-drift", func(t *testing.T) {
		registry, _ := mcpRegistry(t, false)
		definitions := mcpDefinitions(registry)
		raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "mcp-tool-catalog.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := mcpCatalogParity(t, raw, definitions); err != nil {
			t.Fatal(err)
		}
		for _, mutation := range []struct{ before, after string }{
			{`"minLength": 1`, `"minLength": 2`},
			{`"openWorldHint": false`, `"unknownHint": false`},
		} {
			mutated := bytes.Replace(raw, []byte(mutation.before), []byte(mutation.after), 1)
			if bytes.Equal(raw, mutated) || mcpCatalogParity(t, mutated, definitions) == nil {
				t.Fatalf("catalog mutation escaped: %s", mutation.before)
			}
		}
	})
}
