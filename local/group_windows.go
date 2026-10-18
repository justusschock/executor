//go:build windows

package local

import (
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"

	executor "github.com/justusschock/executor"
)

const (
	processSetQuota  = 0x0100
	processTerminate = 0x0001
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject       = kernel32.NewProc("TerminateJobObject")
)

func configureCommand(cmd *exec.Cmd, command executor.Command) {
	attributes := &syscall.SysProcAttr{}
	if commandLine, ok := windowsShellCommandLine(command.Path, command.Args); ok {
		attributes.CmdLine = commandLine
	}
	if command.ProcessGroup {
		attributes.CreationFlags |= syscall.CREATE_NEW_PROCESS_GROUP
	}
	cmd.SysProcAttr = attributes
}

// processGroup kills the child's process tree through a Job Object. When the
// job cannot be created or assigned, it falls back to taskkill /T.
type processGroup struct {
	mu     sync.Mutex
	pid    int
	job    syscall.Handle
	closed bool
	exited <-chan struct{}
}

func newProcessGroup(process *os.Process, exited <-chan struct{}) *processGroup {
	return &processGroup{pid: process.Pid, job: assignJob(process.Pid), exited: exited}
}

func assignJob(pid int) syscall.Handle {
	job, _, _ := procCreateJobObjectW.Call(0, 0)
	if job == 0 {
		return 0
	}
	process, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(pid))
	if err != nil {
		_ = syscall.CloseHandle(syscall.Handle(job))
		return 0
	}
	defer func() { _ = syscall.CloseHandle(process) }()
	if assigned, _, _ := procAssignProcessToJobObject.Call(job, uintptr(process)); assigned == 0 {
		_ = syscall.CloseHandle(syscall.Handle(job))
		return 0
	}
	return syscall.Handle(job)
}

func (group *processGroup) signal(name string) error {
	if _, err := signalNumber(name); err != nil {
		return err
	}
	return group.kill()
}

func (group *processGroup) kill() error {
	select {
	case <-group.exited:
		return os.ErrProcessDone
	default:
	}
	group.mu.Lock()
	defer group.mu.Unlock()
	if group.closed {
		return os.ErrProcessDone
	}
	if group.job != 0 {
		if terminated, _, err := procTerminateJobObject.Call(uintptr(group.job), 1); terminated == 0 {
			return err
		}
		return nil
	}
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(group.pid)).Run()
}

func (group *processGroup) close() {
	group.mu.Lock()
	defer group.mu.Unlock()
	group.closed = true
	if group.job != 0 {
		_ = syscall.CloseHandle(group.job)
		group.job = 0
	}
}
