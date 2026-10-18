package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/justusschock/go-lifecycle/handoff"
)

// FR-EXEC-06
func TestExitErrorPreservesCauseAndResult(t *testing.T) {
	want := errors.New("wait failed")
	err := &ExitError{Result: Result{ExitCode: 7}, Cause: want}

	if !errors.Is(err, want) {
		t.Fatalf("errors.Is(%v, cause) = false", err)
	}
	if err.Error() != "executor: process exited with code 7: wait failed" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if err.Result.ExitCode != 7 {
		t.Fatalf("Result.ExitCode = %d, want 7", err.Result.ExitCode)
	}
}

// FR-EXEC-06
func TestExitErrorWithoutCause(t *testing.T) {
	err := &ExitError{Result: Result{ExitCode: 9}}

	if err.Error() != "executor: process exited with code 9" {
		t.Fatalf("Error() = %q", err.Error())
	}
	if err.Unwrap() != nil {
		t.Fatalf("Unwrap() = %v, want nil", err.Unwrap())
	}
}

// FR-EXEC-06
func TestStableErrorsExist(t *testing.T) {
	errorsByName := map[string]error{
		"ErrInvalidCommand":      ErrInvalidCommand,
		"ErrInvalidSignal":       ErrInvalidSignal,
		"ErrNotStarted":          ErrNotStarted,
		"ErrProcessExited":       ErrProcessExited,
		"ErrNilDependency":       ErrNilDependency,
		"ErrUnsupportedAdoption": ErrUnsupportedAdoption,
	}
	for name, err := range errorsByName {
		if err == nil {
			t.Fatalf("%s is nil", name)
		}
	}
}

type contractExecutor struct{}

func (contractExecutor) Run(context.Context, Command) (Result, error) {
	return Result{}, nil
}

func (contractExecutor) Start(context.Context, Command) (Process, error) {
	return contractProcess{}, nil
}

type contractProcess struct{}

func (contractProcess) Wait(context.Context) (Result, error) {
	return Result{}, nil
}

func (contractProcess) Signal(context.Context, string) error {
	return nil
}

func (contractProcess) Kill(context.Context) error {
	return nil
}

func (contractProcess) Export(context.Context) (handoff.Artifact, error) {
	return handoff.Artifact{}, nil
}

var _ Executor = contractExecutor{}
var _ Process = contractProcess{}
