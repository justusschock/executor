//go:build aix || android || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package local

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	executor "github.com/justusschock/executor"
)

func configureCommand(cmd *exec.Cmd, command executor.Command) {
	if command.ProcessGroup {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
}

// processGroup signals the process group led by the started child. Its ID
// stays reserved while any member is alive, so signals reach grandchildren
// even after the leader has exited.
type processGroup struct {
	id     int
	exited <-chan struct{}
}

func newProcessGroup(process *os.Process, exited <-chan struct{}) *processGroup {
	return &processGroup{id: process.Pid, exited: exited}
}

func (group *processGroup) signal(name string) error {
	signal, err := signalNumber(name)
	if err != nil {
		return err
	}
	return group.send(signal)
}

func (group *processGroup) kill() error {
	return group.send(syscall.SIGKILL)
}

func (group *processGroup) send(signal syscall.Signal) error {
	select {
	case <-group.exited:
		return os.ErrProcessDone
	default:
	}
	if err := syscall.Kill(-group.id, signal); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	return nil
}

func (*processGroup) close() {}
