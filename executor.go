package executor

import (
	"context"

	"github.com/justusschock/go-lifecycle/handoff"
)

type Executor interface {
	Run(context.Context, Command) (Result, error)
	Start(context.Context, Command) (Process, error)
}

type Process interface {
	Wait(context.Context) (Result, error)
	Signal(context.Context, string) error
	Kill(context.Context) error
	Export(context.Context) (handoff.Artifact, error)
}
