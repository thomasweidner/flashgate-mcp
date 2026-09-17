package diagnostics

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
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
	sensitiveEnvName      = regexp.MustCompile(`(?i)(^|_)(auth|authorization|cookie|credential|key|password|secret|token)($|_)`)
)

const redactedValue = "[REDACTED]"

// ExecutionRedactor removes diagnostic patterns and sensitive environment
// values from command output, diagnostics, and audit text. It is immutable and
// safe for concurrent use after construction.
type ExecutionRedactor struct {
	replacer *strings.Replacer
}

// NewExecutionRedactor creates a redactor from an execution environment and
// any additional values known to be sensitive. Only environment variables
// whose names identify secret material contribute output replacements; use
// additionalSecrets for values whose variable names are application-specific.
func NewExecutionRedactor(environment map[string]string, additionalSecrets ...string) *ExecutionRedactor {
	secrets := make(map[string]struct{}, len(additionalSecrets))
	for name, value := range environment {
		if sensitiveEnvName.MatchString(name) {
			addSecret(secrets, value)
		}
	}
	for _, value := range additionalSecrets {
		addSecret(secrets, value)
	}

	values := make([]string, 0, len(secrets))
	for value := range secrets {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		if len(values[i]) == len(values[j]) {
			return values[i] < values[j]
		}
		return len(values[i]) > len(values[j])
	})

	replacements := make([]string, 0, len(values)*2)
	for _, value := range values {
		replacements = append(replacements, value, redactedValue)
	}

	var replacer *strings.Replacer
	if len(replacements) > 0 {
		replacer = strings.NewReplacer(replacements...)
	}
	return &ExecutionRedactor{replacer: replacer}
}

func addSecret(secrets map[string]struct{}, value string) {
	if value == "" || value == redactedValue {
		return
	}
	secrets[value] = struct{}{}
}

// Redact removes known execution secrets followed by the common diagnostic
// secret and host-path patterns.
func (r *ExecutionRedactor) Redact(input string) string {
	if r != nil && r.replacer != nil {
		input = r.replacer.Replace(input)
	}
	return Redact(input)
}

// RedactedEnvironment returns a copy suitable for results, diagnostics, and
// audit events. Names are retained for troubleshooting; values are never
// returned. The input map is not mutated.
func RedactedEnvironment(environment map[string]string) map[string]string {
	redacted := make(map[string]string, len(environment))
	for name := range environment {
		redacted[name] = redactedValue
	}
	return redacted
}

// Logger writes redacted diagnostics to stderr-like writers when debug is enabled.
type Logger struct {
	debug bool
	out   io.Writer
}

// NewLogger creates a new diagnostics logger.
func NewLogger(debug bool, out io.Writer) *Logger {
	return &Logger{
		debug: debug,
		out:   out,
	}
}

// Debugf writes a redacted debug line when debug logging is enabled.
func (l *Logger) Debugf(format string, args ...any) {
	if l == nil || !l.debug || l.out == nil {
		return
	}

	message := fmt.Sprintf(format, args...)
	fmt.Fprintf(l.out, "flashgate-mcp: %s\n", Redact(message))
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
