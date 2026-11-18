package config

import (
	"errors"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
)

var errFileTypePolicy = errors.New("invalid file type policy")

// FileTypePolicy is an immutable name restriction, not an authorization grant
// or a content classifier. The zero value adds no extension restriction.
type FileTypePolicy struct {
	extensions        *extensionSet
	restricted        bool
	denyExtensionless bool
}

type extensionSet struct{ values []string }

// NewFileTypePolicy validates and copies a policy. A nil list is unrestricted;
// a non-nil empty list denies every suffixed name. The boolean is effective.
func NewFileTypePolicy(extensions []string, allowExtensionless bool) (FileTypePolicy, error) {
	p := FileTypePolicy{restricted: extensions != nil, denyExtensionless: !allowExtensionless}
	seen := make(map[string]bool, len(extensions))
	for _, extension := range extensions {
		if !validExtension(extension) || seen[extension] {
			return FileTypePolicy{}, errFileTypePolicy
		}
		seen[extension] = true
	}
	if extensions != nil {
		values := slices.Clone(extensions)
		slices.Sort(values)
		p.extensions = &extensionSet{values: values}
	}
	return p, nil
}

// AllowedExtensions returns an independent snapshot, retaining nil versus [].
func (p FileTypePolicy) AllowedExtensions() []string {
	if p.extensions == nil {
		return nil
	}
	return slices.Clone(p.extensions.values)
}
func (p FileTypePolicy) AllowExtensionless() bool { return !p.denyExtensionless }
func (p FileTypePolicy) Restricted() bool         { return p.restricted || p.denyExtensionless }

// AllowsName matches a final basename without native normalization or I/O.
// With no effective restriction it adds no portable name rules. PathGuard
// and native object/name safety remain independent filesystem checks.
func (p FileTypePolicy) AllowsName(name string) bool {
	if !p.Restricted() {
		return true
	}
	if !ValidFileBasename(name) {
		return false
	}
	if !strings.Contains(name[1:], ".") {
		return !p.denyExtensionless
	}
	if !p.restricted {
		return true
	}
	lower := []byte(name)
	for i, ch := range lower {
		if ch >= 'A' && ch <= 'Z' {
			lower[i] = ch + ('a' - 'A')
		}
	}
	for _, extension := range p.extensions.values {
		if len(lower) > len(extension) && strings.HasSuffix(string(lower), extension) {
			return true
		}
	}
	return false
}

// ValidFileBasename excludes spellings that cannot be portably classified.
// It does not decide whether an extension is allowed or whether access is granted.
func ValidFileBasename(name string) bool {
	if name == "" || !utf8.ValidString(name) || strings.ContainsAny(name, `/\:`) ||
		strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	for _, ch := range name {
		if unicode.IsControl(ch) {
			return false
		}
	}
	return true
}

func validExtension(value string) bool {
	if len(value) < 2 || value[0] != '.' {
		return false
	}
	for _, segment := range strings.Split(value[1:], ".") {
		if len(segment) == 0 || !extensionAlnum(segment[0]) {
			return false
		}
		for _, ch := range []byte(segment) {
			if !extensionAlnum(ch) && ch != '_' && ch != '-' {
				return false
			}
		}
	}
	return true
}

func extensionAlnum(ch byte) bool { return ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' }

type extensionValue struct{ list []string }

// UnmarshalTOML uses the existing bootstrap TOML decoder, like sizeValue.
// A nil pointer means absent; a present value retains [] or unrestricted.
func (e *extensionValue) UnmarshalTOML(raw []byte) error {
	var wire struct {
		Value any `toml:"value"`
	}
	if err := toml.Unmarshal(append([]byte("value = "), raw...), &wire); err != nil {
		return errFileTypePolicy
	}
	switch value := wire.Value.(type) {
	case string:
		if value != "unrestricted" {
			return errFileTypePolicy
		}
		e.list = nil
	case []any:
		e.list = make([]string, 0, len(value))
		for _, item := range value {
			text, ok := item.(string)
			if !ok {
				return errFileTypePolicy
			}
			e.list = append(e.list, text)
		}
	default:
		return errFileTypePolicy
	}
	_, err := NewFileTypePolicy(e.list, true)
	return err
}

type fileTypeFields struct {
	AllowedExtensions  *extensionValue `toml:"allowed_extensions"`
	AllowExtensionless *bool           `toml:"allow_extensionless"`
}

func (f fileTypeFields) resolveFileTypes(inherited fileTypeFields) (FileTypePolicy, error) {
	if f.AllowedExtensions == nil {
		f.AllowedExtensions = inherited.AllowedExtensions
	}
	if f.AllowExtensionless == nil {
		f.AllowExtensionless = inherited.AllowExtensionless
	}
	var extensions []string
	if f.AllowedExtensions != nil {
		extensions = f.AllowedExtensions.list
	}
	allow := extensions == nil
	if f.AllowExtensionless != nil {
		allow = *f.AllowExtensionless
	}
	return NewFileTypePolicy(extensions, allow)
}
