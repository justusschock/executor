package local

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"

	concurrency "github.com/justusschock/concurrency"
	executor "github.com/justusschock/executor"
)

var _ executor.Process = (*process)(nil)

type process struct {
	command   executor.Command
	cmd       *exec.Cmd
	startedAt time.Time
	stdout    *lockedBuffer
	stderr    *lockedBuffer
	// group is non-nil when command.ProcessGroup is set. It is assigned before
	// ready closes, and cancel waits for ready before reading it.
	group  *processGroup
	ready  chan struct{}
	exited chan struct{}
	wait   *concurrency.Future[executor.Result]
}

func newProcess(
	command executor.Command,
	cmd *exec.Cmd,
	stdout *lockedBuffer,
	stderr *lockedBuffer,
) *process {
	return &process{
		command: command,
		cmd:     cmd,
		stdout:  stdout,
		stderr:  stderr,
		ready:   make(chan struct{}),
		exited:  make(chan struct{}),
	}
}

// started records a successfully started child and begins reaping it.
func (processInstance *process) started(startContext context.Context, startedAt time.Time) {
	processInstance.startedAt = startedAt
	if processInstance.command.ProcessGroup {
		processInstance.group = newProcessGroup(processInstance.cmd.Process, processInstance.exited)
	}
	close(processInstance.ready)
	processInstance.wait = concurrency.Go(context.Background(), func(context.Context) (executor.Result, error) {
		waitErr := processInstance.cmd.Wait()
		finishedAt := time.Now()
		close(processInstance.exited)
		if processInstance.group != nil {
			processInstance.group.close()
		}
		result := processInstance.result(finishedAt)
		if waitErr == nil {
			return result, nil
		}
		cause := waitErr
		if contextCause := context.Cause(startContext); contextCause != nil {
			cause = errors.Join(contextCause, waitErr)
		}
		return result, &executor.ExitError{Result: cloneResult(result), Cause: cause}
	})
}

// abandon releases a cancel call blocked on a child that failed to start.
func (processInstance *process) abandon() {
	close(processInstance.ready)
	close(processInstance.exited)
}

// cancel is the exec.Cmd.Cancel hook invoked when the start context ends.
func (processInstance *process) cancel() error {
	<-processInstance.ready
	return escalation{
		grace:     processInstance.command.KillGrace,
		terminate: func() error { return processInstance.signal("TERM") },
		kill:      processInstance.kill,
		exited:    processInstance.exited,
	}.run()
}

func (processInstance *process) result(finishedAt time.Time) executor.Result {
	exitCode := -1
	if processInstance.cmd.ProcessState != nil {
		exitCode = processInstance.cmd.ProcessState.ExitCode()
	}
	stdout, stdoutTruncated := processInstance.stdout.snapshot()
	stderr, stderrTruncated := processInstance.stderr.snapshot()
	return executor.Result{
		ExitCode:        exitCode,
		StartedAt:       processInstance.startedAt,
		FinishedAt:      finishedAt,
		Stdout:          stdout,
		Stderr:          stderr,
		StdoutTruncated: stdoutTruncated,
		StderrTruncated: stderrTruncated,
		Usage:           usageFromState(processInstance.cmd.ProcessState),
	}
}

func (processInstance *process) Wait(ctx context.Context) (executor.Result, error) {
	result, err := processInstance.wait.Await(ctx)
	result = cloneResult(result)
	// Await returns the reaper's *ExitError unwrapped; only that exact value is
	// cloned, so a wait-context cause is never rewritten.
	if exit, ok := err.(*executor.ExitError); ok { //nolint:errorlint // exact identity, see above
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
	return processInstance.signal(name)
}

func (processInstance *process) Kill(ctx context.Context) error {
	if err := context.Cause(ctx); err != nil {
		return err
	}
	return processInstance.kill()
}

func (processInstance *process) signal(name string) error {
	if processInstance.group != nil {
		return processInstance.group.signal(name)
	}
	return signalByName(processInstance.cmd.Process, name)
}

func (processInstance *process) kill() error {
	if processInstance.group != nil {
		return processInstance.group.kill()
	}
	return processInstance.cmd.Process.Kill()
}

func cloneResult(result executor.Result) executor.Result {
	result.Stdout = bytes.Clone(result.Stdout)
	result.Stderr = bytes.Clone(result.Stderr)
	return result
}
