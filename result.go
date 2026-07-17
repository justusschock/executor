package executor

import (
	"fmt"
	"time"
)

// Result is a final execution snapshot. Stdout and Stderr are owned by the
// caller; executor implementations must not retain and later mutate them.
// Truncation flags are final for the corresponding captured streams.
type Result struct {
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time
	Stdout     []byte
	Stderr     []byte
	// StdoutTruncated and StderrTruncated report output omitted by capture limits.
	StdoutTruncated bool
	StderrTruncated bool
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
