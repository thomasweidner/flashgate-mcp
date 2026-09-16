package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestAllToolSchemasDeclareAndConformToJSONSchema202012(t *testing.T) {
	fake := newFakeFileSystem()
	runtimeTools := []Tool{
		NewListDirectoryTool(fake), NewReadFileTool(fake, 1024), NewGetPathInfoTool(fake),
		NewSystemInfoTool(fakeSystemInfoProvider{}), NewWriteFileTool(fake),
		NewCreateDirectoryTool(fake), NewDeletePathTool(fake), NewCopyPathTool(fake),
		NewMovePathTool(fake),
	}

	for _, runtimeTool := range runtimeTools {
		definition := runtimeTool.Definition()
		for _, candidate := range []struct {
			name   string
			schema any
		}{{"inputSchema", definition.InputSchema}, {"outputSchema", definition.OutputSchema}} {
			t.Run(runtimeTool.Name()+"/"+candidate.name, func(t *testing.T) {
				schema := normalizeSchema(t, candidate.schema)
				if schema["$schema"] != jsonSchema202012Dialect {
					t.Fatalf("$schema=%#v, want %q", schema["$schema"], jsonSchema202012Dialect)
				}
				if err := validateSchemaDocument("$", schema, true); err != nil {
					t.Fatal(err)
				}
				first, err := json.Marshal(schema)
				if err != nil {
					t.Fatal(err)
				}
				for range 10 {
					next, err := json.Marshal(schema)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(first, next) {
						t.Fatal("schema serialization is not deterministic")
					}
				}
			})
		}
	}
}

func validateSchemaDocument(path string, schema map[string]any, root bool) error {
	allowed := map[string]bool{
		"$schema": true, "type": true, "properties": true, "required": true,
		"additionalProperties": true, "items": true, "oneOf": true, "const": true,
		"minimum": true, "minLength": true, "description": true,
	}
	for keyword := range schema {
		if !allowed[keyword] {
			return fmt.Errorf("%s uses unvalidated JSON Schema keyword %q", path, keyword)
		}
	}
	if root {
		if schema["$schema"] != jsonSchema202012Dialect {
			return fmt.Errorf("%s does not declare the JSON Schema 2020-12 dialect", path)
		}
	} else if _, ok := schema["$schema"]; ok {
		return fmt.Errorf("%s redundantly changes the schema dialect", path)
	}
	if rawType, ok := schema["type"]; ok {
		typeName, ok := rawType.(string)
		if !ok || !map[string]bool{"object": true, "array": true, "string": true, "boolean": true, "integer": true}[typeName] {
			return fmt.Errorf("%s has invalid type %#v", path, rawType)
		}
	}
	if description, ok := schema["description"]; ok {
		if text, ok := description.(string); !ok || text == "" {
			return fmt.Errorf("%s has invalid description %#v", path, description)
		}
	}
	if minimum, ok := schema["minimum"]; ok {
		if !isJSONNumber(minimum) {
			return fmt.Errorf("%s has non-number minimum %#v", path, minimum)
		}
	}
	if minLength, ok := schema["minLength"]; ok {
		if !isNonnegativeInteger(minLength) {
			return fmt.Errorf("%s has invalid minLength %#v", path, minLength)
		}
	}
	if additional, ok := schema["additionalProperties"]; ok {
		if _, ok := additional.(bool); !ok {
			return fmt.Errorf("%s has non-boolean additionalProperties %#v", path, additional)
		}
	}

	properties, hasProperties := schema["properties"].(map[string]any)
	if rawProperties, ok := schema["properties"]; ok && !hasProperties {
		return fmt.Errorf("%s has invalid properties %#v", path, rawProperties)
	}
	for name, rawChild := range properties {
		child, ok := rawChild.(map[string]any)
		if !ok {
			return fmt.Errorf("%s.properties.%s is not a schema object", path, name)
		}
		if err := validateSchemaDocument(path+".properties."+name, child, false); err != nil {
			return err
		}
	}
	if rawRequired, ok := schema["required"]; ok {
		required, ok := rawRequired.([]any)
		if !ok || !hasProperties {
			return fmt.Errorf("%s has invalid required %#v", path, rawRequired)
		}
		seen := map[string]bool{}
		for _, rawName := range required {
			name, ok := rawName.(string)
			if !ok || seen[name] {
				return fmt.Errorf("%s has invalid required entry %#v", path, rawName)
			}
			seen[name] = true
			if _, ok := properties[name]; !ok {
				return fmt.Errorf("%s requires undefined property %q", path, name)
			}
		}
	}
	if rawItems, ok := schema["items"]; ok {
		items, ok := rawItems.(map[string]any)
		if !ok {
			return fmt.Errorf("%s has invalid items %#v", path, rawItems)
		}
		if err := validateSchemaDocument(path+".items", items, false); err != nil {
			return err
		}
	}
	if rawVariants, ok := schema["oneOf"]; ok {
		variants, ok := rawVariants.([]any)
		if !ok || len(variants) == 0 {
			return fmt.Errorf("%s has invalid oneOf %#v", path, rawVariants)
		}
		for index, rawVariant := range variants {
			variant, ok := rawVariant.(map[string]any)
			if !ok {
				return fmt.Errorf("%s.oneOf[%d] is not a schema object", path, index)
			}
			if err := validateSchemaDocument(fmt.Sprintf("%s.oneOf[%d]", path, index), variant, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func isJSONNumber(value any) bool {
	switch value.(type) {
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		return true
	default:
		return false
	}
}

func isNonnegativeInteger(value any) bool {
	switch number := value.(type) {
	case int:
		return number >= 0
	case int64:
		return number >= 0
	case float64:
		return number >= 0 && number == float64(int64(number))
	case json.Number:
		integer, err := number.Int64()
		return err == nil && integer >= 0
	default:
		return false
	}
}
