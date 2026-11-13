//go:build linux

package benchmark

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

func TestParseLinuxClockTicks(t *testing.T) {
	tests := []struct {
		name      string
		wordSize  int
		byteOrder binary.ByteOrder
		entries   [][2]uint64
		want      uint64
		wantError string
	}{
		{
			name:      "64-bit non-100 tick rate",
			wordSize:  8,
			byteOrder: binary.LittleEndian,
			entries:   [][2]uint64{{6, 4096}, {linuxAuxvClockTicks, 250}, {0, 0}},
			want:      250,
		},
		{
			name:      "32-bit big endian",
			wordSize:  4,
			byteOrder: binary.BigEndian,
			entries:   [][2]uint64{{linuxAuxvClockTicks, 128}, {0, 0}},
			want:      128,
		},
		{
			name:      "missing clock ticks",
			wordSize:  8,
			byteOrder: binary.LittleEndian,
			entries:   [][2]uint64{{6, 4096}, {0, 0}},
			wantError: "omitted AT_CLKTCK",
		},
		{
			name:      "zero clock ticks",
			wordSize:  8,
			byteOrder: binary.LittleEndian,
			entries:   [][2]uint64{{linuxAuxvClockTicks, 0}},
			wantError: "zero AT_CLKTCK",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := encodeAuxv(t, tc.entries, tc.wordSize, tc.byteOrder)
			got, err := parseLinuxClockTicks(bytes.NewReader(data), tc.wordSize, tc.byteOrder)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error=%v, want containing %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("clock ticks=%d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseLinuxClockTicksRejectsMalformedInput(t *testing.T) {
	if _, err := parseLinuxClockTicks(bytes.NewReader(make([]byte, 15)), 8, binary.LittleEndian); err == nil {
		t.Fatal("truncated auxiliary vector accepted")
	}
	if _, err := parseLinuxClockTicks(bytes.NewReader(nil), 16, binary.LittleEndian); err == nil {
		t.Fatal("unsupported word size accepted")
	}
}

func TestReadLinuxCPUReportsClockTickLookupFailure(t *testing.T) {
	_, err := readLinuxCPU(1, 0, errors.New("clock unavailable"))
	if err == nil || !strings.Contains(err.Error(), "determine proc clock tick rate") {
		t.Fatalf("error=%v, want explicit clock tick lookup failure", err)
	}
}

func TestParseLinuxStatUsesNon100ClockTickRate(t *testing.T) {
	metrics, err := parseLinuxStat([]byte("123 (server) S 1 2 3 4 5 6 7 8 9 10 25 50"), 250)
	if err != nil {
		t.Fatal(err)
	}
	if metrics.userCPUNS == nil || *metrics.userCPUNS != 100_000_000 {
		t.Fatalf("user CPU=%v, want 100000000", metrics.userCPUNS)
	}
	if metrics.systemCPUNS == nil || *metrics.systemCPUNS != 200_000_000 {
		t.Fatalf("system CPU=%v, want 200000000", metrics.systemCPUNS)
	}
}

func encodeAuxv(t *testing.T, entries [][2]uint64, wordSize int, byteOrder binary.ByteOrder) []byte {
	t.Helper()
	var data bytes.Buffer
	for _, entry := range entries {
		for _, value := range entry {
			if wordSize == 4 {
				if err := binary.Write(&data, byteOrder, uint32(value)); err != nil {
					t.Fatal(err)
				}
			} else if err := binary.Write(&data, byteOrder, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	return data.Bytes()
}
