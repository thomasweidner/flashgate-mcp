package command

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// PathArgument is the only request representation accepted for a path value.
// RootID selects one of the rule's approved roots; Path is relative to it.
type PathArgument struct {
	RootID string
	Path   string
}

// PathResolver binds a validated root-relative path to an effective path. The
// implementation is responsible for the platform-specific containment and
// link/reparse checks immediately before process creation.
type PathResolver interface {
	ResolveCommandPath(rootID, relativePath string) (string, error)
}

// Invocation is process-launch input with an executable path and argv kept
// separate. It cannot represent a shell command string.
type Invocation struct {
	ExecutablePath string
	Arguments      []string
}

// BuildInvocation resolves commandID and deterministically converts a closed
// structured argument object into argv. Values are limited to bool, integer,
// string, and PathArgument according to the selected definition.
func (r *Registry) BuildInvocation(commandID string, values map[string]any, paths PathResolver) (Invocation, error) {
	definition, executable, ok := r.Resolve(commandID)
	if !ok {
		return Invocation{}, fmt.Errorf("unknown command id %q", commandID)
	}
	if values == nil {
		values = map[string]any{}
	}

	rules := make(map[string]ArgumentRule, len(definition.ArgumentRules))
	for _, rule := range definition.ArgumentRules {
		rules[rule.Name] = rule
	}
	for name := range values {
		if _, ok := rules[name]; !ok {
			return Invocation{}, fmt.Errorf("unknown argument %q", name)
		}
	}

	argv := append([]string(nil), definition.FixedArguments...)
	for _, rule := range definition.ArgumentRules {
		value, present := values[rule.Name]
		if !present {
			if rule.Required {
				return Invocation{}, fmt.Errorf("missing required argument %q", rule.Name)
			}
			continue
		}
		encoded, include, err := encodeArgument(rule, value, paths)
		if err != nil {
			return Invocation{}, err
		}
		if !include {
			continue
		}
		if rule.Flag != "" {
			argv = append(argv, rule.Flag)
		}
		argv = append(argv, encoded...)
	}

	return Invocation{ExecutablePath: executable.Path, Arguments: argv}, nil
}

func encodeArgument(rule ArgumentRule, value any, paths PathResolver) ([]string, bool, error) {
	switch rule.Kind {
	case ValueBool:
		boolean, ok := value.(bool)
		if !ok {
			return nil, false, invalidType(rule)
		}
		if rule.Flag == "" {
			return nil, false, fmt.Errorf("boolean argument %q requires a flag", rule.Name)
		}
		return nil, boolean, nil
	case ValueInteger:
		integer, ok := value.(int64)
		if !ok || integer < *rule.Minimum || integer > *rule.Maximum {
			return nil, false, fmt.Errorf("argument %q is not an integer within policy", rule.Name)
		}
		return []string{fmt.Sprintf("%d", integer)}, true, nil
	case ValueString:
		text, ok := value.(string)
		if !ok || !validTextValue(text, rule.MaxLength) || beginsOptionOrResponse(text) {
			return nil, false, fmt.Errorf("argument %q is not a safe bounded string", rule.Name)
		}
		return []string{text}, true, nil
	case ValueEnum:
		text, ok := value.(string)
		if !ok || !contains(rule.AllowedValues, text) {
			return nil, false, fmt.Errorf("argument %q is not an allowed value", rule.Name)
		}
		return []string{text}, true, nil
	case ValuePath:
		path, ok := value.(PathArgument)
		if !ok || !contains(rule.AllowedRoots, path.RootID) || !validRelativePath(path.Path, rule.MaxLength) {
			return nil, false, fmt.Errorf("argument %q is not an allowed root-relative path", rule.Name)
		}
		if paths == nil {
			return nil, false, fmt.Errorf("argument %q requires path resolution", rule.Name)
		}
		resolved, err := paths.ResolveCommandPath(path.RootID, path.Path)
		if err != nil || resolved == "" || !filepath.IsAbs(resolved) || filepath.Clean(resolved) != resolved {
			return nil, false, fmt.Errorf("argument %q path resolution failed", rule.Name)
		}
		return []string{resolved}, true, nil
	default:
		return nil, false, invalidType(rule)
	}
}

func invalidType(rule ArgumentRule) error {
	return fmt.Errorf("argument %q has the wrong type", rule.Name)
}

func validTextValue(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && len(value) <= maximum && !strings.ContainsAny(value, "\x00\r\n")
}

func beginsOptionOrResponse(value string) bool {
	return strings.HasPrefix(value, "-") || isResponseFile(value)
}

func validRelativePath(value string, maximum int) bool {
	if !validTextValue(value, maximum) || filepath.IsAbs(value) || filepath.Clean(value) != value || value == "." || beginsOptionOrResponse(value) {
		return false
	}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part == ".." {
			return false
		}
	}
	return true
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
