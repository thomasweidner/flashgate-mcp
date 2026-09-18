//go:build windows

package systeminfo

import (
	"fmt"
	"syscall"
)

func platformVersion() (string, error) {
	version, err := syscall.GetVersion()
	if err != nil {
		return "", err
	}
	major := byte(version)
	minor := uint8(version >> 8)
	build := uint16(version >> 16)
	return fmt.Sprintf("%d.%d.%d", major, minor, build), nil
}
