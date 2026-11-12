package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"unicode"
)

var (
	authHeaderPattern     = regexp.MustCompile(`(?i)(Authorization:\s*)(Bearer|Basic)\s+[^\s,;]+`)
	assignmentPattern     = regexp.MustCompile(`(?i)\b(password|token|api_key|apikey|secret)\b\s*[:=]\s*("[^"]*"|'[^']*'|[^\s,;]+)`)
	jsonAssignmentPattern = regexp.MustCompile(`(?i)"(password|token|api_key|apikey|secret)"\s*:\s*"[^"]*"`)
	privateKeyPattern     = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)
	sshKeyPattern         = regexp.MustCompile(`-----BEGIN OPENSSH PRIVATE KEY-----`)
	urlUserInfoPattern    = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^/\s:@]+:[^@\s/]+@`)
	uncPathPattern        = regexp.MustCompile(`\\\\[^\s\\/:*?"<>|]+\\[^\s\\/:*?"<>|]+(?:\\[^\s\\/:*?"<>|]+)*`)
	windowsPathPattern    = regexp.MustCompile(`[A-Za-z]:\\[^\s:*?"<>|]+(?:\\[^\s:*?"<>|]+)*`)
	posixPathPattern      = regexp.MustCompile(`(^|[\s"'])/(?:[^/\s"']+/)+[^\s"']*`)
)

// Logger writes redacted diagnostics to stderr-like writers when debug is enabled.
type Logger struct {
	debug      bool
	jsonFormat bool
	maxBytes   int
	maxRecords int
	out        io.Writer
	mu         sync.Mutex
	records    int
}

// NewLogger creates a new diagnostics logger.
func NewLogger(debug bool, out io.Writer) *Logger {
	return NewBoundedLogger(debug, false, 4096, 10000, out)
}

// NewBoundedLogger creates a logger with validated per-record and lifetime caps.
func NewBoundedLogger(debug, jsonFormat bool, maxBytes, maxRecords int, out io.Writer) *Logger {
	return &Logger{debug: debug, jsonFormat: jsonFormat, maxBytes: maxBytes, maxRecords: maxRecords, out: out}
}

// Debugf writes a redacted debug line when debug logging is enabled.
func (l *Logger) Debugf(format string, args ...any) {
	if l == nil || !l.debug || l.out == nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.records >= l.maxRecords {
		return
	}
	l.records++

	message := strings.Map(func(ch rune) rune {
		if unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch) || ch == '\u2028' || ch == '\u2029' {
			return ' '
		}
		return ch
	}, Redact(fmt.Sprintf(format, args...)))
	record := l.formatRecord(message)
	if len(record) > l.maxBytes {
		record = l.formatRecord("[TRUNCATED]")
	}
	if len(record) <= l.maxBytes {
		_, _ = l.out.Write(record)
	}
}

func (l *Logger) formatRecord(message string) []byte {
	if !l.jsonFormat {
		return []byte("flashgate-mcp: " + message + "\n")
	}
	record, _ := json.Marshal(struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}{"debug", message})
	return append(record, '\n')
}

// Redact removes common secret and host-path patterns from diagnostic text.
func Redact(input string) string {
	redacted := authHeaderPattern.ReplaceAllString(input, `${1}${2} [REDACTED]`)
	redacted = jsonAssignmentPattern.ReplaceAllString(redacted, `"$1":"[REDACTED]"`)
	redacted = assignmentPattern.ReplaceAllString(redacted, `$1=[REDACTED]`)
	redacted = privateKeyPattern.ReplaceAllString(redacted, `[REDACTED_PRIVATE_KEY]`)
	redacted = sshKeyPattern.ReplaceAllString(redacted, `[REDACTED_PRIVATE_KEY]`)
	redacted = urlUserInfoPattern.ReplaceAllString(redacted, `${1}[REDACTED]@`)
	redacted = uncPathPattern.ReplaceAllString(redacted, `[REDACTED_PATH]`)
	redacted = windowsPathPattern.ReplaceAllString(redacted, `[REDACTED_PATH]`)
	redacted = posixPathPattern.ReplaceAllString(redacted, `${1}[REDACTED_PATH]`)

	return redacted
}
