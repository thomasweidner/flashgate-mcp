package benchmark

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const auditReadIOName = "storage_read_B"
const auditWriteIOName = "storage_write_B"

func auditProcessIO(pid int) ([2]uint64, error) {
	var result [2]uint64
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/io", pid))
	if err != nil {
		return result, err
	}
	for index, key := range [2]string{"read_bytes:", "write_bytes:"} {
		_, value, found := strings.Cut(string(raw), "\n"+key)
		if !found {
			return result, fmt.Errorf("procfs storage I/O counter %s unavailable", key)
		}
		line, _, _ := strings.Cut(value, "\n")
		result[index], err = strconv.ParseUint(strings.TrimSpace(line), 10, 64)
		if err != nil {
			return result, fmt.Errorf("parse procfs I/O counter %s: %w", key, err)
		}
	}
	return result, nil
}
