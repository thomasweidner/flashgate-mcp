package resource

import (
	"math"
	"strings"
	"testing"
)

func TestParseSize(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want int64
	}{
		{int64(1234), 1234}, {"1 234", 1234}, {"1,234", 1234}, {"1,234,567 B", 1234567},
		{"1 234 567.9 B", 1234567}, {"8.3 GB", 8300000000}, {"8.3 GiB", 8912057139},
		{"10MiB", 10485760}, {"10 MiB", 10485760}, {"10.48576 MB", 10485760},
		{"8 Gb", 1000000000}, {"8 GB", 8000000000}, {"8 Gib", 1073741824}, {"9b", 1},
		{"1.9 B", 1}, {"8 b", 1}, {"0.001 kB", 1}, {"1 EB", 1000000000000000000},
		{"1 EiB", 1 << 60}, {"1 PiB", 1 << 50}, {"1 TiB", 1 << 40},
		{"1 PB", 1000000000000000}, {"1 TB", 1000000000000},
		{int64(math.MaxInt64), math.MaxInt64}, {"9223372036854775807 B", math.MaxInt64},
	} {
		got, err := ParseSize(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseSize(%v) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
}

func TestParseSizeRejectsInvalid(t *testing.T) {
	for _, in := range []any{
		nil, true, 1.0, int(1), int64(0), int64(-1), []int{1}, "", "0", "-1", "+1", " 1 B", "1 B ",
		"8,3 GB", "8.388.608", "1,23 B", "12 34 B", "1,234 567 B", "1 234,567 B", "1  234 B",
		"1.2.3 B", "1. B", ".1 B", "1e3 B", "1 KB", "1 mb", "1 kib", "1 Bytes", "1 MiBB", "1 ZB",
		"1\tMiB", "1\nMiB", "1  MiB", "7 b", "0.01 B", "1.5 b", "1.00000000000000000001 b",
		"9223372036854775808", "9223372036854775807.1 B", "8 EiB", strings.Repeat("1", 129),
	} {
		if got, err := ParseSize(in); err == nil {
			t.Errorf("accepted %v as %d", in, got)
		}
	}
}

func TestFormatSize(t *testing.T) {
	for _, tc := range []struct {
		n    int64
		want string
	}{
		{0, ""}, {-1, ""}, {1, "1 B"}, {1023, "1023 B"}, {1024, "1 KiB"}, {1536, "1536 B"},
		{10485760, "10 MiB"}, {1 << 30, "1 GiB"}, {1 << 40, "1 TiB"}, {1 << 50, "1 PiB"}, {1 << 60, "1 EiB"},
		{math.MaxInt64, "9223372036854775807 B"},
	} {
		if got := FormatSize(tc.n); got != tc.want {
			t.Errorf("format %d = %q", tc.n, got)
		}
		if tc.n > 0 {
			n, err := ParseSize(FormatSize(tc.n))
			if n != tc.n || err != nil {
				t.Fatal("lossy roundtrip", tc.n)
			}
		}
	}
	for n := int64(1); n < 100000; n += 137 {
		parsed, err := ParseSize(FormatSize(n))
		if err != nil || parsed != n {
			t.Fatal(n, parsed, err)
		}
	}
}

func FuzzSizeRoundtrip(f *testing.F) {
	f.Add(int64(10485760))
	f.Add(int64(math.MaxInt64))
	f.Add(int64(1))
	f.Fuzz(func(t *testing.T, n int64) {
		if n <= 0 {
			return
		}
		got, err := ParseSize(FormatSize(n))
		if err != nil || got != n {
			t.Fatal(n, got, err)
		}
	})
}
