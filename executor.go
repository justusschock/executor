package executor

import (
	"context"

	"github.com/justusschock/go-lifecycle/handoff"
)

// Executor starts commands and waits for their results. Implementations must
// support concurrent method calls when used with RunAll. Run returns a final
// Result whose captured streams remain stable after the call returns.
type Executor interface {
	Run(context.Context, Command) (Result, error)
	Start(context.Context, Command) (Process, error)
}

// Process controls and observes one started command. Every Wait call returns
// independently owned captured streams that remain stable after return.
type Process interface {
	Wait(context.Context) (Result, error)
	Signal(context.Context, string) error
	Kill(context.Context) error
	Export(context.Context) (handoff.Artifact, error)
}
