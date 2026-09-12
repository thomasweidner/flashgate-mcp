//go:build !linux && !windows

package systeminfo

import "errors"

func platformVersion() (string, error) {
	return "", errors.New("system information is unsupported on this platform")
}
