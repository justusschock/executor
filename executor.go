package executor

import (
	"context"

	"github.com/justusschock/go-lifecycle/handoff"
)

// Executor starts commands and waits for their results. Implementations must
// support concurrent method calls when used with RunAll. Run returns a final
// Result whose captured streams remain stable after the call returns.
type Executor interface {
	// Run starts the command and waits for its final Result. A non-zero exit
	// or wait failure returns the populated Result and an *ExitError.
	Run(context.Context, Command) (Result, error)
	// Start validates and starts the command and returns a handle to it. The
	// meaning of the context for the process lifetime is defined by the
	// implementation; for local.Executor it owns the child's lifetime.
	Start(context.Context, Command) (Process, error)
}

// Process controls and observes one started command. Every Wait call returns
// independently owned captured streams that remain stable after return.
type Process interface {
	// Wait blocks until the process has finished or the context ends.
	// Cancelling the context ends only this call, never the process, and Wait
	// may be called again.
	Wait(context.Context) (Result, error)
	// Signal sends a signal by exact uppercase name, such as "TERM". Names
	// the platform does not support return an error wrapping ErrInvalidSignal.
	Signal(context.Context, string) error
	// Kill unconditionally stops the process.
	Kill(context.Context) error
	// Export returns a reference to the live process without transferring
	// custody. It returns ErrNotStarted or ErrProcessExited when the process
	// is unavailable.
	Export(context.Context) (handoff.Artifact, error)
}
