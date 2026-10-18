package executor

import (
	"context"
	"fmt"

	"github.com/justusschock/go-lifecycle/handoff"
)

// Starter is the command-starting port used by Remote. Implementations own
// transport, authentication and session semantics; they receive a validated
// command whose Args and Env they may keep.
type Starter interface {
	Start(context.Context, Command) (Process, error)
}

// Remote adapts a Starter to Executor. It owns and validates commands before
// forwarding them. Remote is safe for concurrent use when its Starter is.
type Remote struct {
	starter Starter
}

// NewRemote returns a Remote that forwards to starter. A nil or typed-nil
// starter returns an error wrapping ErrNilDependency.
func NewRemote(starter Starter) (*Remote, error) {
	if starter == nil || interfaceIsNil(starter) {
		return nil, fmt.Errorf("%w: starter", ErrNilDependency)
	}
	return &Remote{starter: starter}, nil
}

// Start clones and validates command and forwards it with ctx to the Starter
// exactly once. Validation errors wrap ErrInvalidCommand or ErrNilDependency;
// the Starter's process and error are returned unchanged.
func (remote *Remote) Start(ctx context.Context, command Command) (Process, error) {
	ownedCommand := command.Clone()
	if err := ownedCommand.Validate(); err != nil {
		return nil, err
	}
	return remote.starter.Start(ctx, ownedCommand)
}

// Run calls Start and then Wait on the returned process, both with ctx, and
// returns Wait's result and error unchanged.
func (remote *Remote) Run(ctx context.Context, command Command) (Result, error) {
	process, err := remote.Start(ctx, command)
	if err != nil {
		return Result{}, err
	}
	return process.Wait(ctx)
}

// Adopt is unsupported because a process reference does not transfer custody.
func (*Remote) Adopt(context.Context, handoff.Artifact) (Process, error) {
	return nil, ErrUnsupportedAdoption
}
