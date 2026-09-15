//go:build linux

package benchmark

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
)

const linuxAuxvClockTicks = 17 // AT_CLKTCK from linux/auxvec.h.

type linuxProcessMetricReader struct {
	pid           int
	clockTicks    uint64
	clockTicksErr error
}

func newProcessMetricReader(pid int) (processMetricReader, error) {
	clockTicks, clockTicksErr := readLinuxClockTicks()
	return &linuxProcessMetricReader{pid: pid, clockTicks: clockTicks, clockTicksErr: clockTicksErr}, nil
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
	cpu, cpuErr := readLinuxCPU(r.pid, r.clockTicks, r.clockTicksErr)
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

func readLinuxCPU(pid int, clockTicks uint64, clockTicksErr error) (linuxCPUMetrics, error) {
	if clockTicksErr != nil {
		return linuxCPUMetrics{}, fmt.Errorf("determine proc clock tick rate: %w", clockTicksErr)
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return linuxCPUMetrics{}, fmt.Errorf("read proc stat: %w", err)
	}
	return parseLinuxStat(data, clockTicks)
}

func readLinuxClockTicks() (uint64, error) {
	file, err := os.Open("/proc/self/auxv")
	if err != nil {
		return 0, fmt.Errorf("open process auxiliary vector: %w", err)
	}
	defer file.Close()

	return parseLinuxClockTicks(file, strconv.IntSize/8, binary.NativeEndian)
}

func parseLinuxClockTicks(reader io.Reader, wordSize int, byteOrder binary.ByteOrder) (uint64, error) {
	if wordSize != 4 && wordSize != 8 {
		return 0, fmt.Errorf("unsupported auxiliary vector word size %d", wordSize)
	}
	entry := make([]byte, wordSize*2)
	for {
		_, err := io.ReadFull(reader, entry)
		if err == io.EOF {
			return 0, fmt.Errorf("auxiliary vector omitted AT_CLKTCK")
		}
		if err != nil {
			return 0, fmt.Errorf("read auxiliary vector: %w", err)
		}

		var tag, value uint64
		if wordSize == 4 {
			tag = uint64(byteOrder.Uint32(entry[:wordSize]))
			value = uint64(byteOrder.Uint32(entry[wordSize:]))
		} else {
			tag = byteOrder.Uint64(entry[:wordSize])
			value = byteOrder.Uint64(entry[wordSize:])
		}
		if tag == 0 {
			return 0, fmt.Errorf("auxiliary vector omitted AT_CLKTCK")
		}
		if tag == linuxAuxvClockTicks {
			if value == 0 {
				return 0, fmt.Errorf("auxiliary vector contains zero AT_CLKTCK")
			}
			return value, nil
		}
	}
}
