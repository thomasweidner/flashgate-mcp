//go:build linux

package managedprocess

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const linuxCPUPeriodMicros = 1_000_000

// LinuxAdapter starts limited workloads in private leaves below a delegated
// cgroup v2 root. CgroupRoot must be an absolute, service-owned delegation;
// the adapter never mutates the host cgroup hierarchy to obtain delegation.
type LinuxAdapter struct {
	CgroupRoot string
}

func defaultProcessAdapter() ProcessAdapter { return LinuxAdapter{} }

func (adapter LinuxAdapter) Start(launch Launch, stdout, stderr io.Writer) (startedProcess, error) {
	if err := launch.Resources.validate(); err != nil {
		return nil, err
	}
	if !launch.Resources.required() {
		return startLinuxCommand(launch, stdout, stderr, -1, "")
	}
	if adapter.CgroupRoot == "" || !filepath.IsAbs(adapter.CgroupRoot) {
		return nil, ErrResourceControlUnavailable
	}
	leaf, directory, err := adapter.prepare(launch.Resources)
	if err != nil {
		return nil, ErrResourceControlUnavailable
	}
	started, err := startLinuxCommand(launch, stdout, stderr, int(leaf.Fd()), directory)
	_ = leaf.Close()
	if err != nil {
		_ = cleanupLinuxCgroup(directory)
		return nil, err
	}
	return started, nil
}

func (adapter LinuxAdapter) prepare(limits ResourceLimits) (*os.File, string, error) {
	controllers, err := os.ReadFile(filepath.Join(adapter.CgroupRoot, "cgroup.controllers"))
	if err != nil {
		return nil, "", err
	}
	available := strings.Fields(string(controllers))
	if limits.CPURate != 0 && !containsString(available, "cpu") {
		return nil, "", errors.New("cpu controller is not delegated")
	}
	if limits.MemoryBytes != 0 && !containsString(available, "memory") {
		return nil, "", errors.New("memory controller is not delegated")
	}

	var entropy [12]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return nil, "", err
	}
	directory := filepath.Join(adapter.CgroupRoot, "flashgate-"+hex.EncodeToString(entropy[:]))
	if err := os.Mkdir(directory, 0o700); err != nil {
		return nil, "", err
	}
	fail := func(err error) (*os.File, string, error) {
		_ = os.Remove(directory)
		return nil, "", err
	}
	if limits.CPURate != 0 {
		quota := (uint64(limits.CPURate)*linuxCPUPeriodMicros + uint64(MaxCPURate) - 1) / uint64(MaxCPURate)
		// cgroup v2 accepts quotas of at least 1ms and periods up to 1s. A
		// smaller portable rate cannot be represented without weakening it.
		if quota < 1_000 {
			return fail(ErrInvalidResourceLimits)
		}
		if err := writeControl(directory, "cpu.max", fmt.Sprintf("%d %d", quota, linuxCPUPeriodMicros)); err != nil {
			return fail(err)
		}
	}
	if limits.MemoryBytes != 0 {
		if err := writeControl(directory, "memory.max", strconv.FormatInt(limits.MemoryBytes, 10)); err != nil {
			return fail(err)
		}
		if err := writeControl(directory, "memory.oom.group", "1"); err != nil {
			return fail(err)
		}
	}
	leaf, err := os.Open(directory)
	if err != nil {
		return fail(err)
	}
	return leaf, directory, nil
}

func startLinuxCommand(launch Launch, stdout, stderr io.Writer, cgroupFD int, cgroup string) (startedProcess, error) {
	command := exec.Command(launch.Executable, launch.Arguments...)
	command.Dir, command.Env = launch.WorkingDirectory, append([]string(nil), launch.Environment...)
	command.Stdout, command.Stderr = stdout, stderr
	if cgroupFD >= 0 {
		command.SysProcAttr = &syscall.SysProcAttr{UseCgroupFD: true, CgroupFD: cgroupFD}
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	return &linuxProcess{command: command, cgroup: cgroup}, nil
}

type linuxProcess struct {
	command *exec.Cmd
	cgroup  string
	once    sync.Once
}

func (process *linuxProcess) PID() int { return process.command.Process.Pid }
func (process *linuxProcess) Wait() error {
	err := process.command.Wait()
	process.cleanup()
	return err
}
func (process *linuxProcess) Kill() error {
	if process.cgroup == "" {
		return process.command.Process.Kill()
	}
	return writeControl(process.cgroup, "cgroup.kill", "1")
}
func (process *linuxProcess) cleanup() {
	process.once.Do(func() { _ = cleanupLinuxCgroup(process.cgroup) })
}

func cleanupLinuxCgroup(directory string) error {
	if directory == "" {
		return nil
	}
	_ = writeControl(directory, "cgroup.kill", "1")
	return os.Remove(directory)
}

func writeControl(directory, name, value string) error {
	return os.WriteFile(filepath.Join(directory, name), []byte(value), 0o600)
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
