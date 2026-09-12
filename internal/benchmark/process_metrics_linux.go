//go:build linux

package benchmark

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
)

const linuxAuxvClockTicks = 17 // AT_CLKTCK from linux/auxvec.h.

type linuxProcessMetricReader struct {
	pid int
}

func newProcessMetricReader(pid int) (processMetricReader, error) {
	return &linuxProcessMetricReader{pid: pid}, nil
}

func (r *linuxProcessMetricReader) Snapshot() (processSnapshot, error) {
	snapshot := processSnapshot{status: "not_supported"}
	memory, memoryErr := readLinuxMemory(r.pid)
	if memoryErr != nil {
		snapshot.unsupported = append(snapshot.unsupported, "idle_working_set_bytes", "peak_working_set_bytes")
	} else {
		snapshot.workingSetBytes = memory.workingSetBytes
		snapshot.peakWorkingSetBytes = memory.peakWorkingSetBytes
		snapshot.unsupported = append(snapshot.unsupported, memory.unsupported...)
	}
	cpu, cpuErr := readLinuxCPU(r.pid)
	if cpuErr != nil {
		snapshot.unsupported = append(snapshot.unsupported, "user_cpu_ns", "system_cpu_ns")
	} else {
		snapshot.userCPUNS = cpu.userCPUNS
		snapshot.systemCPUNS = cpu.systemCPUNS
		snapshot.unsupported = append(snapshot.unsupported, cpu.unsupported...)
	}
	snapshot.unsupported = uniqueSorted(snapshot.unsupported)
	if snapshot.workingSetBytes != nil || snapshot.peakWorkingSetBytes != nil || snapshot.userCPUNS != nil || snapshot.systemCPUNS != nil {
		snapshot.status = "supported"
		return snapshot, nil
	}
	return snapshot, fmt.Errorf("procfs resource metrics unavailable: memory: %v; CPU: %v", memoryErr, cpuErr)
}

func (r *linuxProcessMetricReader) Close() error { return nil }

func readLinuxMemory(pid int) (linuxMemoryMetrics, error) {
	file, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return linuxMemoryMetrics{}, fmt.Errorf("open proc status: %w", err)
	}
	defer file.Close()
	return parseLinuxStatus(file)
}

func readLinuxCPU(pid int) (linuxCPUMetrics, error) {
	ticksPerSecond, err := readLinuxClockTicks()
	if err != nil {
		return linuxCPUMetrics{}, fmt.Errorf("read Linux clock ticks: %w", err)
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return linuxCPUMetrics{}, fmt.Errorf("read proc stat: %w", err)
	}
	return parseLinuxStat(data, ticksPerSecond)
}

func readLinuxClockTicks() (uint64, error) {
	data, err := os.ReadFile("/proc/self/auxv")
	if err != nil {
		return 0, fmt.Errorf("read proc auxiliary vector: %w", err)
	}
	return parseLinuxClockTicks(data, strconv.IntSize/8, binary.NativeEndian)
}

func parseLinuxClockTicks(data []byte, wordBytes int, order binary.ByteOrder) (uint64, error) {
	if wordBytes != 4 && wordBytes != 8 {
		return 0, fmt.Errorf("unsupported auxiliary vector word size %d", wordBytes)
	}
	entryBytes := wordBytes * 2
	if len(data)%entryBytes != 0 {
		return 0, fmt.Errorf("malformed auxiliary vector length")
	}
	readWord := func(data []byte) uint64 {
		if wordBytes == 4 {
			return uint64(order.Uint32(data))
		}
		return order.Uint64(data)
	}
	for offset := 0; offset < len(data); offset += entryBytes {
		tag := readWord(data[offset : offset+wordBytes])
		value := readWord(data[offset+wordBytes : offset+entryBytes])
		if tag == linuxAuxvClockTicks {
			if value == 0 {
				return 0, fmt.Errorf("auxiliary vector clock tick rate is zero")
			}
			return value, nil
		}
		if tag == 0 { // AT_NULL
			break
		}
	}
	return 0, fmt.Errorf("auxiliary vector omits AT_CLKTCK")
}
