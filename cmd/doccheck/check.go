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
var referenceDefinition = regexp.MustCompile(`(?m)^ {0,3}\[([^\[\]]+)\]:[ \t]*(?:\n[ \t]*)?`)
var linkBlockStart = regexp.MustCompile(`^ {0,3}(?:#{1,6}(?:[ \t]|$)|(?:[-*_][ \t]*){3,}$|=+[ \t]*$|\||<!--)`)
var linkListStart = regexp.MustCompile(`^ {0,3}([-+*]|[0-9]{1,9}[.)])(?: {1,4}|$)`)
var linkQuotePrefix = regexp.MustCompile(`^ {0,3}> ?`)
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
	if plan, present := documents[adapterPlan]; present {
		r.Findings = append(r.Findings, checkAdapterBindings(plan, documents["BACKLOG.md"])...)
	}
	if backlog, present := documents["BACKLOG.md"]; present {
		r.Findings = append(r.Findings, checkCatalogDispositions(backlog)...)
	}
	for _, name := range r.DocumentPaths {
		text := documents[name]
		if !validName(name) {
			r.Findings = append(r.Findings, finding{Path: name, Rule: "NAME", Detail: "Use a stable lower-kebab-case topic name without a task/date prefix, or an explicit conventional-name exception."})
		}
		lines := documentLines(text, false)
		languageLines := documentLines(text, true)
		links := documentLinks(lines)
		for i := range lines {
			visible := stripInlineCode(languageLines[i])
			for _, word := range words.FindAllString(visible, -1) {
				if germanWords[strings.ToLower(word)] {
					r.Findings = append(r.Findings, finding{Path: name, Line: i + 1, Rule: "LANGUAGE", Detail: "Review non-English prose candidate: " + word})
					break
				}
			}
			for _, link := range links[i] {
				if link.missingLabel {
					r.Findings = append(r.Findings, finding{Path: name, Line: i + 1, Rule: "LINK", Detail: "Undefined reference label: " + link.value})
					continue
				}
				target := link.value
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
	maskFencedLines(lines, keepComments, nil)
	return lines
}

func maskFencedLines(lines []string, keepComments bool, quoteDepth []int) {
	fence := byte(0)
	width := 0
	fenceDepth := 0
	for i, l := range lines {
		depth := 0
		if quoteDepth != nil {
			depth = quoteDepth[i]
			if depth < fenceDepth {
				fence = 0
			}
			if depth == 0 {
				continue
			}
			if fence != 0 && depth > fenceDepth {
				lines[i] = ""
				continue
			}
		}
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
					fenceDepth = depth
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

type destinationIndex struct {
	source string
	ends   []int32
}

func isBackslashEscaped(source string, offset int) bool {
	start := offset
	for start > 0 && source[start-1] == '\\' {
		start--
	}
	return (offset-start)%2 != 0
}

// Work from right to left within the bounded block. Each closing parenthesis
// saves the outcome after the pair, so a matching opener skips the entire pair.
// An unmatched opener makes the rest of that bare destination invalid.
func destinationEnds(source string) []int32 {
	ends := make([]int32, len(source))
	bareEnd := int32(len(source))
	angleStop := len(source)
	var afterClose []int32
	for i := len(source); i > 0; {
		r, width := utf8.DecodeLastRuneInString(source[:i])
		i -= width
		switch {
		case unicode.IsSpace(r):
			bareEnd = int32(i)
			afterClose = afterClose[:0]
		case r == ')' && !isBackslashEscaped(source, i):
			afterClose = append(afterClose, bareEnd)
			bareEnd = int32(i)
		case r == '(' && !isBackslashEscaped(source, i):
			bareEnd = -1
			if n := len(afterClose); n > 0 {
				bareEnd = afterClose[n-1]
				afterClose = afterClose[:n-1]
			}
		}
		ends[i] = bareEnd
		if r == '<' {
			ends[i] = -1
			if angleStop < len(source) && source[angleStop] == '>' {
				ends[i] = int32(angleStop + 1)
			}
		}
		if r == '<' || r == '>' || r == '\n' {
			angleStop = i
		}
	}
	return ends
}

// Build the index only when the block yields a candidate destination. Every
// subsequent lookup uses one precomputed endpoint, even for malformed input.
// Keep the destination bytes intact for the path validator.
func (d *destinationIndex) at(start int) (string, int, bool) {
	if start == len(d.source) {
		return "", 0, false
	}
	if d.ends == nil {
		d.ends = destinationEnds(d.source)
	}
	end := int(d.ends[start])
	if end <= start {
		return "", 0, false
	}
	if d.source[start] == '<' {
		return d.source[start+1 : end-1], end, true
	}
	return d.source[start:end], end, true
}

// The adapter binding checks extract links from individual table cells.
func inlineDestinations(line string) []string {
	result := []string{}
	destinations := destinationIndex{source: line}
	for start := 0; start < len(line); {
		at := strings.Index(line[start:], "](")
		if at < 0 {
			break
		}
		start += at + 2
		tail := strings.TrimLeftFunc(line[start:], unicode.IsSpace)
		if target, _, ok := destinations.at(len(line) - len(tail)); ok {
			result = append(result, target)
		}
	}
	return result
}

// linkEnd checks the suffix after a destination. A definition ends on the
// destination line or after its title. An inline link needs a closing ')'.
func linkEnd(s string, definition bool) (int, bool) {
	rest := strings.TrimLeft(s, " \t\n")
	start := len(s) - len(rest)
	if start > 0 && rest != "" {
		close := rest[0]
		switch close {
		case '(':
			close = ')'
		case '"', '\'':
		default:
			close = 0
		}
		if close != 0 {
			for i := 1; i < len(rest); i++ {
				if rest[i] == '\\' && i+1 < len(rest) {
					i++
					continue
				}
				if close == ')' && rest[i] == '(' {
					break
				}
				if rest[i] == close {
					tail := rest[i+1:]
					if definition {
						tail = strings.TrimLeft(tail, " \t")
						if tail == "" || tail[0] == '\n' {
							return start + i + 1, true
						}
					} else {
						tail = strings.TrimLeft(tail, " \t\n")
						if tail != "" && tail[0] == ')' {
							return len(s) - len(tail) + 1, true
						}
					}
					break
				}
			}
		}
	}
	if definition {
		line := strings.TrimLeft(s, " \t")
		return 0, line == "" || line[0] == '\n'
	}
	return start + 1, rest != "" && rest[0] == ')'
}

type linkCheck struct {
	value        string
	missingLabel bool
}

type linkReference struct {
	line     int
	label    string
	explicit bool
}

func referenceKey(label string) string {
	return strings.ToLower(strings.Join(strings.Fields(label), " "))
}

// Code spans may cross lines. Match entire backtick runs and preserve offsets
// while masking their contents, including any link-shaped literal text.
func maskLinkCode(block string) string {
	const backtick = 0x60
	if !strings.ContainsRune(block, backtick) {
		return block
	}
	type run struct{ start, end, next int }
	var runs []run
	for i := 0; i < len(block); i++ {
		if block[i] == backtick {
			start := i
			for i+1 < len(block) && block[i+1] == backtick {
				i++
			}
			runs = append(runs, run{start: start, end: i + 1})
		}
	}
	next := map[int]int{}
	for i := len(runs) - 1; i >= 0; i-- {
		width := runs[i].end - runs[i].start
		runs[i].next = next[width]
		next[width] = i + 1
	}
	var masked []byte
	for i := 0; i < len(runs); i++ {
		start := runs[i].start
		if runs[i].next == 0 || isBackslashEscaped(block, start) {
			continue
		}
		if masked == nil {
			masked = []byte(block)
		}
		end := runs[i].next - 1
		for j := start; j < runs[end].end; j++ {
			if masked[j] != '\n' {
				masked[j] = 0
			}
		}
		i = end
	}
	if masked == nil {
		return block
	}
	return string(masked)
}

func listInterruptsLinkParagraph(line string, outsideItem bool) bool {
	marker := linkListStart.FindStringSubmatch(line)
	if marker == nil {
		return false
	}
	// A marker at the current list's level ends the item. Inside an item or
	// outside a list, starting a new list requires content and, if ordered, 1.
	if outsideItem {
		return true
	}
	if strings.TrimSpace(line[len(marker[0]):]) == "" {
		return false
	}
	number := marker[1][:len(marker[1])-1]
	return number == "" || strings.TrimLeft(number, "0") == "1"
}

func documentLinks(lines []string) map[int][]linkCheck {
	checks := map[int][]linkCheck{}
	definitions := map[string]string{}
	var references []linkReference
	quoteDepth := make([]int, len(lines))
	for i := range lines {
		line := strings.TrimSuffix(lines[i], "\r")
		for prefix := linkQuotePrefix.FindStringIndex(line); prefix != nil; prefix = linkQuotePrefix.FindStringIndex(line) {
			quoteDepth[i]++
			line = line[prefix[1]:]
		}
		lines[i] = line
	}
	maskFencedLines(lines, false, quoteDepth)
	for first := 0; first < len(lines); {
		if strings.TrimSpace(lines[first]) == "" {
			first++
			continue
		}
		end := first + 1
		if !linkBlockStart.MatchString(lines[first]) {
			var indent string
			if prefix := linkListStart.FindStringIndex(lines[first]); prefix != nil {
				indent = strings.Repeat(" ", prefix[1])
				lines[first] = lines[first][prefix[1]:]
			}
			// Paragraph text can continue with omitted quote markers or list
			// indentation. Blank lines and block markers still end the block.
			for end < len(lines) && quoteDepth[end] <= quoteDepth[first] {
				line, insideItem := strings.CutPrefix(lines[end], indent)
				outsideItem := !insideItem && quoteDepth[end] == quoteDepth[first]
				if strings.TrimSpace(line) == "" || linkBlockStart.MatchString(line) || listInterruptsLinkParagraph(line, outsideItem) {
					break
				}
				lines[end] = line
				end++
			}
		}
		block := strings.Join(lines[first:end], "\n")
		if strings.ContainsRune(block, '[') {
			references = scanLinkBlock(block, lines[first:end], first, checks, definitions, references)
		}
		first = end
	}
	for _, ref := range references {
		if target, ok := definitions[referenceKey(ref.label)]; ok {
			checks[ref.line] = append(checks[ref.line], linkCheck{value: target})
		} else if ref.explicit {
			checks[ref.line] = append(checks[ref.line], linkCheck{value: ref.label, missingLabel: true})
		}
	}
	return checks
}

func scanLinkBlock(raw string, lines []string, first int, checks map[int][]linkCheck, definitions map[string]string, references []linkReference) []linkReference {
	block := maskLinkCode(raw)
	destinations := destinationIndex{source: raw}
	starts := make([]int, len(lines))
	for i := 1; i < len(lines); i++ {
		starts[i] = starts[i-1] + len(lines[i-1]) + 1
	}
	sourceLine := func(offset int) int { return first + sort.SearchInts(starts, offset+1) - 1 }
	addLink := func(offset int, target string) {
		line := sourceLine(offset)
		checks[line] = append(checks[line], linkCheck{value: target})
	}

	// Collect definition targets and skip their syntax during inline scanning.
	definitionEnds := map[int]int{}
	previousEnd := 0
	for _, m := range referenceDefinition.FindAllStringSubmatchIndex(block, -1) {
		if m[0] < previousEnd {
			continue
		}
		target, targetEnd, ok := destinations.at(m[1])
		if !ok {
			continue
		}
		suffix, ok := linkEnd(raw[targetEnd:], true)
		if !ok {
			continue
		}
		definitions[referenceKey(raw[m[2]:m[3]])] = target
		previousEnd = targetEnd + suffix
		definitionEnds[m[0]] = previousEnd
		addLink(m[2]-1, target)
	}

	type bracket struct {
		offset    int
		hasNested bool
		image     bool
	}
	var opens []bracket
	inactive := 0
	for i := 0; i < len(block); i++ {
		if end, ok := definitionEnds[i]; ok {
			i = end - 1
			opens = opens[:0]
			inactive = 0
			continue
		}
		switch block[i] {
		case '\\':
			if i+1 < len(block) && strings.ContainsRune("\\[]", rune(block[i+1])) {
				i++
			}
		case '[':
			if len(opens) != 0 {
				opens[len(opens)-1].hasNested = true
			}
			opens = append(opens, bracket{offset: i, image: i > 0 && block[i-1] == '!'})
		case ']':
			depth := len(opens)
			if depth == 0 {
				continue
			}
			open := opens[depth-1]
			opens = opens[:depth-1]
			if depth <= inactive {
				inactive = depth - 1
				if !open.image {
					continue
				}
			}
			tail := block[i+1:]
			if strings.HasPrefix(tail, "(") {
				dest := strings.TrimLeft(raw[i+2:], " \t\n")
				target, targetEnd, ok := destinations.at(len(raw) - len(dest))
				if ok {
					if suffix, closed := linkEnd(raw[targetEnd:], false); closed {
						addLink(open.offset, target)
						i = targetEnd + suffix - 1
						// A link can contain an image but not another link.
						if !open.image {
							inactive = len(opens)
						}
					}
				}
				continue
			}
			label := raw[open.offset+1 : i]
			explicit := false
			if strings.HasPrefix(tail, "[") {
				last := strings.IndexAny(tail[1:], "[]")
				if last >= 0 && tail[last+1] == ']' {
					explicit = true
					if last > 0 {
						label = raw[i+2 : i+2+last]
						open.hasNested = false
					}
					i += last + 2
				}
			}
			if !open.hasNested {
				references = append(references, linkReference{line: sourceLine(open.offset), label: label, explicit: explicit})
			}
		}
	}
	return references
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
