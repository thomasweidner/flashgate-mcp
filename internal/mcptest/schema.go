package mcptest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// SchemaInstance decodes one complete JSON value without rounding numbers.
func SchemaInstance(raw []byte) (any, error) {
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

type offlineSchemaLoader struct{}

func (offlineSchemaLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("schema resource is not registered: %s", url)
}

// CompileToolSchema applies the FlashGate root rules and delegates standard
// metaschema and instance semantics to the pinned Draft 2020-12 compiler.
func CompileToolSchema(schema any, closedInput bool) (*jsonschema.Schema, error) {
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	value, err := SchemaInstance(raw)
	if err != nil {
		return nil, err
	}
	root, ok := value.(map[string]any)
	if !ok || root["type"] != "object" {
		return nil, fmt.Errorf("root schema must have type object")
	}
	if closedInput && root["additionalProperties"] != false {
		return nil, fmt.Errorf("/additionalProperties must be false for the input root")
	}

	const location = "https://flashgate.invalid/tool-schema"
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.UseLoader(offlineSchemaLoader{})
	if err := checkSchemaDialect(root, ""); err != nil {
		return nil, err
	}
	if err := compiler.AddResource(location, root); err != nil {
		return nil, err
	}
	return compiler.Compile(location)
}

// Walk schema positions, leaving instance-valued keywords such as const and
// examples to the compiler. In particular, embedded resources can otherwise
// select an older dialect even with the compiler's default pinned.
func checkSchemaDialect(schema any, path string) error {
	object, ok := schema.(map[string]any)
	if !ok {
		return nil // The metaschema checks the type at this position.
	}
	if dialect, present := object["$schema"]; present && dialect != jsonschema.Draft2020.String() {
		return fmt.Errorf("%s/$schema: expected %q, got %v", path, jsonschema.Draft2020, dialect)
	}
	for _, key := range []string{"$defs", "definitions", "properties", "patternProperties", "dependentSchemas", "dependencies"} {
		children, _ := object[key].(map[string]any)
		names := make([]string, 0, len(children))
		for name := range children {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			escaped := strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
			if err := checkSchemaDialect(children[name], path+"/"+key+"/"+escaped); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		children, _ := object[key].([]any)
		for index, child := range children {
			if err := checkSchemaDialect(child, fmt.Sprintf("%s/%s/%d", path, key, index)); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{
		"additionalProperties", "unevaluatedProperties", "propertyNames",
		"items", "contains", "unevaluatedItems", "contentSchema", "not", "if", "then", "else",
	} {
		if err := checkSchemaDialect(object[key], path+"/"+key); err != nil {
			return err
		}
	}
	return nil
}
