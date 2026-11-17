// Package resource supplies transport-independent size and budget contracts.
package resource

import (
	"errors"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var ErrInvalidSize = errors.New("invalid resource size")

// Decimal points are English; grouping is either commas or single spaces.
// Both the input bound and fixed unit vocabulary bound exact arithmetic work.
var sizePattern = regexp.MustCompile(`^([0-9]+|[0-9]{1,3}(?:,[0-9]{3})+|[0-9]{1,3}(?: [0-9]{3})+)(\.[0-9]+)?(?: ?([A-Za-z]+))?$`)

// ParseSize accepts a positive int64 byte count or a bounded English size
// string. Units are case sensitive: b is bits and B is bytes. Fractional byte
// ceilings are floored, and zero or values outside int64 are rejected.
func ParseSize(value any) (int64, error) {
	if n, ok := value.(int64); ok {
		if n > 0 {
			return n, nil
		}
		return 0, ErrInvalidSize
	}
	s, ok := value.(string)
	if !ok || len(s) == 0 || len(s) > 128 {
		return 0, ErrInvalidSize
	}
	parts := sizePattern.FindStringSubmatch(s)
	if parts == nil {
		return 0, ErrInvalidSize
	}
	unit := parts[3]
	bits := strings.HasSuffix(unit, "b")
	if unit != "" && !bits && !strings.HasSuffix(unit, "B") {
		return 0, ErrInvalidSize
	}
	prefix := ""
	if unit != "" {
		prefix = unit[:len(unit)-1]
	}
	base, power := int64(1000), 0
	switch prefix {
	case "":
	case "k", "M", "G", "T", "P", "E":
		power = strings.Index("kMGTPE", prefix) + 1
	case "Ki", "Mi", "Gi", "Ti", "Pi", "Ei":
		base = 1024
		power = strings.Index("KMGTPE", prefix[:1]) + 1
	default:
		return 0, ErrInvalidSize
	}
	number := strings.NewReplacer(",", "", " ", "").Replace(parts[1]) + parts[2]
	n, ok := new(big.Rat).SetString(number)
	if !ok {
		return 0, ErrInvalidSize
	}
	multiplier := new(big.Int).Exp(big.NewInt(base), big.NewInt(int64(power)), nil)
	n.Mul(n, new(big.Rat).SetInt(multiplier))
	if bits {
		n.Quo(n, big.NewRat(8, 1))
	}
	// Reject overflow before flooring, rather than accepting MaxInt64+fraction.
	if n.Cmp(new(big.Rat).SetInt64(1<<63-1)) > 0 {
		return 0, ErrInvalidSize
	}
	whole := new(big.Int).Quo(n.Num(), n.Denom())
	if whole.Sign() <= 0 || !whole.IsInt64() {
		return 0, ErrInvalidSize
	}
	return whole.Int64(), nil
}

// FormatSize chooses the largest IEC byte unit dividing the value exactly.
// Nonmultiples of KiB use bytes. It is pure, lossless and locale independent.
// Invalid nonpositive inputs have no representation.
func FormatSize(n int64) string {
	if n <= 0 {
		return ""
	}
	units := [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	i := 0
	for n%1024 == 0 && i < len(units)-1 {
		n /= 1024
		i++
	}
	return strconv.FormatInt(n, 10) + " " + units[i]
}
