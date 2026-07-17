package executor

import (
	"context"
	"fmt"

	"github.com/justusschock/go-lifecycle/handoff"
)

// Starter is the command-starting port used by Remote.
type Starter interface {
	Start(context.Context, Command) (Process, error)
}

// Remote owns and validates commands before forwarding them to a Starter.
type Remote struct {
	starter Starter
}

func NewRemote(starter Starter) (*Remote, error) {
	if starter == nil || interfaceIsNil(starter) {
		return nil, fmt.Errorf("%w: starter", ErrNilDependency)
	}
	return &Remote{starter: starter}, nil
}

func (remote *Remote) Start(ctx context.Context, command Command) (Process, error) {
	ownedCommand := command.Clone()
	if err := ownedCommand.Validate(); err != nil {
		return nil, err
	}
	return remote.starter.Start(ctx, ownedCommand)
}

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
