//go:build aix || android || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package local

import (
	"fmt"
	"os"
	"syscall"

	executor "github.com/justusschock/go-executor"
)

func signalByName(process *os.Process, name string) error {
	var signal syscall.Signal
	switch name {
	case "TERM":
		signal = syscall.SIGTERM
	case "KILL":
		signal = syscall.SIGKILL
	case "INT":
		signal = syscall.SIGINT
	case "HUP":
		signal = syscall.SIGHUP
	default:
		return fmt.Errorf("%w: %q", executor.ErrInvalidSignal, name)
	}
	return process.Signal(signal)
}
