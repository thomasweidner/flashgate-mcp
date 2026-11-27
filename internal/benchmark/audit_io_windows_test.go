package benchmark

import (
	"errors"
	"fmt"
	"unsafe"
)

const auditReadIOName = "process_read_transfer_B"
const auditWriteIOName = "process_write_transfer_B"

var getProcessIoCounters = kernel32DLL.NewProc("GetProcessIoCounters")

func auditProcessIO(pid int) (_ [2]uint64, err error) {
	reader, err := newProcessMetricReader(pid)
	if err != nil {
		return [2]uint64{}, err
	}
	defer func() { err = errors.Join(err, reader.Close()) }()
	windowsReader := reader.(*windowsProcessMetricReader)
	// IO_COUNTERS stores three operation counts then three transfer counts.
	var counters [6]uint64
	ok, _, callErr := getProcessIoCounters.Call(uintptr(windowsReader.handle), uintptr(unsafe.Pointer(&counters)))
	if ok == 0 {
		return [2]uint64{}, fmt.Errorf("GetProcessIoCounters: %w", callErr)
	}
	return [2]uint64{counters[3], counters[4]}, nil
}
