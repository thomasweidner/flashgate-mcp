package resource

import (
	"errors"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	for input, expected := range map[string]time.Duration{
		"100 ms": 100 * time.Millisecond, "2 s": 2 * time.Second,
		"5 min": 5 * time.Minute, "24 h": 24 * time.Hour, "30 d": 720 * time.Hour,
	} {
		value, err := ParseDuration(input)
		if err != nil || value != expected {
			t.Fatalf("%q = %s, %v", input, value, err)
		}
	}
	for _, input := range []string{
		"", "1", "1ms", "0 s", "-1 d", "+1 s", "1.5 s", "1 hour", "1  s",
		"9223372036854775807 ms", "99000800902912000000000000000000 d",
	} {
		if _, err := ParseDuration(input); !errors.Is(err, ErrInvalidDuration) {
			t.Fatalf("expected invalid duration for %q, got %v", input, err)
		}
	}
}
