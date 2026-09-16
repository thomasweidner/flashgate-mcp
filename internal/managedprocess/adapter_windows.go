//go:build windows

package managedprocess

import (
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

const (
	createSuspended                = 0x00000004
	processSetQuota                = 0x0100
	processTerminate               = 0x0001
	processQueryLimitedInformation = 0x1000
	processSuspendResume           = 0x0800
	jobObjectClassExtendedLimit    = 9
	jobObjectClassCPURateControl   = 15
	jobObjectLimitJobMemory        = 0x00000200
	jobObjectLimitKillOnJobClose   = 0x00002000
	jobObjectCPURateEnable         = 0x00000001
	jobObjectCPURateHardCap        = 0x00000004
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	ntdll                    = syscall.NewLazyDLL("ntdll.dll")
	createJobObject          = kernel32.NewProc("CreateJobObjectW")
	setInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	assignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	isProcessInJob           = kernel32.NewProc("IsProcessInJob")
	terminateJobObject       = kernel32.NewProc("TerminateJobObject")
	ntResumeProcess          = ntdll.NewProc("NtResumeProcess")
)

type WindowsAdapter struct{}

func defaultProcessAdapter() ProcessAdapter { return WindowsAdapter{} }

func (WindowsAdapter) Start(launch Launch, stdout, stderr io.Writer) (startedProcess, error) {
	if err := launch.Resources.validate(); err != nil {
		return nil, err
	}
	if !launch.Resources.required() {
		return startWindowsCommand(launch, stdout, stderr)
	}
	if uint64(launch.Resources.MemoryBytes) > uint64(^uintptr(0)) {
		return nil, ErrInvalidResourceLimits
	}
	job, _, callErr := createJobObject.Call(0, 0)
	if job == 0 {
		return nil, callErr
	}
	closeJob := true
	defer func() {
		if closeJob {
			_ = syscall.CloseHandle(syscall.Handle(job))
		}
	}()

	extended := jobObjectExtendedLimitInformation{}
	extended.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	if launch.Resources.MemoryBytes != 0 {
		extended.BasicLimitInformation.LimitFlags |= jobObjectLimitJobMemory
		extended.JobMemoryLimit = uintptr(launch.Resources.MemoryBytes)
	}
	if err := setJobInformation(job, jobObjectClassExtendedLimit, unsafe.Pointer(&extended), unsafe.Sizeof(extended)); err != nil {
		return nil, err
	}
	if launch.Resources.CPURate != 0 {
		cpu := jobObjectCPURateControlInformation{
			ControlFlags: jobObjectCPURateEnable | jobObjectCPURateHardCap,
			CPURate:      launch.Resources.CPURate,
		}
		if err := setJobInformation(job, jobObjectClassCPURateControl, unsafe.Pointer(&cpu), unsafe.Sizeof(cpu)); err != nil {
			return nil, err
		}
	}

	command := newWindowsCommand(launch, stdout, stderr)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createSuspended}
	if err := command.Start(); err != nil {
		return nil, err
	}
	processHandle, err := syscall.OpenProcess(processSetQuota|processTerminate|processQueryLimitedInformation|processSuspendResume, false, uint32(command.Process.Pid))
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, err
	}
	fail := func(err error) (startedProcess, error) {
		_, _, _ = terminateJobObject.Call(job, 1)
		_ = command.Process.Kill()
		_ = syscall.CloseHandle(processHandle)
		_ = command.Wait()
		return nil, err
	}
	if result, _, err := assignProcessToJobObject.Call(job, uintptr(processHandle)); result == 0 {
		return fail(err)
	}
	var contained int32
	if result, _, err := isProcessInJob.Call(uintptr(processHandle), job, uintptr(unsafe.Pointer(&contained))); result == 0 || contained == 0 {
		return fail(err)
	}
	if status, _, _ := ntResumeProcess.Call(uintptr(processHandle)); status != 0 {
		return fail(fmt.Errorf("resume contained process: NTSTATUS 0x%x", status))
	}
	_ = syscall.CloseHandle(processHandle)
	closeJob = false
	return &windowsJobProcess{command: command, job: syscall.Handle(job)}, nil
}

func startWindowsCommand(launch Launch, stdout, stderr io.Writer) (startedProcess, error) {
	command := newWindowsCommand(launch, stdout, stderr)
	if err := command.Start(); err != nil {
		return nil, err
	}
	return osProcess{command}, nil
}

func newWindowsCommand(launch Launch, stdout, stderr io.Writer) *exec.Cmd {
	command := exec.Command(launch.Executable, launch.Arguments...)
	command.Dir, command.Env = launch.WorkingDirectory, append([]string(nil), launch.Environment...)
	command.Stdout, command.Stderr = stdout, stderr
	return command
}

type windowsJobProcess struct {
	command *exec.Cmd
	job     syscall.Handle
	once    sync.Once
}

func (process *windowsJobProcess) PID() int { return process.command.Process.Pid }
func (process *windowsJobProcess) Wait() error {
	err := process.command.Wait()
	process.close()
	return err
}
func (process *windowsJobProcess) Kill() error {
	result, _, err := terminateJobObject.Call(uintptr(process.job), 1)
	if result == 0 {
		return err
	}
	return nil
}
func (process *windowsJobProcess) close() {
	process.once.Do(func() { _ = syscall.CloseHandle(process.job) })
}

func setJobInformation(job uintptr, class uintptr, information unsafe.Pointer, size uintptr) error {
	result, _, err := setInformationJobObject.Call(job, class, uintptr(information), size)
	if result == 0 {
		return err
	}
	return nil
}

type jobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInformation struct {
	BasicLimitInformation jobObjectBasicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type jobObjectCPURateControlInformation struct {
	ControlFlags uint32
	CPURate      uint32
}
