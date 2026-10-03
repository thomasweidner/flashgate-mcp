package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var timeType = reflect.TypeOf(time.Time{})

// decodeStrictJSON decodes one complete JSON value while rejecting ambiguous or
// schema-incompatible representations before populating the typed destination.
func decodeStrictJSON(data []byte, destination any) error {
	destinationType := reflect.TypeOf(destination)
	if destinationType == nil || destinationType.Kind() != reflect.Pointer || reflect.ValueOf(destination).IsNil() {
		return fmt.Errorf("destination must be a non-nil pointer")
	}
	if err := validateJSONUnicode(data); err != nil {
		return err
	}

	shapeDecoder := json.NewDecoder(bytes.NewReader(data))
	shapeDecoder.UseNumber()
	shape, err := decodeUniqueJSONValue(shapeDecoder, "$")
	if err != nil {
		return err
	}
	if token, trailingErr := shapeDecoder.Token(); trailingErr != io.EOF {
		if trailingErr != nil {
			return fmt.Errorf("trailing JSON data: %w", trailingErr)
		}
		return fmt.Errorf("trailing JSON data begins with %v", token)
	}
	if err := validateJSONShape(shape, destinationType.Elem(), "$"); err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode typed JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err != nil {
			return fmt.Errorf("trailing JSON data: %w", err)
		}
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

// validateJSONUnicode closes encoding/json's intentionally permissive handling
// of malformed UTF-8 and invalid UTF-16 surrogate escapes. Replacement
// characters that are genuinely present in the input remain valid.
func validateJSONUnicode(data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("invalid JSON Unicode: malformed UTF-8")
	}

	for index := 0; index < len(data); index++ {
		if data[index] != '"' {
			continue
		}
		for index++; index < len(data) && data[index] != '"'; index++ {
			if data[index] != '\\' {
				continue
			}
			index++
			if index >= len(data) || data[index] != 'u' {
				continue
			}
			codeUnit, ok := parseJSONHexCodeUnit(data, index+1)
			if !ok {
				continue // The JSON decoder reports malformed escape syntax.
			}
			index += 4
			switch {
			case codeUnit >= 0xDC00 && codeUnit <= 0xDFFF:
				return fmt.Errorf("invalid JSON Unicode: unpaired low surrogate escape")
			case codeUnit >= 0xD800 && codeUnit <= 0xDBFF:
				if index+6 >= len(data) || data[index+1] != '\\' || data[index+2] != 'u' {
					return fmt.Errorf("invalid JSON Unicode: unpaired high surrogate escape")
				}
				low, validLow := parseJSONHexCodeUnit(data, index+3)
				if !validLow || low < 0xDC00 || low > 0xDFFF {
					return fmt.Errorf("invalid JSON Unicode: unpaired high surrogate escape")
				}
				index += 6
			}
		}
	}
	return nil
}

func parseJSONHexCodeUnit(data []byte, start int) (uint16, bool) {
	if start+4 > len(data) {
		return 0, false
	}
	var value uint16
	for _, character := range data[start : start+4] {
		value <<= 4
		switch {
		case character >= '0' && character <= '9':
			value += uint16(character - '0')
		case character >= 'a' && character <= 'f':
			value += uint16(character-'a') + 10
		case character >= 'A' && character <= 'F':
			value += uint16(character-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}

func decodeUniqueJSONValue(decoder *json.Decoder, path string) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", path, err)
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return token, nil
	}

	switch delimiter {
	case '{':
		object := make(map[string]any)
		for decoder.More() {
			nameToken, err := decoder.Token()
			if err != nil {
				return nil, fmt.Errorf("%s: invalid object field: %w", path, err)
			}
			name, ok := nameToken.(string)
			if !ok {
				return nil, fmt.Errorf("%s: object field name is not a string", path)
			}
			fieldPath := appendJSONPath(path, name)
			if _, exists := object[name]; exists {
				return nil, fmt.Errorf("%s: duplicate JSON field", fieldPath)
			}
			value, err := decodeUniqueJSONValue(decoder, fieldPath)
			if err != nil {
				return nil, err
			}
			object[name] = value
		}
		closing, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%s: unterminated object: %w", path, err)
		}
		if closing != json.Delim('}') {
			return nil, fmt.Errorf("%s: invalid object terminator", path)
		}
		return object, nil
	case '[':
		array := make([]any, 0)
		for index := 0; decoder.More(); index++ {
			value, err := decodeUniqueJSONValue(decoder, fmt.Sprintf("%s[%d]", path, index))
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		closing, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("%s: unterminated array: %w", path, err)
		}
		if closing != json.Delim(']') {
			return nil, fmt.Errorf("%s: invalid array terminator", path)
		}
		return array, nil
	default:
		return nil, fmt.Errorf("%s: unexpected JSON delimiter %q", path, delimiter)
	}
}

func validateJSONShape(value any, targetType reflect.Type, path string) error {
	for targetType.Kind() == reflect.Pointer {
		if value == nil {
			return fmt.Errorf("%s: null is not allowed", path)
		}
		targetType = targetType.Elem()
	}
	if value == nil {
		return fmt.Errorf("%s: null is not allowed", path)
	}
	if targetType == timeType {
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: expected date-time string", path)
		}
		return nil
	}

	switch targetType.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object", path)
		}
		fields := make(map[string]reflect.StructField, targetType.NumField())
		for index := 0; index < targetType.NumField(); index++ {
			field := targetType.Field(index)
			name, options := jsonFieldName(field)
			if name == "-" || name == "" {
				continue
			}
			fields[name] = field
			fieldValue, exists := object[name]
			if !exists {
				if !options["omitempty"] {
					return fmt.Errorf("%s: missing required field", appendJSONPath(path, name))
				}
				continue
			}
			if err := validateJSONShape(fieldValue, field.Type, appendJSONPath(path, name)); err != nil {
				return err
			}
		}
		unknown := make([]string, 0)
		for name := range object {
			if _, ok := fields[name]; !ok {
				unknown = append(unknown, name)
			}
		}
		sort.Strings(unknown)
		if len(unknown) > 0 {
			return fmt.Errorf("%s: unknown field", appendJSONPath(path, unknown[0]))
		}
		return nil
	case reflect.Slice, reflect.Array:
		array, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: expected array", path)
		}
		for index := range array {
			if err := validateJSONShape(array[index], targetType.Elem(), fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object", path)
		}
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := validateJSONShape(object[key], targetType.Elem(), appendJSONPath(path, key)); err != nil {
				return err
			}
		}
		return nil
	case reflect.String:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: expected string", path)
		}
	case reflect.Bool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected boolean", path)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		if _, ok := value.(json.Number); !ok {
			return fmt.Errorf("%s: expected number", path)
		}
	case reflect.Interface:
		return nil
	default:
		return fmt.Errorf("%s: unsupported JSON destination type %s", path, targetType)
	}
	return nil
}

func jsonFieldName(field reflect.StructField) (string, map[string]bool) {
	tag := field.Tag.Get("json")
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = field.Name
	}
	options := make(map[string]bool, len(parts)-1)
	for _, option := range parts[1:] {
		options[option] = true
	}
	return name, options
}

func appendJSONPath(path string, field string) string {
	if field != "" && strings.IndexFunc(field, func(r rune) bool {
		return !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) == -1 {
		return path + "." + field
	}
	encoded, _ := json.Marshal(field)
	return path + "[" + string(encoded) + "]"
}
