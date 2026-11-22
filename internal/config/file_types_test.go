package config

import (
	"slices"
	"strings"
	"testing"
)

func TestFileTypeNameMatching(t *testing.T) {
	for _, tc := range []struct {
		name            string
		extensions      []string
		extensionless   bool
		allowed, denied []string
	}{
		{"absent", nil, true, []string{"a.exe", "README", ".env", "日本語.txt", "a.", "a.txt ", "a:stream.txt", `a\b.txt`}, nil},
		{"empty", []string{}, false, nil, []string{"a.txt", "README", ".env"}},
		{"extensionless only", []string{}, true, []string{"README", ".env"}, []string{".env.txt", "a.txt", "a."}},
		{"text", []string{".txt"}, false, []string{"a.txt", "A.TXT", ".env.txt", "日本語.txt"}, []string{"a.txt.exe", "a.txт", "a.ＴＸＴ", "README", ".txt", "a.txt.", "a.txt ", "", "a:stream.txt", "a/b.txt", "a\\b.txt", "x\n.txt", "x\x7f.txt", "\xff.txt"}},
		{"compound", []string{".tar.gz"}, true, []string{"a.tar.gz", "A.TAR.GZ", "README"}, []string{"a.gz", ".tar.gz", "a.tar.gz.exe"}},
		{"last suffix", []string{".gz"}, false, []string{"a.tar.gz", "a.gz"}, []string{"a.tar", "a.gz.exe"}},
		{"unrestricted suffixed", nil, false, []string{"a.txt", "a.exe"}, []string{"README", ".env"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := NewFileTypePolicy(tc.extensions, tc.extensionless)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range tc.allowed {
				if !policy.AllowsName(name) {
					t.Errorf("denied %q", name)
				}
			}
			for _, name := range tc.denied {
				if policy.AllowsName(name) {
					t.Errorf("admitted %q", name)
				}
			}
		})
	}
}

func TestFileTypePolicySnapshots(t *testing.T) {
	input := []string{".txt", ".md"}
	p, err := NewFileTypePolicy(input, false)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = ".exe"
	output := p.AllowedExtensions()
	output[0] = ".exe"
	if !p.AllowsName("a.txt") || p.AllowsName("a.exe") || !p.Restricted() || p.AllowExtensionless() {
		t.Fatal("mutable policy")
	}
	if !slices.Equal(p.AllowedExtensions(), []string{".md", ".txt"}) {
		t.Fatal("unstable snapshot")
	}
	empty, _ := NewFileTypePolicy([]string{}, false)
	if empty.AllowedExtensions() == nil {
		t.Fatal("empty collapsed to absent")
	}
	var zero FileTypePolicy
	if zero.Restricted() || !zero.AllowExtensionless() || !zero.AllowsName("README") || zero.AllowedExtensions() != nil {
		t.Fatal("zero/default drift")
	}
}

func TestFileTypeUnrestrictedNativeNames(t *testing.T) {
	root := documentWithRoot(t, t.TempDir())
	for _, fields := range []string{
		"",
		"work.allowed_extensions='unrestricted'\n",
		"work.allowed_extensions='unrestricted'\nwork.allow_extensionless=true\n",
		"work.policy='p'\nwork.allowed_extensions='unrestricted'\nwork.allow_extensionless=true\n[policy.p]\nallowed_extensions=['.txt']\nallow_extensionless=false\n",
	} {
		cfg, err := Decode([]byte(root+fields), Direct)
		if err != nil {
			t.Fatal(err)
		}
		p := cfg.Files().Roots()[0].FileTypes
		if p.Restricted() {
			t.Fatal("unrestricted effective policy", fields)
		}
		for _, name := range []string{"normal.txt", "colon:token.txt", "tail.txt.", "tail.txt ", `back\slash.txt`} {
			if !p.AllowsName(name) {
				t.Errorf("unrestricted policy imposed portable name rules on %q", name)
			}
		}
	}
}

func TestFileTypeInheritance(t *testing.T) {
	root := documentWithRoot(t, t.TempDir())
	for _, tc := range []struct {
		name, root, policy string
		txt, exe, bare     bool
	}{
		{"absent", "", "", true, true, true},
		{"empty", "allowed_extensions=[]", "", false, false, false},
		{"unrestricted", "allowed_extensions='unrestricted'", "", true, true, true},
		{"restricted", "allowed_extensions=['.txt']", "", true, false, false},
		{"empty plus true", "allowed_extensions=[]\nwork.allow_extensionless=true", "", false, false, true},
		{"inherit", "policy='p'", "allowed_extensions=['.txt']", true, false, false},
		{"inherit false with reset", "policy='p'\nwork.allowed_extensions='unrestricted'", "allowed_extensions=['.txt']\nallow_extensionless=false", true, true, false},
		{"reset both", "policy='p'\nwork.allowed_extensions='unrestricted'\nwork.allow_extensionless=true", "allowed_extensions=['.txt']\nallow_extensionless=false", true, true, true},
		{"empty replacement", "policy='p'\nwork.allowed_extensions=[]", "allowed_extensions=['.txt']\nallow_extensionless=true", false, false, true},
		{"list replacement default", "policy='p'\nwork.allowed_extensions=['.txt']", "allowed_extensions='unrestricted'", true, false, false},
		{"independent boolean override", "policy='p'\nwork.allow_extensionless=false", "allowed_extensions=['.txt']\nallow_extensionless=true", true, false, false},
		{"empty policy", "policy='p'", "", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := root
			if tc.root != "" {
				settings += "work." + tc.root + "\n"
			}
			if strings.Contains(tc.root, "policy=") {
				settings += "[policy.p]\n" + tc.policy + "\n"
			}
			cfg, err := Decode([]byte(settings), Direct)
			if err != nil {
				t.Fatal(err)
			}
			p := cfg.Files().Roots()[0].FileTypes
			for name, want := range map[string]bool{"a.txt": tc.txt, "a.exe": tc.exe, "README": tc.bare} {
				if p.AllowsName(name) != want {
					t.Errorf("%s: %t want %t", name, p.AllowsName(name), want)
				}
			}
			snapshot := cfg.Files().Roots()
			snapshot[0].FileTypes = FileTypePolicy{}
			if cfg.Files().Roots()[0].FileTypes.AllowsName("a.exe") != tc.exe {
				t.Fatal("root snapshot mutated policy")
			}
		})
	}
}

func TestFileTypeInvalidDocuments(t *testing.T) {
	root := documentWithRoot(t, t.TempDir())
	for _, value := range []string{"42", "true", "{}", "'any'", "'UNRESTRICTED'", "['*']", "['.TXT']", "['txt']", "['.']", "['.tar..gz']", "['._txt']", "['.txt ' ]", "['.t/xt']", "['.t:xt']", "['.t\\xt']", "['.txт']", "['.txt', '.txt']", "['.txt', 2]"} {
		for _, prefix := range []string{"work.allowed_extensions=", "[policy.unused]\nallowed_extensions="} {
			_, err := Decode([]byte(root+prefix+value+"\n"), Direct)
			assertCategory(t, err, CategoryInvalidConfig)
		}
	}
	for _, extra := range []string{"allowed_extensions=['.txt']", "work.allow_extensionless='true'", "work.allowed_extensions=[]\nwork.allowed_extensions=[]", "[policy.unused]\nallow_extensionless=1", "work.policy='missing'\nwork.allowed_extensions='unrestricted'", "work.ALLOWED_EXTENSIONS=[]"} {
		_, err := Decode([]byte(root+extra+"\n"), Direct)
		assertCategory(t, err, CategoryInvalidConfig)
	}
}
