package benchmark

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestParseLinuxStatusSupportsPartialMetrics(t *testing.T) {
	tests := []struct {
		name            string
		status          string
		wantRSS         *uint64
		wantHWM         *uint64
		wantUnsupported string
	}{
		{"complete", "Name:\tserver\nVmRSS:\t123 kB\nVmHWM:\t456 kB\n", metricPointer(123 * 1024), metricPointer(456 * 1024), ""},
		{"missing VmHWM", "VmRSS:\t123 kB\n", metricPointer(123 * 1024), nil, "peak_working_set_bytes"},
		{"missing VmRSS", "VmHWM:\t456 kB\n", nil, metricPointer(456 * 1024), "idle_working_set_bytes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metrics, err := parseLinuxStatus(strings.NewReader(tc.status))
			if err != nil {
				t.Fatal(err)
			}
			if !equalOptionalMetric(metrics.workingSetBytes, tc.wantRSS) || !equalOptionalMetric(metrics.peakWorkingSetBytes, tc.wantHWM) {
				t.Fatalf("memory metrics=%+v", metrics)
			}
			if strings.Join(metrics.unsupported, ",") != tc.wantUnsupported {
				t.Fatalf("unsupported=%v, want %q", metrics.unsupported, tc.wantUnsupported)
			}
		})
	}
}

func TestParseLinuxStatHandlesProcessNamesAndInvalidInput(t *testing.T) {
	for _, name := range []string{"flashgate server", "flashgate (worker)"} {
		t.Run(name, func(t *testing.T) {
			stat := "123 (" + name + ") S 1 2 3 4 5 6 7 8 9 10 11 12 13 14"
			metrics, err := parseLinuxStat([]byte(stat), 100)
			if err != nil {
				t.Fatal(err)
			}
			if metrics.userCPUNS == nil || *metrics.userCPUNS != 110_000_000 || metrics.systemCPUNS == nil || *metrics.systemCPUNS != 120_000_000 {
				t.Fatalf("CPU metrics=%+v", metrics)
			}
		})
	}
	partial := []struct {
		name            string
		data            string
		wantUser        bool
		wantSystem      bool
		wantUnsupported string
	}{
		{"invalid user ticks", "1 (server) S 1 2 3 4 5 6 7 8 9 10 nope 12", false, true, "user_cpu_ns"},
		{"invalid system ticks", "1 (server) S 1 2 3 4 5 6 7 8 9 10 11 nope", true, false, "system_cpu_ns"},
		{"missing system ticks", "1 (server) S 1 2 3 4 5 6 7 8 9 10 11", true, false, "system_cpu_ns"},
	}
	for _, tc := range partial {
		t.Run(tc.name, func(t *testing.T) {
			metrics, err := parseLinuxStat([]byte(tc.data), 100)
			if err != nil {
				t.Fatal(err)
			}
			if (metrics.userCPUNS != nil) != tc.wantUser || (metrics.systemCPUNS != nil) != tc.wantSystem || strings.Join(metrics.unsupported, ",") != tc.wantUnsupported {
				t.Fatalf("partial CPU metrics=%+v", metrics)
			}
		})
	}
	invalid := []struct {
		name  string
		data  string
		ticks uint64
	}{
		{"zero tick rate", "1 (server) S 1 2 3 4 5 6 7 8 9 10 11 12", 0},
		{"missing closing parenthesis", "1 (server S 1 2", 100},
		{"missing fields", "1 (server) S 1 2", 100},
		{"both CPU fields invalid", "1 (server) S 1 2 3 4 5 6 7 8 9 10 nope nope", 100},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseLinuxStat([]byte(tc.data), tc.ticks); err == nil {
				t.Fatal("invalid proc stat accepted")
			}
		})
	}
}

func TestParseLinuxStatUsesNonStandardClockTickRate(t *testing.T) {
	stat := []byte("123 (server) S 1 2 3 4 5 6 7 8 9 10 25 50")
	metrics, err := parseLinuxStat(stat, 250)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.userCPUNS == nil || *metrics.userCPUNS != 100_000_000 || metrics.systemCPUNS == nil || *metrics.systemCPUNS != 200_000_000 {
		t.Fatalf("CPU metrics=%+v", metrics)
	}
}

func TestParseLinuxClockTicks(t *testing.T) {
	encode := func(wordBytes int, entries ...uint64) []byte {
		data := make([]byte, len(entries)*wordBytes)
		for index, value := range entries {
			if wordBytes == 4 {
				binary.LittleEndian.PutUint32(data[index*wordBytes:], uint32(value))
			} else {
				binary.LittleEndian.PutUint64(data[index*wordBytes:], value)
			}
		}
		return data
	}
	for _, wordBytes := range []int{4, 8} {
		data := encode(wordBytes, 6, 4096, linuxAuxvClockTicks, 250, 0, 0)
		got, err := parseLinuxClockTicks(data, wordBytes, binary.LittleEndian)
		if err != nil {
			t.Fatalf("word size %d: %v", wordBytes, err)
		}
		if got != 250 {
			t.Fatalf("word size %d: clock ticks=%d, want 250", wordBytes, got)
		}
	}

	invalid := []struct {
		name      string
		data      []byte
		wordBytes int
	}{
		{"unsupported word size", nil, 2},
		{"malformed length", []byte{1}, 8},
		{"missing clock ticks", encode(8, 6, 4096, 0, 0), 8},
		{"zero clock ticks", encode(8, linuxAuxvClockTicks, 0), 8},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseLinuxClockTicks(tc.data, tc.wordBytes, binary.LittleEndian); err == nil {
				t.Fatal("invalid auxiliary vector accepted")
			}
		})
	}
}

func equalOptionalMetric(left *uint64, right *uint64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
