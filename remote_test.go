package executor

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/justusschock/go-lifecycle/handoff"
)

func TestNewRemoteRejectsNilStarter(t *testing.T) {
	var typedNil *remoteStarter
	tests := []struct {
		name    string
		starter Starter
	}{
		{name: "nil"},
		{name: "typed nil", starter: typedNil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			remote, err := NewRemote(test.starter)
			if !errors.Is(err, ErrNilDependency) {
				t.Fatalf("NewRemote() error = %v, want %v", err, ErrNilDependency)
			}
			if remote != nil {
				t.Fatalf("NewRemote() remote = %#v, want nil", remote)
			}
		})
	}
}

func TestRemoteStartValidatesBeforeForwarding(t *testing.T) {
	calls := 0
	remote, err := NewRemote(&remoteStarter{start: func(context.Context, Command) (Process, error) {
		calls++
		return &remoteProcess{}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}

	process, err := remote.Start(context.Background(), Command{})
	if !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("Start() error = %v, want %v", err, ErrInvalidCommand)
	}
	if process != nil {
		t.Fatalf("Start() process = %#v, want nil", process)
	}
	if calls != 0 {
		t.Fatalf("starter calls = %d, want 0", calls)
	}
}

func TestRemoteStartForwardsOwnedCommandExactlyOnce(t *testing.T) {
	stdin := bytes.NewBufferString("input")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command := Command{
		Path:           "tool",
		Args:           []string{"original-argument"},
		Dir:            "/work",
		Env:            map[string]string{"KEY": "original-environment"},
		ClearEnv:       true,
		Stdin:          stdin,
		Stdout:         stdout,
		Stderr:         stderr,
		MaxStdoutBytes: 10,
		MaxStderrBytes: 20,
	}
	wantProcess := &remoteProcess{}
	wantErr := errors.New("starter result")
	ctx := context.WithValue(context.Background(), remoteContextKey{}, "value")

	calls := 0
	var forwardedContext context.Context
	var forwarded Command
	remote, err := NewRemote(&remoteStarter{start: func(gotContext context.Context, gotCommand Command) (Process, error) {
		calls++
		forwardedContext = gotContext
		forwarded = gotCommand
		forwarded.Args[0] = "starter-mutation"
		forwarded.Env["KEY"] = "starter-mutation"
		return wantProcess, wantErr
	}})
	if err != nil {
		t.Fatal(err)
	}

	process, err := remote.Start(ctx, command)
	if process != wantProcess || err != wantErr { //nolint:errorlint // Remote must forward the exact error value, not a wrapper.
		t.Fatalf("Start() = (%#v, %v), want (%#v, %v)", process, err, wantProcess, wantErr)
	}
	if calls != 1 {
		t.Fatalf("starter calls = %d, want 1", calls)
	}
	if forwardedContext != ctx {
		t.Fatal("Start() did not forward the exact context")
	}
	if command.Args[0] != "original-argument" || command.Env["KEY"] != "original-environment" {
		t.Fatalf("starter mutated caller command: %#v", command)
	}
	if forwarded.Stdin != stdin || forwarded.Stdout != stdout || forwarded.Stderr != stderr {
		t.Fatal("Start() did not preserve stream references")
	}
	if !forwarded.ClearEnv {
		t.Fatal("Start() did not preserve ClearEnv")
	}
	if forwarded.MaxStdoutBytes != 10 || forwarded.MaxStderrBytes != 20 {
		t.Fatalf("Start() did not preserve capture limits: %#v", forwarded)
	}

	command.Args[0] = "caller-mutation"
	command.Env["KEY"] = "caller-mutation"
	if forwarded.Args[0] != "starter-mutation" || forwarded.Env["KEY"] != "starter-mutation" {
		t.Fatalf("caller retained mutable command ownership: %#v", forwarded)
	}
}

func TestRemoteRunStartsAndWaitsExactlyOnce(t *testing.T) {
	wantResult := Result{
		ExitCode:        7,
		Stdout:          []byte("out"),
		Stderr:          []byte("err"),
		StdoutTruncated: true,
		StderrTruncated: true,
	}
	wantErr := errors.New("wait result")
	ctx := context.WithValue(context.Background(), remoteContextKey{}, "value")

	startCalls := 0
	waitCalls := 0
	var startContext context.Context
	var waitContext context.Context
	process := &remoteProcess{wait: func(gotContext context.Context) (Result, error) {
		waitCalls++
		waitContext = gotContext
		return wantResult, wantErr
	}}
	remote, err := NewRemote(&remoteStarter{start: func(gotContext context.Context, command Command) (Process, error) {
		startCalls++
		startContext = gotContext
		if command.Path != "tool" {
			t.Fatalf("forwarded command = %#v", command)
		}
		return process, nil
	}})
	if err != nil {
		t.Fatal(err)
	}

	result, err := remote.Run(ctx, Command{Path: "tool"})
	if err != wantErr { //nolint:errorlint // Remote must forward the exact error value, not a wrapper.
		t.Fatalf("Run() error = %v, want exact error %v", err, wantErr)
	}
	if !reflect.DeepEqual(result, wantResult) {
		t.Fatalf("Run() result = %#v, want %#v", result, wantResult)
	}
	if &result.Stdout[0] != &wantResult.Stdout[0] || &result.Stderr[0] != &wantResult.Stderr[0] {
		t.Fatal("Run() changed the exact result returned by Wait()")
	}
	if startCalls != 1 || waitCalls != 1 {
		t.Fatalf("calls = start %d, wait %d; want 1/1", startCalls, waitCalls)
	}
	if startContext != ctx || waitContext != ctx {
		t.Fatal("Run() did not pass its exact context to Start() and Wait()")
	}
}

func TestRemoteRunReturnsStartErrorWithoutWaiting(t *testing.T) {
	wantErr := errors.New("start failed")
	waitCalls := 0
	process := &remoteProcess{wait: func(context.Context) (Result, error) {
		waitCalls++
		return Result{ExitCode: 9}, nil
	}}
	startCalls := 0
	remote, err := NewRemote(&remoteStarter{start: func(context.Context, Command) (Process, error) {
		startCalls++
		return process, wantErr
	}})
	if err != nil {
		t.Fatal(err)
	}

	result, err := remote.Run(context.Background(), Command{Path: "tool"})
	if err != wantErr { //nolint:errorlint // Remote must forward the exact error value, not a wrapper.
		t.Fatalf("Run() error = %v, want exact error %v", err, wantErr)
	}
	if !reflect.DeepEqual(result, Result{}) {
		t.Fatalf("Run() result = %#v, want zero", result)
	}
	if startCalls != 1 || waitCalls != 0 {
		t.Fatalf("calls = start %d, wait %d; want 1/0", startCalls, waitCalls)
	}
}

func TestRemoteAdoptionIsUnsupported(t *testing.T) {
	remote, err := NewRemote(&remoteStarter{start: func(context.Context, Command) (Process, error) {
		return &remoteProcess{}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}

	process, err := remote.Adopt(context.Background(), handoff.Artifact{})
	if err != ErrUnsupportedAdoption { //nolint:errorlint // Adopt returns the sentinel itself, not a wrapper.
		t.Fatalf("Adopt() error = %v, want exact error %v", err, ErrUnsupportedAdoption)
	}
	if process != nil {
		t.Fatalf("Adopt() process = %#v, want nil", process)
	}
}

type remoteContextKey struct{}

type remoteStarter struct {
	start func(context.Context, Command) (Process, error)
}

func (starter *remoteStarter) Start(ctx context.Context, command Command) (Process, error) {
	return starter.start(ctx, command)
}

type remoteProcess struct {
	wait func(context.Context) (Result, error)
}

func (process *remoteProcess) Wait(ctx context.Context) (Result, error) {
	return process.wait(ctx)
}

func (*remoteProcess) Signal(context.Context, string) error {
	return nil
}

func (*remoteProcess) Kill(context.Context) error {
	return nil
}

func (*remoteProcess) Export(context.Context) (handoff.Artifact, error) {
	return handoff.Artifact{}, ErrUnsupportedAdoption
}

var _ Executor = (*Remote)(nil)
var _ handoff.Adopter[Process] = (*Remote)(nil)
