//go:build windows

package local

import (
	"fmt"
	"os"

	executor "github.com/justusschock/executor"
)

func signalByName(process *os.Process, name string) error {
	if _, err := signalNumber(name); err != nil {
		return err
	}
	return process.Signal(os.Kill)
}

// signalNumber accepts only KILL because Windows has no portable equivalent of
// the Unix termination signals.
func signalNumber(name string) (os.Signal, error) {
	if name != "KILL" {
		return nil, fmt.Errorf("%w: %q", executor.ErrInvalidSignal, name)
	}
	return os.Kill, nil
}
