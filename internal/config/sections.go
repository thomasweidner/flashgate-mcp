package config

import (
	"errors"
	"reflect"

	"github.com/pelletier/go-toml/v2"
)

func (s *settings) decode(document []byte) error {
	// TOML verifies table shape and duplicate declarations before flat
	// sections are split into their fixed options and named definitions.
	wire := struct {
		commonSettings
		Files    map[string]any `toml:"files"`
		Profiles map[string]any `toml:"profiles"`
	}{commonSettings: s.commonSettings}
	if err := decodeTOML(document, &wire); err != nil {
		return err
	}
	if err := decodeFlatSection(wire.Files, &s.Files.fileOptions, &s.Files.Roots, "roots"); err != nil {
		return err
	}
	if err := decodeFlatSection(wire.Profiles, &s.Profiles.profileOptions, &s.Profiles.Definitions, "definitions"); err != nil {
		return err
	}
	s.commonSettings = wire.commonSettings
	return nil
}

// Use the typed options' TOML tags as the reservation source. A new option
// reserves its name in the same change that declares the option.
func decodeFlatSection[Options any, ID ~string, Definition any](
	section map[string]any, options *Options, definitions *map[ID]Definition, retired string,
) error {
	optionNames := make(map[string]bool)
	optionType := reflect.TypeFor[Options]()
	for i := range optionType.NumField() {
		optionNames[optionType.Field(i).Tag.Get("toml")] = true
	}
	fixed, named := make(map[string]any), make(map[string]any)
	for key, value := range section {
		if key == retired {
			return errors.New("retired configuration namespace")
		}
		if optionNames[key] {
			if _, table := value.(map[string]any); table {
				return errors.New("configuration option is reserved")
			}
			fixed[key] = value
			continue
		}
		if _, table := value.(map[string]any); !table {
			return errors.New("expected a named configuration table")
		}
		named[key] = value
	}
	if err := decodeTOMLFields(fixed, options); err != nil {
		return err
	}
	return decodeTOMLFields(named, definitions)
}

func decodeTOMLFields(fields map[string]any, target any) error {
	if len(fields) == 0 {
		return nil
	}
	encoded, err := toml.Marshal(fields)
	if err != nil {
		return err
	}
	return decodeTOML(encoded, target)
}
