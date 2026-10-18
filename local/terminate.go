package local

import (
	"errors"
	"time"

	executor "github.com/justusschock/executor"
)

// escalation stops a process when its start context ends: it requests
// termination, waits for the grace period, and then kills unless the process
// was reaped first.
type escalation struct {
	grace     time.Duration
	terminate func() error
	kill      func() error
	exited    <-chan struct{}
}

func (stop escalation) run() error {
	if stop.grace <= 0 {
		return stop.kill()
	}
	if err := stop.terminate(); err != nil {
		if errors.Is(err, executor.ErrInvalidSignal) {
			// The platform has no terminate request; kill immediately.
			return stop.kill()
		}
		return err
	}
	go func() {
		timer := time.NewTimer(stop.grace)
		defer timer.Stop()
		select {
		case <-timer.C:
			_ = stop.kill()
		case <-stop.exited:
		}
	}()
	return nil
}
