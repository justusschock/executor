package executor

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/justusschock/concurrency"
)

type runAllExecutor struct {
	run func(context.Context, Command) (Result, error)
}

type runAllOutcome struct {
	results []Result
	err     error
}

func (e *runAllExecutor) Run(ctx context.Context, command Command) (Result, error) {
	return e.run(ctx, command)
}

func (*runAllExecutor) Start(context.Context, Command) (Process, error) {
	panic("unexpected Start call")
}

// FR-EXEC-14
func TestRunAllPreservesOrderAndBoundsConcurrency(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		started := make(chan string, 3)
		releases := map[string]chan struct{}{
			"0": make(chan struct{}),
			"1": make(chan struct{}),
			"2": make(chan struct{}),
		}
		var active atomic.Int64
		var peak atomic.Int64
		runner := &runAllExecutor{run: func(ctx context.Context, command Command) (Result, error) {
			current := active.Add(1)
			defer active.Add(-1)
			for observed := peak.Load(); current > observed && !peak.CompareAndSwap(observed, current); observed = peak.Load() {
			}
			started <- command.Path
			select {
			case <-releases[command.Path]:
			case <-ctx.Done():
				return Result{}, context.Cause(ctx)
			}
			exitCode, err := strconv.Atoi(command.Path)
			if err != nil {
				return Result{}, err
			}
			return Result{ExitCode: exitCode}, nil
		}}

		finished := make(chan runAllOutcome, 1)
		go func() {
			results, err := RunAll(
				context.Background(),
				runner,
				[]Command{{Path: "0"}, {Path: "1"}, {Path: "2"}},
				2,
			)
			finished <- runAllOutcome{results: results, err: err}
		}()

		synctest.Wait()
		if got := len(started); got != 2 {
			t.Fatalf("started commands before any release = %d, want 2", got)
		}
		first := receiveString(t, started)
		second := receiveString(t, started)
		if got := map[string]bool{first: true, second: true}; !reflect.DeepEqual(got, map[string]bool{"0": true, "1": true}) {
			t.Fatalf("first started commands = %q and %q, want commands 0 and 1", first, second)
		}
		close(releases["1"])
		if got := receiveString(t, started); got != "2" {
			t.Fatalf("third started command = %q, want 2", got)
		}
		close(releases["2"])
		close(releases["0"])

		got := receiveRunAllOutcome(t, finished)
		if got.err != nil {
			t.Fatal(got.err)
		}
		exitCodes := []int{got.results[0].ExitCode, got.results[1].ExitCode, got.results[2].ExitCode}
		if want := []int{0, 1, 2}; !reflect.DeepEqual(exitCodes, want) {
			t.Fatalf("exit codes = %v, want %v", exitCodes, want)
		}
		if got := peak.Load(); got != 2 {
			t.Fatalf("peak concurrency = %d, want 2", got)
		}
	})
}

// FR-EXEC-14
func TestRunAllReturnsIndexedErrorWithoutPartialResults(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		want := errors.New("command failed")
		runner := &runAllExecutor{run: func(_ context.Context, command Command) (Result, error) {
			if command.Path == "bad" {
				return Result{ExitCode: 7}, want
			}
			return Result{ExitCode: 0}, nil
		}}

		results, err := RunAll(context.Background(), runner, []Command{{Path: "ok"}, {Path: "bad"}}, 1)
		if results != nil {
			t.Fatalf("results = %#v, want nil", results)
		}
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want errors.Is(_, %v)", err, want)
		}
		if got, want := err.Error(), "command 1: command failed"; got != want {
			t.Fatalf("error = %q, want %q", got, want)
		}
	})
}

// FR-EXEC-14
func TestRunAllCancelsPeersStopsAdmissionAndJoinsIndexedErrors(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		firstErr := errors.New("first failed")
		secondErr := errors.New("peer cleanup failed")
		bothStarted := make(chan struct{})
		siblingCause := make(chan error, 1)
		var started atomic.Int64
		runner := &runAllExecutor{run: func(ctx context.Context, command Command) (Result, error) {
			if started.Add(1) == 2 {
				close(bothStarted)
			}
			switch command.Path {
			case "first":
				select {
				case <-bothStarted:
				case <-ctx.Done():
					return Result{}, context.Cause(ctx)
				}
				return Result{}, firstErr
			case "peer":
				<-ctx.Done()
				cause := context.Cause(ctx)
				siblingCause <- cause
				return Result{}, errors.Join(cause, secondErr)
			default:
				return Result{}, errors.New("unadmitted command started")
			}
		}}

		results, err := RunAll(
			context.Background(),
			runner,
			[]Command{{Path: "first"}, {Path: "peer"}, {Path: "must-not-start"}},
			2,
		)
		if results != nil {
			t.Fatalf("results = %#v, want nil", results)
		}
		if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
			t.Fatalf("error = %v, want both command failures", err)
		}
		if got := receiveError(t, siblingCause); !errors.Is(got, firstErr) {
			t.Fatalf("peer cancellation cause = %v, want first command failure", got)
		}
		if got := started.Load(); got != 2 {
			t.Fatalf("started commands = %d, want 2", got)
		}
		if text := err.Error(); !strings.Contains(text, "command 0:") || !strings.Contains(text, "command 1:") {
			t.Fatalf("joined error lacks indexed failures: %q", text)
		}
	})
}

// FR-EXEC-14
func TestRunAllOwnsCommandsBeforeExecution(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		observed := make(chan Command, 2)
		runner := &runAllExecutor{run: func(_ context.Context, command Command) (Result, error) {
			observed <- command
			if command.Path == "first" {
				close(started)
				<-release
			}
			return Result{}, nil
		}}
		commands := []Command{
			{Path: "first"},
			{Path: "second", Args: []string{"original"}, Env: map[string]string{"KEY": "original"}},
		}
		finished := make(chan error, 1)
		go func() {
			_, err := RunAll(context.Background(), runner, commands, 1)
			finished <- err
		}()

		synctest.Wait()
		select {
		case <-started:
		default:
			t.Fatal("first command did not start")
		}
		commands[1].Path = "mutated"
		commands[1].Args[0] = "mutated"
		commands[1].Env["KEY"] = "mutated"
		close(release)

		if err := receiveError(t, finished); err != nil {
			t.Fatal(err)
		}
		<-observed
		second := <-observed
		if second.Path != "second" || !reflect.DeepEqual(second.Args, []string{"original"}) || !reflect.DeepEqual(second.Env, map[string]string{"KEY": "original"}) {
			t.Fatalf("second command = %#v, want owned original", second)
		}
	})
}

// FR-EXEC-14
func TestRunAllRejectsInvalidDependenciesAndLimits(t *testing.T) {
	t.Parallel()

	valid := &runAllExecutor{run: func(context.Context, Command) (Result, error) { return Result{}, nil }}
	var typedNil *runAllExecutor
	for name, runner := range map[string]Executor{"nil": nil, "typed nil": typedNil} {
		t.Run(name, func(t *testing.T) {
			results, err := RunAll(context.Background(), runner, nil, 1)
			if results != nil || !errors.Is(err, ErrNilDependency) {
				t.Fatalf("RunAll() = (%#v, %v), want (nil, %v)", results, err, ErrNilDependency)
			}
		})
	}
	for _, limit := range []int{0, -1} {
		results, err := RunAll(context.Background(), valid, nil, limit)
		if results != nil || !errors.Is(err, concurrency.ErrInvalidLimit) {
			t.Fatalf("RunAll(limit=%d) = (%#v, %v), want (nil, %v)", limit, results, err, concurrency.ErrInvalidLimit)
		}
	}
}

// FR-EXEC-14
func TestRunAllPropagatesCancellation(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		started := make(chan struct{})
		runner := &runAllExecutor{run: func(ctx context.Context, _ Command) (Result, error) {
			close(started)
			<-ctx.Done()
			return Result{}, context.Cause(ctx)
		}}
		finished := make(chan error, 1)
		go func() {
			_, err := RunAll(ctx, runner, []Command{{Path: "blocked"}}, 1)
			finished <- err
		}()
		synctest.Wait()
		select {
		case <-started:
		default:
			t.Fatal("command did not start")
		}
		cancel()
		if err := receiveError(t, finished); !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
	})
}

// The receive helpers run inside a synctest bubble: synctest.Wait returns once
// every other goroutine is durably blocked, so a missing value is a definite
// failure rather than a slow scheduler.

func receiveString(t *testing.T, values <-chan string) string {
	t.Helper()
	synctest.Wait()
	select {
	case value := <-values:
		return value
	default:
		t.Fatal("no string was sent")
		return ""
	}
}

func receiveError(t *testing.T, values <-chan error) error {
	t.Helper()
	synctest.Wait()
	select {
	case err := <-values:
		return err
	default:
		t.Fatal("no error was sent")
		return nil
	}
}

func receiveRunAllOutcome(t *testing.T, values <-chan runAllOutcome) runAllOutcome {
	t.Helper()
	synctest.Wait()
	select {
	case outcome := <-values:
		return outcome
	default:
		t.Fatal("RunAll did not return")
		return runAllOutcome{}
	}
}
