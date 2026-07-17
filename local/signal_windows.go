//go:build windows

package local

import (
	"fmt"
	"os"

	executor "github.com/justusschock/go-executor"
)

func signalByName(process *os.Process, name string) error {
	if name != "KILL" {
		return fmt.Errorf("%w: %q", executor.ErrInvalidSignal, name)
	}
	return process.Signal(os.Kill)
}
