package config

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxExclusionPatterns   = 64
	maxExclusionBytes      = 1024
	maxExclusionComponents = 64
	maxExclusionNameBytes  = 255
)

var errInvalidExclusion = errors.New("invalid excluded path pattern")

// PathExclusions is an immutable list of compiled root-relative patterns.
// Its zero value excludes nothing.
type PathExclusions struct {
	patterns [][][]rune
}

func compilePathExclusions(raw []string) (PathExclusions, error) {
	if len(raw) > maxExclusionPatterns {
		return PathExclusions{}, errInvalidExclusion
	}
	var policy PathExclusions
	for _, pattern := range raw {
		if len(pattern) == 0 || len(pattern) > maxExclusionBytes || !utf8.ValidString(pattern) ||
			strings.ContainsAny(pattern, "\\:[]{}<>|\"") || strings.Contains(pattern, "**") ||
			strings.ContainsFunc(pattern, unicode.IsControl) {
			return PathExclusions{}, errInvalidExclusion
		}
		parts := strings.Split(pattern, "/")
		if len(parts) > maxExclusionComponents {
			return PathExclusions{}, errInvalidExclusion
		}
		compiled := make([][]rune, 0, len(parts))
		for _, part := range parts {
			if part == "" || part == "." || part == ".." || len(part) > maxExclusionNameBytes ||
				strings.TrimSpace(part) != part || strings.HasSuffix(part, ".") {
				return PathExclusions{}, errInvalidExclusion
			}
			compiled = append(compiled, []rune(part))
		}
		policy.patterns = append(policy.patterns, compiled)
	}
	return policy, nil
}

// Active reports whether applying this list can deny a path.
func (p PathExclusions) Active() bool { return len(p.patterns) != 0 }

// ExclusionName describes an entry spelling and its parent's confirmed name
// comparison mode. A future directory can conservatively allow ASCII folding
// in the comparison so a potentially excluded creation is rejected.
type ExclusionName struct {
	Text            string
	CaseInsensitive bool
}

// Excludes matches a path or any ancestor. It returns an error if it cannot
// prove the comparison, including when the shared route budget is exhausted.
func (p PathExclusions) Excludes(path []ExclusionName, work *int) (bool, error) {
	for _, pattern := range p.patterns {
		if len(pattern) > len(path) {
			continue
		}
		matches := true
		for i, part := range pattern {
			name := path[i]
			if !utf8.ValidString(name.Text) {
				return false, errInvalidExclusion
			}
			matched, err := matchExclusionComponent(part, []rune(name.Text), name.CaseInsensitive, work)
			if err != nil {
				return false, err
			}
			if !matched {
				matches = false
				break
			}
		}
		if matches {
			return true, nil
		}
	}
	return false, nil
}

// Backtrack only to the most recent star. This matcher never recurses, does
// not expand patterns, and charges each comparison, including a retry.
func matchExclusionComponent(pattern, name []rune, fold bool, work *int) (bool, error) {
	p, n, star, retry := 0, 0, -1, 0
	for n < len(name) || p < len(pattern) {
		if *work <= 0 {
			return false, errInvalidExclusion
		}
		*work--
		if p < len(pattern) && pattern[p] == '*' {
			star, retry = p, n
			p++
			continue
		}
		if n < len(name) && p < len(pattern) {
			literal, actual := pattern[p], name[n]
			if fold && literal != '?' && literal != actual {
				// NTFS case tables are filesystem-owned. ASCII has a
				// proven mapping. Refuse an unproved Unicode comparison.
				if literal > unicode.MaxASCII || actual > unicode.MaxASCII {
					return false, errInvalidExclusion
				}
				literal, actual = upperASCII(literal), upperASCII(actual)
			}
			if literal == '?' || literal == actual {
				p++
				n++
				continue
			}
		}
		if star < 0 || retry == len(name) {
			return false, nil
		}
		retry++
		p, n = star+1, retry
	}
	return true, nil
}

func upperASCII(r rune) rune {
	if 'a' <= r && r <= 'z' {
		return r - ('a' - 'A')
	}
	return r
}
