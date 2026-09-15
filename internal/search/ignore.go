package search

import (
	"errors"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

var ErrInvalidIgnoreFile = errors.New("invalid search ignore file")

const (
	MaxIgnoreFileBytes = 64 * 1024
	MaxIgnorePatterns  = 256
	MaxIgnoreLineBytes = 1024
)

type ignoreRule struct {
	pattern   *regexp.Regexp
	negated   bool
	directory bool
}

type ignoreMatcher struct {
	base  string
	rules []ignoreRule
}

func compileIgnoreFile(ignorePath string, content []byte) (*ignoreMatcher, error) {
	if ignorePath == "" || path.IsAbs(ignorePath) || !utf8.ValidString(ignorePath) || len(content) > MaxIgnoreFileBytes {
		return nil, ErrInvalidIgnoreFile
	}
	clean := path.Clean(strings.ReplaceAll(ignorePath, `\`, "/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return nil, ErrInvalidIgnoreFile
	}

	m := &ignoreMatcher{base: path.Dir(clean)}
	for _, raw := range strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n") {
		if len(raw) > MaxIgnoreLineBytes {
			return nil, ErrInvalidIgnoreFile
		}
		line := strings.TrimRight(raw, " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		negated := strings.HasPrefix(line, "!")
		if negated {
			line = strings.TrimPrefix(line, "!")
		} else if strings.HasPrefix(line, `\!`) || strings.HasPrefix(line, `\#`) {
			line = line[1:]
		}
		directory := strings.HasSuffix(line, "/")
		line = strings.TrimSuffix(line, "/")
		if line == "" || !utf8.ValidString(line) || len(m.rules) == MaxIgnorePatterns {
			return nil, ErrInvalidIgnoreFile
		}
		rx, err := compileGitignorePattern(line)
		if err != nil {
			return nil, ErrInvalidIgnoreFile
		}
		m.rules = append(m.rules, ignoreRule{pattern: rx, negated: negated, directory: directory})
	}
	return m, nil
}

func compileGitignorePattern(pattern string) (*regexp.Regexp, error) {
	anchored := strings.HasPrefix(pattern, "/") || strings.Contains(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	var b strings.Builder
	if anchored {
		b.WriteString("^")
	} else {
		b.WriteString("(^|.*/)")
	}
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '[':
			end := strings.IndexByte(pattern[i+1:], ']')
			if end < 0 {
				return nil, ErrInvalidIgnoreFile
			}
			end += i + 1
			class := pattern[i+1 : end]
			if strings.HasPrefix(class, "!") {
				class = "^" + regexp.QuoteMeta(class[1:])
			} else {
				class = regexp.QuoteMeta(class)
			}
			b.WriteByte('[')
			b.WriteString(class)
			b.WriteByte(']')
			i = end
		case '\\':
			if i+1 >= len(pattern) {
				return nil, ErrInvalidIgnoreFile
			}
			i++
			b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		default:
			b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func (m *ignoreMatcher) ignored(relativePath string, isDir bool) bool {
	if m == nil {
		return false
	}
	candidate := relativePath
	if m.base != "." {
		prefix := m.base + "/"
		if !strings.HasPrefix(candidate, prefix) {
			return false
		}
		candidate = strings.TrimPrefix(candidate, prefix)
	}
	ignored := false
	for _, rule := range m.rules {
		if rule.directory && !isDir {
			continue
		}
		if rule.pattern.MatchString(candidate) {
			ignored = !rule.negated
		}
	}
	return ignored
}
