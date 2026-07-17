package local

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"

	concurrency "github.com/justusschock/go-concurrency"
	executor "github.com/justusschock/go-executor"
)

var _ executor.Process = (*process)(nil)

type process struct {
	command   executor.Command
	cmd       *exec.Cmd
	startedAt time.Time
	stdout    *lockedBuffer
	stderr    *lockedBuffer
	wait      *concurrency.Future[executor.Result]
}

func newProcess(
	startContext context.Context,
	command executor.Command,
	cmd *exec.Cmd,
	startedAt time.Time,
	stdout *lockedBuffer,
	stderr *lockedBuffer,
) *process {
	processInstance := &process{
		command:   command,
		cmd:       cmd,
		startedAt: startedAt,
		stdout:    stdout,
		stderr:    stderr,
	}
	processInstance.wait = concurrency.Go(context.Background(), func(context.Context) (executor.Result, error) {
		waitErr := processInstance.cmd.Wait()
		result := processInstance.result(time.Now())
		if waitErr == nil {
			return result, nil
		}
		cause := waitErr
		if contextCause := context.Cause(startContext); contextCause != nil {
			cause = errors.Join(contextCause, waitErr)
		}
		return result, &executor.ExitError{Result: cloneResult(result), Cause: cause}
	})
	return processInstance
}

func (processInstance *process) result(finishedAt time.Time) executor.Result {
	exitCode := -1
	if processInstance.cmd.ProcessState != nil {
		exitCode = processInstance.cmd.ProcessState.ExitCode()
	}
	return executor.Result{
		ExitCode:   exitCode,
		StartedAt:  processInstance.startedAt,
		FinishedAt: finishedAt,
		Stdout:     processInstance.stdout.bytes(),
		Stderr:     processInstance.stderr.bytes(),
	}
}

func (processInstance *process) Wait(ctx context.Context) (executor.Result, error) {
	result, err := processInstance.wait.Await(ctx)
	result = cloneResult(result)
	if exit, ok := err.(*executor.ExitError); ok {
		clonedExit := *exit
		clonedExit.Result = cloneResult(exit.Result)
		return result, &clonedExit
	}
	return result, err
}

func (processInstance *process) Signal(ctx context.Context, name string) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	return signalByName(processInstance.cmd.Process, name)
}

func (processInstance *process) Kill(ctx context.Context) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	return processInstance.cmd.Process.Kill()
}

func cloneResult(result executor.Result) executor.Result {
	result.Stdout = bytes.Clone(result.Stdout)
	result.Stderr = bytes.Clone(result.Stderr)
	return result
}
