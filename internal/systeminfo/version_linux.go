//go:build linux

package systeminfo

import (
	"bytes"
	"syscall"
)

func platformVersion() (string, error) {
	var name syscall.Utsname
	if err := syscall.Uname(&name); err != nil {
		return "", err
	}
	return charsToString(name.Release[:]), nil
}

func charsToString(value []int8) string {
	buffer := make([]byte, 0, len(value))
	for _, char := range value {
		if char == 0 {
			break
		}
		buffer = append(buffer, byte(char))
	}
	return string(bytes.ToValidUTF8(buffer, []byte("?")))
}
