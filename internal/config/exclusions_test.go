package config

import (
	"slices"
	"strings"
	"testing"
)

func TestExclusionInheritance(t *testing.T) {
	t.Parallel()
	root := documentWithRoot(t, t.TempDir())
	for _, tc := range []struct {
		name, fields string
		match        string
	}{
		{"default", "", ""},
		{"missing policy field", "work.policy = 'empty'\n", ""},
		{"inherit", "work.policy = 'rules'\n", "inherited"},
		{"clear", "work.policy = 'rules'\nwork.excluded = []\n", ""},
		{"replace", "work.policy = 'rules'\nwork.excluded = ['direct']\n", "direct"},
		{"define", "work.excluded = ['direct']\n", "direct"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := Decode([]byte(root+tc.fields+"[policy.rules]\nexcluded = ['inherited']\n[policy.empty]\n"), Direct)
			if err != nil {
				t.Fatal(err)
			}
			roots := cfg.Files().Roots()
			rules := roots[0].Excluded
			roots[0].Excluded = PathExclusions{}
			for _, name := range []string{"inherited", "direct", "sibling"} {
				work := 100
				got, err := cfg.Files().Roots()[0].Excluded.Excludes([]ExclusionName{{Text: name}}, &work)
				if err != nil || got != (name == tc.match) || rules.Active() != (tc.match != "") {
					t.Fatalf("pattern for %s: match %t, error %v", name, got, err)
				}
			}
		})
	}
}

func TestExclusionPatternValidation(t *testing.T) {
	t.Parallel()
	root := documentWithRoot(t, t.TempDir())
	for _, value := range []string{
		`"private"`, `["valid", 1]`, `[[ "nested" ]]`, `[""]`, `["/absolute"]`, `["C:/absolute"]`,
		`["a/../b"]`, `["./a"]`, `["a//b"]`, `["a/"]`, `['a\b']`, `["a/**/b"]`, `["a/**b"]`,
		`["a/[bc]"]`, `["a/{b,c}"]`, `["(a|b)"]`, `["ambiguous."]`, `[" trailing "]`, `["line\nfeed"]`,
		`["` + strings.Repeat("n", maxExclusionNameBytes+1) + `"]`,
		`["` + strings.Repeat("n/", maxExclusionComponents) + `n"]`,
		`["` + strings.Repeat(strings.Repeat("n", 250)+"/", 4) + strings.Repeat("n", 21) + `"]`,
		"[" + strings.Repeat("'valid',", maxExclusionPatterns) + "'extra']",
	} {
		_, err := Decode([]byte(root+"work.excluded = "+value+"\n"), Direct)
		assertCategory(t, err, CategoryInvalidConfig)
		_, err = Decode([]byte(root+"work.policy = 'unused'\nwork.excluded = []\n[policy.unused]\nexcluded = "+value+"\n"), Direct)
		assertCategory(t, err, CategoryInvalidConfig)
		_, err = Decode([]byte(root+"[policy.unused]\nexcluded = "+value+"\n"), Direct)
		assertCategory(t, err, CategoryInvalidConfig)
	}
	if _, err := compilePathExclusions(slices.Repeat([]string{"valid"}, maxExclusionPatterns)); err != nil {
		t.Fatal(err)
	}
}

func TestExclusionComponentMatching(t *testing.T) {
	t.Parallel()
	rules, err := compilePathExclusions([]string{"A/X", "logs/my*.log", "reports/report-202?.pdf", "unicode/?", "dots/*.*"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path string
		fold bool
		want bool
	}{
		{"A", false, false},
		{"A/X", false, true},
		{"A/X/Z/file.txt", false, true},
		{"A/X2", false, false},
		{"a/x", false, false},
		{"a/x", true, true},
		{"logs/my.log", false, true},
		{"logs/my-final.log", false, true},
		{"logs/my/data.log", false, false},
		{"reports/report-2025.pdf", false, true},
		{"reports/report-202.pdf", false, false},
		{"reports/report-20255.pdf", false, false},
		{"unicode/👍", false, true},
		{"dots/.file", false, true},
	} {
		var path []ExclusionName
		for _, name := range strings.Split(tc.path, "/") {
			path = append(path, ExclusionName{Text: name, CaseInsensitive: tc.fold})
		}
		work := 1000
		got, err := rules.Excludes(path, &work)
		if err != nil || got != tc.want {
			t.Errorf("%s (fold=%v): match %v, error %v", tc.path, tc.fold, got, err)
		}
	}
	for _, tc := range []struct {
		path []ExclusionName
		work int
	}{
		{[]ExclusionName{{Text: "logs"}, {Text: "my-final.log"}}, 1},
		{[]ExclusionName{{Text: "logș", CaseInsensitive: true}, {Text: "my.log"}}, 100},
	} {
		if matched, err := rules.Excludes(tc.path, &tc.work); matched || err == nil {
			t.Fatal("unproved comparison did not fail closed")
		}
	}
}
