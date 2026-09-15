// Package command defines the server-owned policy used to resolve typed
// command identifiers. It deliberately contains no process-launching code.
package command

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

// ValueKind identifies the closed value type accepted by an argument rule.
type ValueKind string

const (
	ValueBool    ValueKind = "bool"
	ValueInteger ValueKind = "integer"
	ValueString  ValueKind = "string"
	ValueEnum    ValueKind = "enum"
	ValuePath    ValueKind = "path"
)

// NetworkPolicy states whether the execution backend must deny network use.
type NetworkPolicy string

const (
	NetworkDenied  NetworkPolicy = "denied"
	NetworkAllowed NetworkPolicy = "allowed"
)

// BinaryIdentity optionally pins the approved executable content.
// SHA256 is the lowercase hexadecimal SHA-256 digest.
type BinaryIdentity struct {
	SHA256 string
}

// Executable defines a server-owned executable ID and approved absolute path.
// The absolute path is internal policy and must not be returned to clients.
type Executable struct {
	ID       string
	Path     string
	Identity *BinaryIdentity
}

// ArgumentRule describes one named, typed request field. Flag is the exact
// argv flag associated with the value; an empty flag denotes a positional.
// Path values must name one or more server-configured roots.
type ArgumentRule struct {
	Name          string
	Flag          string
	Kind          ValueKind
	Required      bool
	AllowedValues []string
	AllowedRoots  []string
	Minimum       *int64
	Maximum       *int64
	MaxLength     int
}

// Limits are server-owned maxima for one command invocation.
type Limits struct {
	Timeout     time.Duration
	StdoutBytes int64
	StderrBytes int64
}

// Definition is the complete typed policy selected by a public command ID.
// FixedArguments commonly hold a fixed subcommand and always precede values
// produced later from ArgumentRules.
type Definition struct {
	ID             string
	ExecutableID   string
	FixedArguments []string
	ArgumentRules  []ArgumentRule
	Limits         Limits
	Network        NetworkPolicy
}

// Registry is an immutable, validated server-side command catalog.
type Registry struct {
	executables map[string]Executable
	definitions map[string]Definition
}

// NewRegistry validates and copies a server-owned catalog. Invalid or
// ambiguous policy fails closed before any command can be resolved.
func NewRegistry(executables []Executable, definitions []Definition) (*Registry, error) {
	r := &Registry{
		executables: make(map[string]Executable, len(executables)),
		definitions: make(map[string]Definition, len(definitions)),
	}

	for _, executable := range executables {
		if err := validateExecutable(executable); err != nil {
			return nil, err
		}
		if _, exists := r.executables[executable.ID]; exists {
			return nil, fmt.Errorf("duplicate executable id %q", executable.ID)
		}
		r.executables[executable.ID] = cloneExecutable(executable)
	}

	for _, definition := range definitions {
		if err := validateDefinition(definition, r.executables); err != nil {
			return nil, err
		}
		if _, exists := r.definitions[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate command id %q", definition.ID)
		}
		r.definitions[definition.ID] = cloneDefinition(definition)
	}

	return r, nil
}

// Resolve returns defensive copies of the definition and its executable.
// Resolution never accepts or performs path lookup from request input.
func (r *Registry) Resolve(commandID string) (Definition, Executable, bool) {
	if r == nil {
		return Definition{}, Executable{}, false
	}
	definition, ok := r.definitions[commandID]
	if !ok {
		return Definition{}, Executable{}, false
	}
	executable := r.executables[definition.ExecutableID]
	return cloneDefinition(definition), cloneExecutable(executable), true
}

func validateExecutable(executable Executable) error {
	if !validIdentifier(executable.ID) {
		return fmt.Errorf("invalid executable id %q", executable.ID)
	}
	if executable.Path == "" || !filepath.IsAbs(executable.Path) || filepath.Clean(executable.Path) != executable.Path {
		return fmt.Errorf("executable %q path must be absolute and clean", executable.ID)
	}
	if strings.ContainsRune(executable.Path, '\x00') {
		return fmt.Errorf("executable %q path contains NUL", executable.ID)
	}
	if executable.Identity != nil && !isLowerHexSHA256(executable.Identity.SHA256) {
		return fmt.Errorf("executable %q has invalid SHA-256 identity", executable.ID)
	}
	return nil
}

func validateDefinition(definition Definition, executables map[string]Executable) error {
	if !validIdentifier(definition.ID) {
		return fmt.Errorf("invalid command id %q", definition.ID)
	}
	if _, ok := executables[definition.ExecutableID]; !ok {
		return fmt.Errorf("command %q references unknown executable id %q", definition.ID, definition.ExecutableID)
	}
	if definition.Limits.Timeout <= 0 || definition.Limits.StdoutBytes <= 0 || definition.Limits.StderrBytes <= 0 {
		return fmt.Errorf("command %q limits must be positive", definition.ID)
	}
	if definition.Network != NetworkDenied && definition.Network != NetworkAllowed {
		return fmt.Errorf("command %q network policy must be explicit", definition.ID)
	}
	for _, value := range definition.FixedArguments {
		if value == "" || strings.ContainsRune(value, '\x00') {
			return fmt.Errorf("command %q has an invalid fixed argument", definition.ID)
		}
	}

	names := make(map[string]struct{}, len(definition.ArgumentRules))
	flags := make(map[string]struct{}, len(definition.ArgumentRules))
	seenPositional := false
	for _, rule := range definition.ArgumentRules {
		if err := validateRule(rule); err != nil {
			return fmt.Errorf("command %q: %w", definition.ID, err)
		}
		if _, exists := names[rule.Name]; exists {
			return fmt.Errorf("command %q has duplicate argument name %q", definition.ID, rule.Name)
		}
		names[rule.Name] = struct{}{}
		if rule.Flag == "" {
			seenPositional = true
			continue
		}
		if seenPositional {
			return fmt.Errorf("command %q has a flag after a positional rule", definition.ID)
		}
		if _, exists := flags[rule.Flag]; exists {
			return fmt.Errorf("command %q has duplicate flag %q", definition.ID, rule.Flag)
		}
		flags[rule.Flag] = struct{}{}
	}
	return nil
}

func validateRule(rule ArgumentRule) error {
	if !validIdentifier(rule.Name) {
		return fmt.Errorf("invalid argument name %q", rule.Name)
	}
	if rule.Flag != "" && (!strings.HasPrefix(rule.Flag, "-") || rule.Flag == "-" || strings.ContainsAny(rule.Flag, "= \t\r\n\x00")) {
		return fmt.Errorf("argument %q has invalid flag", rule.Name)
	}
	switch rule.Kind {
	case ValueBool:
		if rule.Minimum != nil || rule.Maximum != nil || rule.MaxLength != 0 || len(rule.AllowedValues) != 0 || len(rule.AllowedRoots) != 0 {
			return fmt.Errorf("boolean argument %q has incompatible constraints", rule.Name)
		}
	case ValueInteger:
		if rule.Minimum == nil || rule.Maximum == nil || *rule.Minimum > *rule.Maximum || rule.MaxLength != 0 || len(rule.AllowedValues) != 0 || len(rule.AllowedRoots) != 0 {
			return fmt.Errorf("integer argument %q requires a valid closed range", rule.Name)
		}
	case ValueString:
		if rule.MaxLength <= 0 || rule.Minimum != nil || rule.Maximum != nil || len(rule.AllowedValues) != 0 || len(rule.AllowedRoots) != 0 {
			return fmt.Errorf("string argument %q requires only a positive maximum length", rule.Name)
		}
	case ValueEnum:
		if len(rule.AllowedValues) == 0 || rule.Minimum != nil || rule.Maximum != nil || rule.MaxLength != 0 || len(rule.AllowedRoots) != 0 {
			return fmt.Errorf("enum argument %q requires allowed values", rule.Name)
		}
		if hasEmptyOrDuplicate(rule.AllowedValues) {
			return fmt.Errorf("enum argument %q has invalid allowed values", rule.Name)
		}
	case ValuePath:
		if len(rule.AllowedRoots) == 0 || rule.Minimum != nil || rule.Maximum != nil || rule.MaxLength <= 0 || len(rule.AllowedValues) != 0 {
			return fmt.Errorf("path argument %q requires roots and a positive maximum length", rule.Name)
		}
		if hasInvalidIdentifiers(rule.AllowedRoots) {
			return fmt.Errorf("path argument %q has invalid allowed roots", rule.Name)
		}
	default:
		return fmt.Errorf("argument %q has invalid value kind %q", rule.Name, rule.Kind)
	}
	return nil
}

func validIdentifier(value string) bool { return identifierPattern.MatchString(value) }

func isLowerHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func hasEmptyOrDuplicate(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" || strings.ContainsRune(value, '\x00') {
			return true
		}
		if _, exists := seen[value]; exists {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func hasInvalidIdentifiers(values []string) bool {
	if hasEmptyOrDuplicate(values) {
		return true
	}
	for _, value := range values {
		if !validIdentifier(value) {
			return true
		}
	}
	return false
}

func cloneExecutable(executable Executable) Executable {
	if executable.Identity != nil {
		identity := *executable.Identity
		executable.Identity = &identity
	}
	return executable
}

func cloneDefinition(definition Definition) Definition {
	definition.FixedArguments = append([]string(nil), definition.FixedArguments...)
	definition.ArgumentRules = append([]ArgumentRule(nil), definition.ArgumentRules...)
	for index := range definition.ArgumentRules {
		rule := &definition.ArgumentRules[index]
		rule.AllowedValues = append([]string(nil), rule.AllowedValues...)
		rule.AllowedRoots = append([]string(nil), rule.AllowedRoots...)
		if rule.Minimum != nil {
			minimum := *rule.Minimum
			rule.Minimum = &minimum
		}
		if rule.Maximum != nil {
			maximum := *rule.Maximum
			rule.Maximum = &maximum
		}
	}
	return definition
}
