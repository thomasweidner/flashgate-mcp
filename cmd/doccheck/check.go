package main

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type finding struct {
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	Rule   string `json:"rule"`
	Detail string `json:"detail"`
}

type report struct {
	SchemaVersion           int       `json:"schemaVersion"`
	Status                  string    `json:"status"`
	DocumentCount           int       `json:"documentCount"`
	ThirdPartyDocumentCount int       `json:"thirdPartyDocumentCount"`
	LocalLinkCount          int       `json:"localLinkCount"`
	LanguageScan            string    `json:"languageScan"`
	DocumentPaths           []string  `json:"documentPaths"`
	Findings                []finding `json:"findings"`
}

var topicName = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*\.md$`)
var datedName = regexp.MustCompile(`(?:^|-)\d{4}-\d{2}-\d{2}(?:-|\.)`)
var taskName = regexp.MustCompile(`^(?:\d{3}-|pr-\d|spr-\d|sprint-\d|mobile-|classic-independent-review-|backlog-id-migration|sprint-id-migration)`)
var headingLine = regexp.MustCompile(`^ {0,3}#{1,6}\s+(.+?)(?:\s+#+)?\s*$`)
var referenceDefinition = regexp.MustCompile(`^ {0,3}\[([^\]]+)\]:\s*(.*)$`)
var explicitAnchor = regexp.MustCompile(`(?i)<a\s+(?:id|name)=["']([^"']+)["'][^>]*>`)
var htmlTag = regexp.MustCompile(`<[^>]*>`)
var words = regexp.MustCompile(`[\p{L}]+`)
var germanWords = map[string]bool{
	"für": true, "gemäß": true, "müssen": true, "muss": true, "dürfen": true,
	"darf": true, "wird": true, "werden": true, "enthält": true, "enthielten": true,
	"dieses": true, "diese": true, "dieser": true, "zweck": true, "entscheidung": true,
	"entscheidungen": true, "prüfen": true, "prüfung": true, "verzeichnis": true,
	"verbindlich": true, "maßgeblich": true, "ausschließlich": true, "gegebenenfalls": true,
	"benutzer": true, "beispielsweise": true, "festgelegt": true, "folgende": true,
	"folgender": true, "zuordnung": true, "hersteller": true,
	"kennzahl": true, "mindestwert": true, "fehlgeschlagen": true,
}

func newReport() report {
	return report{SchemaVersion: 1, Status: "PASS", LanguageScan: "HEURISTIC_REQUIRES_EDITORIAL_REVIEW", DocumentPaths: []string{}, Findings: []finding{}}
}

// limitBuffer never retains output beyond the configured bound.
type limitBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.Len()
	if len(p) > remaining {
		n, _ := b.Buffer.Write(p[:remaining])
		return n, io.ErrShortBuffer
	}
	return b.Buffer.Write(p)
}

func readDocument(file string) ([]byte, error) {
	handle, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	data, err := io.ReadAll(io.LimitReader(handle, 4*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 4*1024*1024 {
		return nil, fmt.Errorf("document exceeds size limit")
	}
	return data, nil
}

func checkRepository(root string) (r report) {
	r = newReport()
	defer func() {
		if len(r.Findings) > 0 {
			r.Status = "FAIL"
		}
	}()
	resolved, err := filepath.Abs(root)
	if err != nil {
		r.Findings = append(r.Findings, finding{Rule: "ROOT", Detail: "Cannot resolve repository root."})
		return r
	}
	// Inventory includes tracked and nonignored new files. Git data and ignored
	// generated output are not source documentation; vendor documents are counted separately.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "-c", "core.fsmonitor=false", "-C", resolved, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	var output limitBuffer
	output.limit = 16 * 1024 * 1024
	command.Stdout = &output
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	err = command.Run()
	raw := output.Bytes()
	if err != nil {
		r.Findings = append(r.Findings, finding{Rule: "INVENTORY", Detail: "Git source inventory failed; no partial inventory is accepted."})
		return r
	}
	if len(raw) > 16*1024*1024 {
		r.Findings = append(r.Findings, finding{Rule: "INVENTORY", Detail: "Source inventory exceeds 16 MiB."})
		return r
	}
	inventory := map[string]bool{}
	documents := map[string]string{}
	total := 0
	for _, item := range bytes.Split(raw, []byte{0}) {
		name := string(item)
		if name == "" || inventory[name] {
			continue
		}
		if !utf8.ValidString(name) || path.IsAbs(name) || path.Clean(name) != name || name == ".." || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\\r\n\x00") {
			r.Findings = append(r.Findings, finding{Rule: "PATH", Detail: "Inventory contains a nonportable or unsafe path."})
			continue
		}
		diskPath := filepath.Join(resolved, filepath.FromSlash(name))
		info, statErr := os.Lstat(diskPath)
		// Unstaged deletions are allowed in a candidate; required canonical
		// documents are checked by the enclosing consistency gate.
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil {
			r.Findings = append(r.Findings, finding{Path: name, Rule: "READ", Detail: "Cannot inspect source path."})
			continue
		}
		inventory[name] = true
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			inventory[dir] = true
		}
		if !strings.EqualFold(path.Ext(name), ".md") {
			continue
		}
		if strings.HasPrefix(name, "vendor/") {
			r.ThirdPartyDocumentCount++
			continue
		}
		if !info.Mode().IsRegular() {
			r.Findings = append(r.Findings, finding{Path: name, Rule: "READ", Detail: "Maintained Markdown must be a regular file, not a symbolic link."})
			continue
		}
		effective, resolveErr := filepath.EvalSymlinks(diskPath)
		rootEffective, rootErr := filepath.EvalSymlinks(resolved)
		rel, relErr := filepath.Rel(rootEffective, effective)
		if resolveErr != nil || rootErr != nil || relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			r.Findings = append(r.Findings, finding{Path: name, Rule: "READ", Detail: "Document resolves outside the repository or cannot be resolved."})
			continue
		}
		if info.Size() > 4*1024*1024 || total+int(info.Size()) > 32*1024*1024 {
			r.Findings = append(r.Findings, finding{Path: name, Rule: "SIZE", Detail: "Document exceeds the 4 MiB file or 32 MiB total documentation limit."})
			continue
		}
		data, readErr := readDocument(diskPath)
		if readErr != nil || total+len(data) > 32*1024*1024 || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			r.Findings = append(r.Findings, finding{Path: name, Rule: "UTF8", Detail: "Document must be readable strict UTF-8 without NUL bytes."})
			continue
		}
		total += len(data)
		documents[name] = string(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
	}
	checked := checkDocuments(documents, inventory)
	r.DocumentCount = checked.DocumentCount
	r.DocumentPaths = checked.DocumentPaths
	r.LocalLinkCount = checked.LocalLinkCount
	r.Findings = append(r.Findings, checked.Findings...)
	return r
}

// checkDocuments is independent of Git and disk access so every rule can be
// exercised with tiny deterministic fixtures, including Windows case behavior.
func checkDocuments(documents map[string]string, inventory map[string]bool) (r report) {
	r = newReport()
	defer func() {
		if len(r.Findings) > 0 {
			r.Status = "FAIL"
		}
	}()
	for name := range documents {
		r.DocumentPaths = append(r.DocumentPaths, name)
	}
	sort.Strings(r.DocumentPaths)
	r.DocumentCount = len(r.DocumentPaths)
	if r.DocumentCount == 0 {
		r.Findings = append(r.Findings, finding{Rule: "INVENTORY", Detail: "No maintained Markdown documents found."})
	}
	anchors := map[string]map[string]bool{}
	for name, text := range documents {
		anchors[name] = headingAnchors(text)
	}
	for _, name := range r.DocumentPaths {
		text := documents[name]
		if !validName(name) {
			r.Findings = append(r.Findings, finding{Path: name, Rule: "NAME", Detail: "Use a stable lower-kebab-case topic name without a task/date prefix, or an explicit conventional-name exception."})
		}
		lines := documentLines(text, false)
		languageLines := documentLines(text, true)
		references := map[string]string{}
		for _, l := range lines {
			if m := referenceDefinition.FindStringSubmatch(l); m != nil {
				target, _ := readDestination(m[2])
				references[strings.ToLower(strings.Join(strings.Fields(m[1]), " "))] = target
			}
		}
		for i, l := range lines {
			visible := stripInlineCode(languageLines[i])
			for _, word := range words.FindAllString(visible, -1) {
				if germanWords[strings.ToLower(word)] {
					r.Findings = append(r.Findings, finding{Path: name, Line: i + 1, Rule: "LANGUAGE", Detail: "Review non-English prose candidate: " + word})
					break
				}
			}
			linkLine := stripInlineCode(l)
			targets := inlineDestinations(linkLine)
			if m := referenceDefinition.FindStringSubmatch(l); m != nil {
				target, ok := readDestination(m[2])
				if ok {
					targets = append(targets, target)
				}
			} else {
				refTargets, missing := referenceTargets(linkLine, references)
				targets = append(targets, refTargets...)
				for _, label := range missing {
					r.Findings = append(r.Findings, finding{Path: name, Line: i + 1, Rule: "LINK", Detail: "Undefined reference label: " + label})
				}
			}
			for _, target := range targets {
				local, detail := validateLink(name, target, inventory, anchors)
				if local {
					r.LocalLinkCount++
				}
				if detail != "" {
					r.Findings = append(r.Findings, finding{Path: name, Line: i + 1, Rule: "LINK", Detail: detail})
				}
			}
		}
	}
	return r
}

func validName(name string) bool {
	base := path.Base(name)
	if base == "README.md" {
		return true
	}
	if path.Dir(name) == "." {
		switch base {
		case "CHANGELOG.md", "CONTRIBUTING.md", "AGENTS.md", "BACKLOG.md", "THIRD-PARTY-NOTICES.md":
			return true
		}
	}
	return topicName.MatchString(base) && !datedName.MatchString(base) && !taskName.MatchString(base)
}

// Keep line positions while masking fenced examples. Comments inside an example
// remain visible for the prose scan; literal test payloads and identifiers do not.
func documentLines(text string, keepComments bool) []string {
	lines := strings.Split(text, "\n")
	fence := byte(0)
	width := 0
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if len(trimmed) >= 3 && (trimmed[0] == '`' || trimmed[0] == '~') {
			ch := trimmed[0]
			count := 0
			for count < len(trimmed) && trimmed[count] == ch {
				count++
			}
			if count >= 3 {
				if fence == 0 {
					fence = ch
					width = count
					lines[i] = ""
					continue
				}
				if ch == fence && count >= width && strings.TrimSpace(trimmed[count:]) == "" {
					fence = 0
					lines[i] = ""
					continue
				}
			}
		}
		if fence != 0 {
			if keepComments && (strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "// ")) {
				lines[i] = trimmed
			} else {
				lines[i] = ""
			}
		}
	}
	return lines
}

func stripInlineCode(line string) string {
	var out strings.Builder
	for i := 0; i < len(line); {
		if line[i] != '`' {
			out.WriteByte(line[i])
			i++
			continue
		}
		j := i
		for j < len(line) && line[j] == '`' {
			j++
		}
		close := strings.Index(line[j:], line[i:j])
		if close < 0 {
			out.WriteString(line[i:])
			break
		}
		i = j + close + (j - i)
	}
	return out.String()
}

func headingAnchors(text string) map[string]bool {
	ids := map[string]bool{}
	counts := map[string]int{}
	lines := documentLines(text, false)
	for i, l := range lines {
		for _, m := range explicitAnchor.FindAllStringSubmatch(l, -1) {
			ids[m[1]] = true
		}
		title := ""
		if m := headingLine.FindStringSubmatch(l); m != nil {
			title = m[1]
		}
		trimmed := strings.TrimSpace(l)
		if i > 0 && len(trimmed) >= 3 && (strings.Trim(trimmed, "=") == "" || strings.Trim(trimmed, "-") == "") && strings.TrimSpace(lines[i-1]) != "" {
			// Table separators contain pipes and therefore cannot be mistaken for headings.
			title = strings.TrimSpace(lines[i-1])
		}
		if title == "" {
			continue
		}
		title = html.UnescapeString(htmlTag.ReplaceAllString(title, ""))
		var slug strings.Builder
		for _, c := range strings.ToLower(title) {
			if c == ' ' {
				slug.WriteByte('-')
			} else if c == '-' || c == '_' || (!unicode.IsPunct(c) && !unicode.IsSymbol(c)) {
				slug.WriteRune(c)
			}
		}
		base := slug.String()
		n := counts[base]
		counts[base] = n + 1
		if n > 0 {
			base = fmt.Sprintf("%s-%d", base, n)
		}
		ids[base] = true
	}
	return ids
}

func readDestination(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if s[0] == '<' {
		i := strings.IndexByte(s, '>')
		if i < 0 {
			return "", false
		}
		return s[1:i], true
	}
	depth := 0
	for i, c := range s {
		if c == '(' {
			depth++
		}
		if c == ')' {
			if depth == 0 {
				return s[:i], i > 0
			}
			depth--
		}
		if unicode.IsSpace(c) && depth == 0 {
			return s[:i], i > 0
		}
	}
	return s, true
}

func inlineDestinations(line string) []string {
	result := []string{}
	for start := 0; start < len(line); {
		at := strings.Index(line[start:], "](")
		if at < 0 {
			break
		}
		at += start + 2
		if value, ok := readDestination(line[at:]); ok {
			result = append(result, value)
		}
		start = at
	}
	return result
}

func referenceTargets(line string, refs map[string]string) ([]string, []string) {
	result := []string{}
	missing := []string{}
	for i := 0; i < len(line); i++ {
		if line[i] != '[' {
			continue
		}
		end := strings.IndexByte(line[i+1:], ']')
		if end < 0 {
			break
		}
		end += i + 1
		label := line[i+1 : end]
		explicit := false
		if end+1 < len(line) && line[end+1] == '(' {
			i = end
			continue
		}
		if end+1 < len(line) && line[end+1] == '[' {
			explicit = true
			last := strings.IndexByte(line[end+2:], ']')
			if last < 0 {
				continue
			}
			last += end + 2
			if last > end+2 {
				label = line[end+2 : last]
			}
			end = last
		}
		key := strings.ToLower(strings.Join(strings.Fields(label), " "))
		if target, ok := refs[key]; ok {
			result = append(result, target)
		} else if explicit {
			missing = append(missing, label)
		}
		i = end
	}
	return result, missing
}

func validateLink(source, target string, inventory map[string]bool, anchors map[string]map[string]bool) (bool, string) {
	u, err := url.Parse(target)
	if err != nil {
		return true, "Invalid link destination: " + target
	}
	if strings.EqualFold(u.Scheme, "file") || (len(u.Scheme) == 1 && len(target) > 1 && target[1] == ':') {
		return true, "Use a repository-relative link: " + target
	}
	if u.Scheme != "" || u.Host != "" {
		return false, ""
	}
	p := u.Path
	if strings.ContainsAny(p, "\\\x00\r\n") || strings.HasPrefix(p, "/") {
		return true, "Use a repository-relative link: " + target
	}
	resolved := source
	if p != "" {
		resolved = path.Clean(path.Join(path.Dir(source), p))
	}
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return true, "Link escapes the repository: " + target
	}
	if !inventory[resolved] {
		return true, "Missing target or wrong path case: " + target
	}
	if u.Fragment != "" {
		if ids, ok := anchors[resolved]; ok && !ids[u.Fragment] {
			return true, "Missing heading/anchor: " + target
		}
	}
	return true, ""
}
