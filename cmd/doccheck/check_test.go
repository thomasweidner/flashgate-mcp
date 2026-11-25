package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func testSet(docs map[string]string) (map[string]string, map[string]bool) {
	files := map[string]bool{}
	for name := range docs {
		files[name] = true
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			files[dir] = true
		}
	}
	return docs, files
}

func ruleCount(r report, rule string) int {
	n := 0
	for _, f := range r.Findings {
		if f.Rule == rule {
			n++
		}
	}
	return n
}

func TestNames(t *testing.T) {
	cases := map[string]bool{
		"README.md": true, "docs/README.md": true, "AGENTS.md": true, "BACKLOG.md": true,
		"CHANGELOG.md": true, "CONTRIBUTING.md": true, "THIRD-PARTY-NOTICES.md": true,
		"docs/architecture.md": true, "docs/adr/filesystem-abstraction.md": true,
		"docs/MyDocument.md": false, "docs/foo_bar.md": false, "docs/005-topic.md": false,
		"docs/pr-15-review.md": false, "docs/spr-047-benchmark.md": false,
		"docs/mobile-preparation.md": false, "docs/topic-2026-09-25.md": false,
		"docs/backlog-id-migration.md": false, "docs/sprint-id-migration.md": false,
		"docs/CHANGELOG.md": false, "docs/readme.MD": false,
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			if got := validName(name); got != want {
				t.Fatalf("validName(%q)=%v want %v", name, got, want)
			}
		})
	}
}

func TestEnglishAndUnicodeBoundaries(t *testing.T) {
	cases := []struct {
		name, text string
		want       int
	}{
		{"english", "# Guide\nThis is an English guide.\n", 0},
		{"german-umlaut", "# Guide\nDieses Dokument ist für Benutzer.\n", 1},
		{"german-ascii", "# Guide\nDieser Text muss angepasst werden.\n", 1},
		{"unicode-not-language-ban", "# Guide\nUnicode examples: café, 日本語, résumé.\n", 0},
		{"inline-literal", "# Guide\nA literal name is `für`.\n", 0},
		{"fenced-data", "# Guide\n```json\n{\"für\":\"Benutzer\"}\n```\n", 0},
		{"fenced-comment", "# Guide\n```powershell\n# Dieses Beispiel prüfen.\n$x = 1\n```\n", 1},
		{"comment-c-style", "# Guide\n```go\n// Dieses Beispiel prüfen.\n```\n", 1},
		{"double-backtick", "# Guide\nUse ``für `Benutzer` `` literally.\n", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, f := testSet(map[string]string{"docs/guide.md": c.text})
			r := checkDocuments(d, f)
			if got := ruleCount(r, "LANGUAGE"); got != c.want {
				t.Fatalf("got %d, want %d: %+v", got, c.want, r)
			}
		})
	}
}

func TestAnchors(t *testing.T) {
	text := "# Title\n## Tool `read_file`\n## Repeat\n## Repeat\n## Café\n<a id=\"explicit\"></a>\nSetext\n------\n```md\n## Not an anchor\n```\n"
	got := headingAnchors(text)
	for _, name := range []string{"title", "tool-read_file", "repeat", "repeat-1", "café", "explicit", "setext"} {
		if !got[name] {
			t.Errorf("missing anchor %q in %+v", name, got)
		}
	}
	if got["not-an-anchor"] {
		t.Fatal("code-fence heading became an anchor")
	}
}

func TestDestinations(t *testing.T) {
	cases := map[string][]string{
		"[simple](file.md)":                               {"file.md"},
		"[title](file.md \"caption\")":                    {"file.md"},
		"[encoded](file%20name.md#hello)":                 {"file%20name.md#hello"},
		"[bracket](<file name.md>)":                       {"file name.md"},
		"[nested](file(part).md)":                         {"file(part).md"},
		"![image](assets/logo.svg) and [guide](guide.md)": {"assets/logo.svg", "guide.md"},
		"ordinary text":                                   {},
	}
	for line, want := range cases {
		if got := inlineDestinations(line); !reflect.DeepEqual(got, want) {
			t.Errorf("%q got %#v want %#v", line, got, want)
		}
	}
}

func TestLocalLinks(t *testing.T) {
	docs, files := testSet(map[string]string{
		"README.md": "# Start\n", "docs/guide.md": "# Guide\n## Details\n", "docs/note.md": "# Note\n",
	})
	anchors := map[string]map[string]bool{}
	for name, text := range docs {
		anchors[name] = headingAnchors(text)
	}
	cases := []struct {
		source, target string
		local, fail    bool
	}{
		{"README.md", "docs/guide.md#details", true, false},
		{"docs/guide.md", "../README.md", true, false},
		{"docs/guide.md", "#details", true, false},
		{"docs/guide.md", "#missing", true, true},
		{"README.md", "docs/Guide.md", true, true},
		{"README.md", "missing.md", true, true},
		{"README.md", "../outside.md", true, true},
		{"README.md", "%2e%2e/outside.md", true, true},
		{"README.md", "/docs/guide.md", true, true},
		{"README.md", "docs\\guide.md", true, true},
		{"README.md", "https://example.invalid/path#heading", false, false},
		{"README.md", "mailto:example@example.invalid", false, false},
		{"README.md", "%ZZ", true, true},
		{"README.md", "C:/data/guide.md", true, true},
		{"README.md", "file:///C:/data/guide.md", true, true},
	}
	for _, c := range cases {
		t.Run(c.target, func(t *testing.T) {
			local, detail := validateLink(c.source, c.target, files, anchors)
			if local != c.local || (detail != "") != c.fail {
				t.Fatalf("got local=%v detail=%q", local, detail)
			}
		})
	}
}

func TestReferenceLinksAndLineNumbers(t *testing.T) {
	docs, files := testSet(map[string]string{
		"README.md":     "# Start\n[Guide][ref]\n[Guide][]\n[missing][undefined]\n[ref]: docs/guide.md#details\n[guide]: docs/guide.md\n",
		"docs/guide.md": "# Guide\n## Details\n",
	})
	r := checkDocuments(docs, files)
	if ruleCount(r, "LINK") != 1 || r.Findings[0].Line != 4 {
		t.Fatalf("unexpected reference findings: %+v", r)
	}
	if r.LocalLinkCount != 4 {
		t.Fatalf("local links=%d want 4", r.LocalLinkCount)
	}
}

func TestFencedLinksAreNotRendered(t *testing.T) {
	d, f := testSet(map[string]string{"README.md": "# Start\n````md\n[bad](missing.md)\n```\n## fake\n````\n"})
	r := checkDocuments(d, f)
	if len(r.Findings) != 0 {
		t.Fatalf("fenced example generated findings: %+v", r)
	}
}

func TestMultilineLinks(t *testing.T) {
	const target = "docs/guide.md"
	cases := []struct {
		name, body string
		count      int
		findings   []finding
	}{
		{name: "destination and title", body: "[Guide](\n    <" + target + "#details>\n    \"Guide title\"\n)\n", count: 1},
		{name: "missing target", body: "[Guide](\n    docs/nonexistent.md\n)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Missing target or wrong path case: docs/nonexistent.md"}}},
		{name: "missing anchor", body: "[Guide](\n" + target + "#missing)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Missing heading/anchor: docs/guide.md#missing"}}},
		{name: "outside repository", body: "[Guide](\n../outside.md\n)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Link escapes the repository: ../outside.md"}}},
		{name: "wrong path case", body: "[Guide](docs/Guide.md\n)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Missing target or wrong path case: docs/Guide.md"}}},
		{name: "reference definition and link text", body: "[Read the\n guide][ref]\n\n[ref]:\n    " + target + "#details\n", count: 2},
		{name: "reference label continuation", body: "[Read][guide\n reference]\n\n[guide\n reference]:\n    " + target + "\n", count: 2},
		{name: "bad definition and use", body: "[Read][ref]\n\n[ref]:\n    absent.md\n", count: 2,
			findings: []finding{{Line: 3, Detail: "Missing target or wrong path case: absent.md"},
				{Line: 5, Detail: "Missing target or wrong path case: absent.md"}}},
		{name: "bad multiline link text", body: "[Read the\n guide](absent.md)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Missing target or wrong path case: absent.md"}}},
		{name: "fenced code", body: "```md\n[Guide](\nabsent.md\n)\n```\n"},
		{name: "inline code", body: "`[Guide](absent.md)` and ``[Guide](\nabsent.md\n)``\n"},
		{name: "blank line", body: "[Guide](\n\nabsent.md)\n\n[Read\n\n guide](absent.md)\n"},
		{name: "fence boundary", body: "[Guide](\n```md\nignored\n```\nabsent.md)\n"},
		{name: "heading boundary", body: "[Guide](\n## Next section\nabsent.md)\n"},
		{name: "list item", body: "- [Guide](\n    " + target + ")\n", count: 1},
		{name: "list item boundary", body: "- [Guide](\n- absent.md)\n"},
		{name: "ordered marker in prose", body: "[Read the\n2. chapter](absent.md)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Missing target or wrong path case: absent.md"}}},
		{name: "ordered marker inside list paragraph", body: "- [Read the\n  2. chapter](absent.md)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Missing target or wrong path case: absent.md"}}},
		{name: "ordered list starts with one", body: "[Read the\n01. chapter](absent.md)\n"},
		{name: "nested ordered list interrupts item paragraph", body: "- [Read the\n  1. chapter](absent.md)\n"},
		{name: "ordered list sibling", body: "1. [Read the\n2. chapter](absent.md)\n"},
		{name: "new ordered list after delimiter change", body: "1. [Read the\n3) chapter](absent.md)\n"},
		{name: "block quote", body: "> [Guide](\n> " + target + ")\n", count: 1},
		{name: "lazy block quote continuation", body: "> [Guide](\nabsent.md)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Missing target or wrong path case: absent.md"}}},
		{name: "lazy list continuation", body: "- [Guide](\nabsent.md)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Missing target or wrong path case: absent.md"}}},
		{name: "nested lazy continuation", body: "> > - [Guide](\n> " + target + ")\n", count: 1},
		{name: "lazy text in list and quote", body: "> - [Read the\n guide](" + target + ")\n", count: 1},
		{name: "quote paragraph boundary", body: "> [Guide](\n>\nabsent.md)\n"},
		{name: "list paragraph boundary", body: "- [Guide](\n\nabsent.md)\n"},
		{name: "quote heading boundary", body: "> [Guide](\n## Heading\nabsent.md)\n"},
		{name: "list fence boundary", body: "- [Guide](\n  ~~~\n  absent.md)\n  ~~~\n"},
		{name: "new quote boundary", body: "[Guide](\n> absent.md)\n"},
		{name: "single-line links in containers", body: "> [Guide](" + target + ")\n\n- [Guide](" + target + ")\n", count: 2},
		{name: "fenced quote", body: "> ~~~md\n> [Guide](\n> absent.md)\n> ~~~\n"},
		{name: "nested quote marks in fenced quote", body: "> ~~~md\n> > [Guide](\n> > absent.md)\n> ~~~\n"},
		{name: "nested bracket text", body: "[Guide [part\n one]](" + target + ")\n", count: 1},
		{name: "nested inline link", body: "[Read [the guide](" + target + ")](\nabsent.md)\n", count: 1},
		{name: "nested image", body: "[![Guide](" + target + ")](\n" + target + ")\n", count: 2},
		{name: "deep bracket literals", body: strings.Repeat("[", 10_000) + "\n" + strings.Repeat("]", 10_000)},
		{name: "single and multiline", body: "[Guide](" + target + ") and [More](\n" + target + ")\n", count: 2},
		{name: "escaped opening parenthesis", body: "[Guide](absent\\(file.md)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Use a repository-relative link: absent\\(file.md"}}},
		{name: "escaped closing parenthesis", body: "[Guide](\nabsent\\)file.md\n)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Use a repository-relative link: absent\\)file.md"}}},
		{name: "escaped closing parenthesis does not end the link", body: "[Guide](absent.md\\)\n"},
		{name: "even backslashes keep a balanced pair structural", body: "[Guide](absent\\\\(file.md))\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Use a repository-relative link: absent\\\\(file.md)"}}},
		{name: "even backslashes keep an unclosed pair structural", body: "[Guide](absent\\\\(file.md)\n"},
		{name: "unescaped balanced destination", body: "[Guide](absent(file).md)\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Missing target or wrong path case: absent(file).md"}}},
		{name: "escaped parenthesis in reference destination", body: "[ref]: absent\\(file.md\n", count: 1,
			findings: []finding{{Line: 3, Detail: "Use a repository-relative link: absent\\(file.md"}}},
		{name: "external URL", body: "[Website](\nhttps://example.invalid/guide#heading\n)\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, newline := range []string{"\n", "\r\n"} {
				docs, files := testSet(map[string]string{
					"README.md": strings.ReplaceAll("# Start\n\n"+tc.body, "\n", newline),
					target:      strings.ReplaceAll("# Guide\n## Details\n", "\n", newline),
				})
				got := checkDocuments(docs, files)
				want := append([]finding{}, tc.findings...)
				for i := range want {
					want[i].Path = "README.md"
					want[i].Rule = "LINK"
				}
				if got.LocalLinkCount != tc.count || !reflect.DeepEqual(got.Findings, want) {
					t.Errorf("newline %q: got %d links and %+v, want %d links and %+v", newline, got.LocalLinkCount, got.Findings, tc.count, want)
				}
			}
		})
	}
}

func TestMalformedLinkDestinationsKeepLaterLink(t *testing.T) {
	const valid = "[Guide](docs/guide.md)"
	const unclosed = "[x]("
	const maxSize = 4 << 20
	line := strings.Repeat(unclosed, (maxSize-len(valid))/len(unclosed)) + valid
	got := documentLinks([]string{line})
	want := map[int][]linkCheck{0: {{value: "docs/guide.md"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("malformed destinations interfered with the final link: %v", got)
	}
}

func BenchmarkUnclosedLinkDestinations(b *testing.B) {
	for _, size := range []struct {
		name string
		kib  int
	}{{"4KiB", 4}, {"8KiB", 8}, {"16KiB", 16}, {"32KiB", 32}} {
		b.Run(size.name, func(b *testing.B) {
			line := strings.Repeat("[x](", size.kib*1024/4)
			b.SetBytes(int64(len(line)))
			for b.Loop() {
				if links := documentLinks([]string{line}); len(links) != 0 {
					b.Fatalf("found links in an unterminated sequence: %v", links)
				}
			}
		})
	}
}

func TestEmptyInventoryAndDeterminism(t *testing.T) {
	empty := checkDocuments(map[string]string{}, map[string]bool{})
	if ruleCount(empty, "INVENTORY") != 1 {
		t.Fatal("empty inventory passed")
	}
	docs, files := testSet(map[string]string{"z.md": "# Z\n", "a.md": "# A\n"})
	want := checkDocuments(docs, files)
	for i := 0; i < 10; i++ {
		if got := checkDocuments(docs, files); !reflect.DeepEqual(got, want) {
			t.Fatal("nondeterministic report")
		}
	}
	raw, err := json.Marshal(want)
	if err != nil || !strings.Contains(string(raw), `"findings":[]`) {
		t.Fatalf("invalid JSON: %s %v", raw, err)
	}
}

func initFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git unavailable for inventory fixture")
	}
	root := t.TempDir()
	cmd := exec.Command("git", "-C", root, "init", "--quiet")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fixture init: %s %v", out, err)
	}
	return root
}

func writeFixture(t *testing.T, root, name string, data []byte) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryInventory(t *testing.T) {
	root := initFixture(t)
	writeFixture(t, root, "README.md", []byte("# Root\n[Guide](docs/guide.md)\n"))
	writeFixture(t, root, "docs/guide.md", []byte("\xef\xbb\xbf# Guide\n"))
	writeFixture(t, root, "vendor/upstream/README.md", []byte("Dieses Dokument bleibt unverändert.\n"))
	writeFixture(t, root, ".gitignore", []byte("build/\n"))
	writeFixture(t, root, "build/report.md", []byte("Dieses Dokument ist generiert.\n"))
	writeFixture(t, root, ".github/README.md", []byte("# Contributions\n"))
	r := checkRepository(root)
	if len(r.Findings) != 0 || r.DocumentCount != 3 || r.ThirdPartyDocumentCount != 1 {
		t.Fatalf("inventory: %+v", r)
	}
	// Newly created nonignored documents must not escape the gate.
	writeFixture(t, root, "docs/NewDocument.md", []byte("# Neuer Text\nDieses Dokument muss geändert werden.\n"))
	r = checkRepository(root)
	if ruleCount(r, "NAME") != 1 || ruleCount(r, "LANGUAGE") != 1 {
		t.Fatalf("new document escaped: %+v", r)
	}
}

func TestUnreadableTextRejected(t *testing.T) {
	for name, data := range map[string][]byte{"invalid-utf8": {0xff}, "nul": {97, 0}} {
		t.Run(name, func(t *testing.T) {
			root := initFixture(t)
			writeFixture(t, root, "README.md", data)
			r := checkRepository(root)
			if ruleCount(r, "UTF8") != 1 {
				t.Fatalf("bad text passed: %+v", r)
			}
		})
	}
}

func TestSymlinkDocumentRejected(t *testing.T) {
	root := initFixture(t)
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("# Outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "README.md")); err != nil {
		t.Skip("symlink privilege unavailable")
	}
	if r := checkRepository(root); ruleCount(r, "READ") != 1 {
		t.Fatalf("symlink passed: %+v", r)
	}
}

func TestInventoryFailureIsNotEmptySuccess(t *testing.T) {
	root := t.TempDir()
	r := checkRepository(root)
	if len(r.Findings) == 0 || r.Findings[0].Rule != "INVENTORY" {
		t.Fatalf("nonrepository passed: %+v", r)
	}
}

func TestBoundedIO(t *testing.T) {
	b := &limitBuffer{limit: 3}
	if n, err := b.Write([]byte("1234")); n != 3 || err == nil || b.String() != "123" {
		t.Fatalf("overflow was not bounded: %d %v %q", n, err, b.String())
	}
	root := t.TempDir()
	file := filepath.Join(root, "large.md")
	if err := os.WriteFile(file, make([]byte, 4*1024*1024+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDocument(file); err == nil {
		t.Fatal("oversized document accepted")
	}
	if _, err := readDocument(filepath.Join(root, "absent")); err == nil {
		t.Fatal("absent file accepted")
	}
}

func TestInlineCodeIsNotALink(t *testing.T) {
	docs := map[string]string{"README.md": "# Overview\nUse `^[A-Za-z0-9][A-Za-z0-9._-]{0,79}$` and `[not a link](absent.md)`.\n"}
	r := checkDocuments(docs, map[string]bool{"README.md": true})
	if len(r.Findings) != 0 || r.LocalLinkCount != 0 {
		t.Fatalf("code parsed as link: %+v", r)
	}
}
