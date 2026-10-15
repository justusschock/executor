//go:build aix || android || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package local

import (
	"fmt"
	"os"
	"syscall"

	executor "github.com/justusschock/go-executor"
)

func signalByName(process *os.Process, name string) error {
	signal, err := signalNumber(name)
	if err != nil {
		return err
	}
	return process.Signal(signal)
}

func signalNumber(name string) (syscall.Signal, error) {
	switch name {
	case "TERM":
		return syscall.SIGTERM, nil
	case "KILL":
		return syscall.SIGKILL, nil
	case "INT":
		return syscall.SIGINT, nil
	case "HUP":
		return syscall.SIGHUP, nil
	default:
		return 0, fmt.Errorf("%w: %q", executor.ErrInvalidSignal, name)
	}
}
