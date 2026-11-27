package resource

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidDuration = errors.New("invalid resource duration")

// ParseDuration accepts a positive integer and one explicit unit. Days are
// fixed 24-hour intervals, independent of the local time zone.
func ParseDuration(value string) (time.Duration, error) {
	if len(value) == 0 || len(value) > 32 {
		return 0, ErrInvalidDuration
	}
	number, unit, ok := strings.Cut(value, " ")
	if !ok || number == "" || number[0] < '0' || number[0] > '9' {
		return 0, ErrInvalidDuration
	}
	n, err := strconv.ParseUint(number, 10, 63)
	if err != nil || n == 0 {
		return 0, ErrInvalidDuration
	}
	var scale time.Duration
	switch unit {
	case "ms":
		scale = time.Millisecond
	case "s":
		scale = time.Second
	case "min":
		scale = time.Minute
	case "h":
		scale = time.Hour
	case "d":
		scale = 24 * time.Hour
	default:
		return 0, ErrInvalidDuration
	}
	if n > uint64(math.MaxInt64/int64(scale)) {
		return 0, ErrInvalidDuration
	}
	return time.Duration(n) * scale, nil
}
