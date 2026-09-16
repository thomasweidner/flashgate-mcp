//go:build !windows && !linux

package managedprocess

import (
	"io"
	"os/exec"
)

func defaultProcessAdapter() ProcessAdapter { return processAdapterFunc(startPortableProcess) }

func startPortableProcess(launch Launch, stdout, stderr io.Writer) (startedProcess, error) {
	if launch.Resources.required() {
		return nil, ErrResourceControlUnavailable
	}
	command := exec.Command(launch.Executable, launch.Arguments...)
	command.Dir, command.Env = launch.WorkingDirectory, append([]string(nil), launch.Environment...)
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Start(); err != nil {
		return nil, err
	}
	return osProcess{command}, nil
}
