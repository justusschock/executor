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
	// Usage reports resource consumption when the executor can observe it.
	// Zero fields mean the value is unknown, not that nothing was consumed.
	Usage Usage
}

// Usage is a best-effort resource accounting snapshot for one finished
// process. Unix-like local executions report the child's rusage, which also
// includes descendants the child waited for, such as programs run by a shell.
type Usage struct {
	// MaxRSSBytes is the peak resident set size in bytes.
	MaxRSSBytes int64
	// UserCPU and SystemCPU are the CPU time spent in user and kernel mode.
	UserCPU   time.Duration
	SystemCPU time.Duration
	// ReadBytes and WriteBytes approximate storage I/O. They are zero when the
	// platform does not report byte-granular block I/O.
	ReadBytes  int64
	WriteBytes int64
}

// ExitError reports a process that exited with a non-zero code or whose wait
// failed, for example because a configured output writer returned an error or
// the start context ended. Result holds the same snapshot the call returned.
// Cause is reachable through errors.Is and errors.As; for local executions it
// is the os/exec wait error, joined with the start context's cause when that
// context ended.
type ExitError struct {
	Result Result
	Cause  error
}

// Error reports the exit code and, when present, the cause.
func (e *ExitError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("executor: process exited with code %d", e.Result.ExitCode)
	}
	return fmt.Sprintf("executor: process exited with code %d: %v", e.Result.ExitCode, e.Cause)
}

// Unwrap returns Cause.
func (e *ExitError) Unwrap() error {
	return e.Cause
}
