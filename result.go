package executor

import (
	"fmt"
	"time"
)

type Result struct {
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time
	Stdout     []byte
	Stderr     []byte
}

type ExitError struct {
	Result Result
	Cause  error
}

func (e *ExitError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("executor: process exited with code %d", e.Result.ExitCode)
	}
	return fmt.Sprintf("executor: process exited with code %d: %v", e.Result.ExitCode, e.Cause)
}

func (e *ExitError) Unwrap() error {
	return e.Cause
}
