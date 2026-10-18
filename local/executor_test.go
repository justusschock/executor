package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	executor "github.com/justusschock/executor"
)

const processTestTimeout = 5 * time.Second

func TestExecutorHelper(t *testing.T) {
	if os.Getenv("GO_WANT_EXECUTOR_HELPER") != "1" {
		return
	}

	switch os.Getenv("EXECUTOR_HELPER_MODE") {
	case "success":
		_, _ = fmt.Fprintln(os.Stdout, "out")
		_, _ = fmt.Fprintln(os.Stderr, "err")
	case "exit7":
		_, _ = fmt.Fprintln(os.Stdout, "out7")
		_, _ = fmt.Fprintln(os.Stderr, "err7")
		os.Exit(7)
	case "delay":
		time.Sleep(150 * time.Millisecond)
		_, _ = fmt.Fprintln(os.Stdout, "delayed")
	case "block":
		for {
			time.Sleep(time.Hour)
		}
	case "stdin":
		if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
			os.Exit(8)
		}
	case "cwd":
		directory, err := os.Getwd()
		if err != nil {
			os.Exit(8)
		}
		_, _ = fmt.Fprint(os.Stdout, directory)
	case "inspect":
		_, _ = fmt.Fprintf(os.Stdout, "%s/%s", os.Args[len(os.Args)-1], os.Getenv("OWNED_VALUE"))
	case "environment":
		_, _ = fmt.Fprintf(os.Stdout, "%s/%s", os.Getenv("EXECUTOR_INHERITED_VALUE"), os.Getenv("EXECUTOR_EXPLICIT_VALUE"))
	case "flood":
		var writers sync.WaitGroup
		writers.Add(2)
		go func() {
			defer writers.Done()
			for range 128 {
				_, _ = fmt.Fprintln(os.Stdout, strings.Repeat("o", 64))
			}
		}()
		go func() {
			defer writers.Done()
			for range 128 {
				_, _ = fmt.Fprintln(os.Stderr, strings.Repeat("e", 64))
			}
		}()
		writers.Wait()
	case "usage", "usage-exit":
		consumeResources()
		if os.Getenv("EXECUTOR_HELPER_MODE") == "usage-exit" {
			os.Exit(3)
		}
	default:
		os.Exit(9)
	}
	os.Exit(0)
}

const usageTestBytes = 64 << 20

var usageSink []byte

// consumeResources touches usageTestBytes of memory and spins the CPU so that
// rusage reports clearly nonzero values.
func consumeResources() {
	usageSink = make([]byte, usageTestBytes)
	for index := 0; index < len(usageSink); index += 4096 {
		usageSink[index] = 1
	}
	deadline := time.Now().Add(100 * time.Millisecond)
	for counter := 0; time.Now().Before(deadline); counter++ {
		usageSink[counter%len(usageSink)]++
	}
}

func TestExecutorEmptyEnvironmentHelper(t *testing.T) {
	if len(os.Args) == 0 || os.Args[len(os.Args)-1] != "executor-empty-environment-helper" {
		return
	}

	environment := os.Environ()
	hasSystemRoot := false
	for _, entry := range environment {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "SYSTEMROOT") {
			hasSystemRoot = true
		}
	}
	_, _ = fmt.Fprintf(os.Stdout, "%d/%t", len(environment), hasSystemRoot)
	os.Exit(0)
}

// FR-EXEC-03
func TestRunStreamsAndCaptures(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result, err := New().Run(context.Background(), helperCommand(t, "success", &stdout, &stderr))
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || stdout.String() != "out\n" || stderr.String() != "err\n" || string(result.Stdout) != "out\n" || string(result.Stderr) != "err\n" {
		t.Fatalf("result=%+v streams=%q/%q", result, stdout.String(), stderr.String())
	}
	if result.StartedAt.IsZero() || result.FinishedAt.Before(result.StartedAt) {
		t.Fatalf("timestamps started=%v finished=%v", result.StartedAt, result.FinishedAt)
	}

	stdout.WriteString("caller mutation")
	if string(result.Stdout) != "out\n" {
		t.Fatalf("captured stdout changed with caller writer: %q", result.Stdout)
	}
}

// FR-EXEC-05
func TestRunBoundsCaptureWithoutTruncatingDestinations(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := helperCommand(t, "success", &stdout, &stderr)
	command.MaxStdoutBytes = 2
	command.MaxStderrBytes = 1

	result, err := New().Run(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(result.Stdout), "ou"; got != want {
		t.Fatalf("captured stdout = %q, want %q", got, want)
	}
	if got, want := string(result.Stderr), "e"; got != want {
		t.Fatalf("captured stderr = %q, want %q", got, want)
	}
	if !result.StdoutTruncated || !result.StderrTruncated {
		t.Fatalf("truncation flags = %t/%t, want true/true", result.StdoutTruncated, result.StderrTruncated)
	}
	if got, want := stdout.String(), "out\n"; got != want {
		t.Fatalf("streamed stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "err\n"; got != want {
		t.Fatalf("streamed stderr = %q, want %q", got, want)
	}
}

// FR-EXEC-05
func TestRunDoesNotReportExactLimitAsTruncated(t *testing.T) {
	t.Parallel()

	command := helperCommand(t, "success", nil, nil)
	command.MaxStdoutBytes = 4
	command.MaxStderrBytes = 4
	result, err := New().Run(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Stdout) != "out\n" || string(result.Stderr) != "err\n" {
		t.Fatalf("captured output = %q/%q", result.Stdout, result.Stderr)
	}
	if result.StdoutTruncated || result.StderrTruncated {
		t.Fatalf("truncation flags = %t/%t, want false/false", result.StdoutTruncated, result.StderrTruncated)
	}
}

// FR-EXEC-06
func TestRunReturnsPopulatedExitError(t *testing.T) {
	t.Parallel()

	result, err := New().Run(context.Background(), helperCommand(t, "exit7", nil, nil))
	var exit *executor.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("Run() error = %v, want *executor.ExitError", err)
	}
	if result.ExitCode != 7 || string(result.Stdout) != "out7\n" || string(result.Stderr) != "err7\n" {
		t.Fatalf("Run() result = %+v", result)
	}
	if !reflect.DeepEqual(exit.Result, result) {
		t.Fatalf("ExitError.Result = %+v, want %+v", exit.Result, result)
	}
	if exit.Cause == nil {
		t.Fatal("ExitError.Cause is nil for nonzero exit")
	}
}

// FR-EXEC-12
func TestRunReportsResourceUsage(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"usage", "usage-exit"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			result, err := New().Run(context.Background(), helperCommand(t, mode, nil, nil))
			if mode == "usage-exit" {
				var exit *executor.ExitError
				if !errors.As(err, &exit) || result.ExitCode != 3 {
					t.Fatalf("Run() = (%+v, %v), want exit 3", result, err)
				}
				if exit.Result.Usage != result.Usage {
					t.Fatalf("ExitError usage = %+v, want %+v", exit.Result.Usage, result.Usage)
				}
			} else if err != nil {
				t.Fatal(err)
			}

			usage := result.Usage
			if usage.UserCPU+usage.SystemCPU <= 0 {
				t.Fatalf("CPU usage = %+v, want positive", usage)
			}
			if usage.ReadBytes < 0 || usage.WriteBytes < 0 {
				t.Fatalf("I/O usage = %+v, want non-negative", usage)
			}
			if runtime.GOOS == "windows" {
				return
			}
			// Bounds also detect a missing or doubled kilobyte normalization.
			if usage.MaxRSSBytes < usageTestBytes || usage.MaxRSSBytes > 1<<40 {
				t.Fatalf("MaxRSSBytes = %d, want at least %d bytes", usage.MaxRSSBytes, usageTestBytes)
			}
		})
	}
}

// FR-EXEC-01, FR-EXEC-02
func TestStartValidatesAndOwnsCommand(t *testing.T) {
	t.Parallel()

	if _, err := New().Start(context.Background(), executor.Command{}); !errors.Is(err, executor.ErrInvalidCommand) {
		t.Fatalf("Start(invalid) error = %v, want %v", err, executor.ErrInvalidCommand)
	}

	command := helperCommand(t, "inspect", nil, nil)
	command.Args = append(command.Args, "original-argument")
	command.Env["OWNED_VALUE"] = "original-environment"
	process, err := New().Start(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	command.Args[len(command.Args)-1] = "mutated-argument"
	command.Env["OWNED_VALUE"] = "mutated-environment"

	result, err := waitForProcess(t, process)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(result.Stdout), "original-argument/original-environment"; got != want {
		t.Fatalf("owned command output = %q, want %q", got, want)
	}
}

// FR-EXEC-01
func TestStartRejectsTypedNilStreamsBeforeStarting(t *testing.T) {
	var stream *nilSafeStream
	tests := []struct {
		name string
		set  func(*executor.Command)
	}{
		{name: "stdin", set: func(command *executor.Command) { command.Stdin = stream }},
		{name: "stdout", set: func(command *executor.Command) { command.Stdout = stream }},
		{name: "stderr", set: func(command *executor.Command) { command.Stderr = stream }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := helperCommand(t, "success", nil, nil)
			test.set(&command)
			process, err := New().Start(context.Background(), command)
			if err == nil {
				_, _ = waitForProcess(t, process)
				t.Fatalf("Start() error = nil, want %v", executor.ErrNilDependency)
			}
			if !errors.Is(err, executor.ErrNilDependency) {
				t.Fatalf("Start() error = %v, want %v", err, executor.ErrNilDependency)
			}
		})
	}
}

// FR-EXEC-03
func TestRunUsesDirectoryAndStdin(t *testing.T) {
	t.Parallel()

	t.Run("directory", func(t *testing.T) {
		directory := t.TempDir()
		resolvedDirectory, err := filepath.EvalSymlinks(directory)
		if err != nil {
			t.Fatal(err)
		}
		command := helperCommand(t, "cwd", nil, nil)
		command.Dir = directory
		result, err := New().Run(context.Background(), command)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(result.Stdout); got != resolvedDirectory {
			t.Fatalf("working directory = %q, want %q", got, resolvedDirectory)
		}
	})

	t.Run("stdin", func(t *testing.T) {
		command := helperCommand(t, "stdin", nil, nil)
		command.Stdin = strings.NewReader("input payload")
		result, err := New().Run(context.Background(), command)
		if err != nil {
			t.Fatal(err)
		}
		if got := string(result.Stdout); got != "input payload" {
			t.Fatalf("stdin echo = %q", got)
		}
	})
}

// FR-EXEC-04
func TestRunEnvironmentInheritance(t *testing.T) {
	t.Setenv("EXECUTOR_INHERITED_VALUE", "parent-environment")

	tests := []struct {
		name     string
		clearEnv bool
		want     string
	}{
		{name: "default inherits", want: "parent-environment/command-environment"},
		{name: "clear excludes inherited", clearEnv: true, want: "/command-environment"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := helperCommand(t, "environment", nil, nil)
			command.ClearEnv = test.clearEnv
			command.Env["EXECUTOR_EXPLICIT_VALUE"] = "command-environment"

			result, err := New().Run(context.Background(), command)
			if err != nil {
				t.Fatal(err)
			}
			if got := string(result.Stdout); got != test.want {
				t.Fatalf("environment = %q, want %q", got, test.want)
			}
		})
	}
}

// FR-EXEC-04
func TestRunClearEnvironmentAllowsEmptyExplicitEnvironment(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	result, err := New().Run(context.Background(), executor.Command{
		Path:     binary,
		Args:     []string{"-test.run=^TestExecutorEmptyEnvironmentHelper$", "--", "executor-empty-environment-helper"},
		Env:      map[string]string{},
		ClearEnv: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "0/false"
	if runtime.GOOS == "windows" {
		// os/exec injects SYSTEMROOT when it is missing, even when Cmd.Env is
		// otherwise empty.
		want = "1/true"
	}
	if got := string(result.Stdout); got != want {
		t.Fatalf("child environment summary = %q, want %q", got, want)
	}
}

// FR-EXEC-04
func TestBuildEnvironmentSortsOverridesAndReplacesPlatformKeys(t *testing.T) {
	t.Parallel()

	inherited := []string{"Path=old", "KEEP=value", "A=old", "=C:=drive"}
	overrides := map[string]string{"PATH": "new", "B": "second", "A": "new"}

	caseSensitive := buildEnvironment(inherited, overrides, false)
	if want := []string{"Path=old", "KEEP=value", "=C:=drive", "A=new", "B=second", "PATH=new"}; !reflect.DeepEqual(caseSensitive, want) {
		t.Fatalf("case-sensitive environment = %#v, want %#v", caseSensitive, want)
	}

	caseInsensitive := buildEnvironment(inherited, overrides, true)
	if want := []string{"KEEP=value", "=C:=drive", "A=new", "B=second", "PATH=new"}; !reflect.DeepEqual(caseInsensitive, want) {
		t.Fatalf("case-insensitive environment = %#v, want %#v", caseInsensitive, want)
	}
}

// FR-EXEC-04
func TestBuildEnvironmentCollapsesCaseEquivalentWindowsOverrides(t *testing.T) {
	t.Parallel()

	got := buildEnvironment(
		[]string{"Path=old", "KEEP=value"},
		map[string]string{"PATH": "upper", "Path": "mixed"},
		true,
	)
	if want := []string{"KEEP=value", "Path=mixed"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("case-insensitive environment = %#v, want %#v", got, want)
	}
}

// FR-EXEC-04
func TestBuildEnvironmentWithoutInheritanceSortsAndFoldsOverrides(t *testing.T) {
	t.Parallel()

	caseSensitive := buildEnvironment(nil, map[string]string{"B": "second", "A": "first"}, false)
	if want := []string{"A=first", "B=second"}; !reflect.DeepEqual(caseSensitive, want) {
		t.Fatalf("case-sensitive environment = %#v, want %#v", caseSensitive, want)
	}

	caseInsensitive := buildEnvironment(nil, map[string]string{"PATH": "upper", "Path": "mixed"}, true)
	if want := []string{"Path=mixed"}; !reflect.DeepEqual(caseInsensitive, want) {
		t.Fatalf("case-insensitive environment = %#v, want %#v", caseInsensitive, want)
	}

	empty := buildEnvironment(nil, map[string]string{}, false)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty environment = %#v, want non-nil empty slice", empty)
	}
}

// FR-EXEC-07
func TestWaitIsRepeatableAndReturnsOwnedResults(t *testing.T) {
	t.Parallel()

	process, err := New().Start(context.Background(), helperCommand(t, "success", nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	first, firstErr := waitForProcess(t, process)
	if firstErr != nil {
		t.Fatal(firstErr)
	}
	second, secondErr := waitForProcess(t, process)
	if secondErr != nil {
		t.Fatal(secondErr)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Wait results differ: first=%+v second=%+v", first, second)
	}

	first.Stdout[0] = 'X'
	first.Stderr[0] = 'Y'
	third, thirdErr := waitForProcess(t, process)
	if thirdErr != nil {
		t.Fatal(thirdErr)
	}
	if string(third.Stdout) != "out\n" || string(third.Stderr) != "err\n" {
		t.Fatalf("later Wait observed caller mutation: %q/%q", third.Stdout, third.Stderr)
	}
	if third.StartedAt != second.StartedAt || third.FinishedAt != second.FinishedAt || third.ExitCode != second.ExitCode {
		t.Fatalf("stable fields changed: second=%+v third=%+v", second, third)
	}
}

// FR-EXEC-07
func TestExitWaitClonesResultAndExitErrorOutput(t *testing.T) {
	t.Parallel()

	process, err := New().Start(context.Background(), helperCommand(t, "exit7", nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	first, firstErr := waitForProcess(t, process)
	var firstExit *executor.ExitError
	if !errors.As(firstErr, &firstExit) {
		t.Fatalf("first Wait error = %v", firstErr)
	}
	first.Stdout[0] = 'X'
	firstExit.Result.Stdout[0] = 'Y'

	second, secondErr := waitForProcess(t, process)
	var secondExit *executor.ExitError
	if !errors.As(secondErr, &secondExit) {
		t.Fatalf("second Wait error = %v", secondErr)
	}
	if string(second.Stdout) != "out7\n" || string(secondExit.Result.Stdout) != "out7\n" {
		t.Fatalf("second Wait reused mutable output: result=%q exit=%q", second.Stdout, secondExit.Result.Stdout)
	}
}

// FR-EXEC-07
func TestWaitCancellationDoesNotStopProcess(t *testing.T) {
	t.Parallel()

	process, err := New().Start(context.Background(), helperCommand(t, "delay", nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	waitContext, cancelWait := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancelWait()
	if result, err := process.Wait(waitContext); !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(result, executor.Result{}) {
		t.Fatalf("timed Wait = (%+v, %v), want zero result and deadline", result, err)
	}

	result, err := waitForProcess(t, process)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || string(result.Stdout) != "delayed\n" {
		t.Fatalf("later Wait result = %+v", result)
	}
}

// FR-EXEC-07
func TestStartAndRunCancellationReturnDiscoverableExitError(t *testing.T) {
	t.Parallel()

	t.Run("start then wait", func(t *testing.T) {
		startContext, cancelProcess := context.WithCancel(context.Background())
		process, err := New().Start(startContext, helperCommand(t, "block", nil, nil))
		if err != nil {
			t.Fatal(err)
		}
		cancelProcess()
		result, err := waitForProcess(t, process)
		var exit *executor.ExitError
		if !errors.Is(err, context.Canceled) || !errors.As(err, &exit) {
			t.Fatalf("Wait error = %v, want canceled *ExitError", err)
		}
		if !reflect.DeepEqual(exit.Result, result) || result.StartedAt.IsZero() || result.FinishedAt.Before(result.StartedAt) {
			t.Fatalf("canceled result=%+v exit=%+v", result, exit)
		}
	})

	t.Run("run", func(t *testing.T) {
		runContext, cancelRun := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancelRun()
		result, err := New().Run(runContext, helperCommand(t, "block", nil, nil))
		var exit *executor.ExitError
		if !errors.Is(err, context.DeadlineExceeded) || !errors.As(err, &exit) {
			t.Fatalf("Run error = %v, want deadline *ExitError", err)
		}
		if !reflect.DeepEqual(exit.Result, result) || result.StartedAt.IsZero() || result.FinishedAt.Before(result.StartedAt) {
			t.Fatalf("canceled Run result=%+v exit=%+v", result, exit)
		}
	})
}

// FR-EXEC-06
func TestCallerWriterErrorIsWaitErrorAndCaptureRemainsOwned(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("destination failed")
	command := helperCommand(t, "success", failingWriter{err: wantErr}, nil)
	process, err := New().Start(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	result, err := waitForProcess(t, process)
	var exit *executor.ExitError
	if !errors.Is(err, wantErr) || !errors.As(err, &exit) {
		t.Fatalf("Wait error = %v, want writer error in *ExitError", err)
	}
	if result.ExitCode != 0 || string(result.Stdout) != "out\n" || string(exit.Result.Stdout) != "out\n" {
		t.Fatalf("writer-error capture result=%+v exit=%+v", result, exit)
	}

	result.Stdout[0] = 'X'
	exit.Result.Stdout[0] = 'Y'
	again, againErr := waitForProcess(t, process)
	if !errors.Is(againErr, wantErr) || string(again.Stdout) != "out\n" {
		t.Fatalf("later Wait = (%q, %v)", again.Stdout, againErr)
	}
}

// FR-EXEC-03
func TestSharedDestinationWritesAreSerializedAcrossStreams(t *testing.T) {
	t.Parallel()

	destination := &overlapWriter{}
	result, err := New().Run(context.Background(), helperCommand(t, "flood", destination, destination))
	if err != nil {
		t.Fatal(err)
	}
	if destination.overlapped.Load() {
		t.Fatal("stdout and stderr writes overlapped at the shared destination")
	}
	if len(result.Stdout) == 0 || len(result.Stderr) == 0 {
		t.Fatalf("missing captured output: stdout=%d stderr=%d", len(result.Stdout), len(result.Stderr))
	}
	if got, want := destination.buffer.Len(), len(result.Stdout)+len(result.Stderr); got != want {
		t.Fatalf("destination bytes = %d, want %d", got, want)
	}
}

// FR-EXEC-08
func TestSignalAndKill(t *testing.T) {
	t.Parallel()

	t.Run("invalid signal", func(t *testing.T) {
		process, err := New().Start(context.Background(), helperCommand(t, "delay", nil, nil))
		if err != nil {
			t.Fatal(err)
		}
		if err := process.Signal(context.Background(), "BOGUS"); !errors.Is(err, executor.ErrInvalidSignal) {
			t.Fatalf("Signal() error = %v, want %v", err, executor.ErrInvalidSignal)
		}
		if _, err := waitForProcess(t, process); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("canceled kill", func(t *testing.T) {
		process, err := New().Start(context.Background(), helperCommand(t, "delay", nil, nil))
		if err != nil {
			t.Fatal(err)
		}
		killContext, cancelKill := context.WithCancel(context.Background())
		cancelKill()
		if err := process.Kill(killContext); !errors.Is(err, context.Canceled) {
			t.Fatalf("Kill() error = %v, want %v", err, context.Canceled)
		}
		if _, err := waitForProcess(t, process); err != nil {
			t.Fatal(err)
		}
	})
}

// FR-EXEC-08
func TestPlatformSignals(t *testing.T) {
	if runtime.GOOS == "windows" {
		for _, name := range []string{"TERM", "INT", "HUP"} {
			process, err := New().Start(context.Background(), helperCommand(t, "delay", nil, nil))
			if err != nil {
				t.Fatal(err)
			}
			if err := process.Signal(context.Background(), name); !errors.Is(err, executor.ErrInvalidSignal) {
				t.Fatalf("Signal(%q) error = %v, want %v", name, err, executor.ErrInvalidSignal)
			}
			if _, err := waitForProcess(t, process); err != nil {
				t.Fatal(err)
			}
		}
	} else {
		for _, name := range []string{"TERM", "KILL", "INT", "HUP"} {
			t.Run(name, func(t *testing.T) {
				process, err := New().Start(context.Background(), helperCommand(t, "block", nil, nil))
				if err != nil {
					t.Fatal(err)
				}
				if err := process.Signal(context.Background(), name); err != nil {
					t.Fatalf("Signal(%q) error = %v", name, err)
				}
				if _, err := waitForProcess(t, process); err == nil {
					t.Fatalf("Wait after Signal(%q) returned nil", name)
				}
			})
		}
	}

	process, err := New().Start(context.Background(), helperCommand(t, "block", nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Kill(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := waitForProcess(t, process); err == nil {
		t.Fatal("Wait after Kill() returned nil")
	}
}

func helperCommand(t *testing.T, mode string, stdout, stderr io.Writer) executor.Command {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return executor.Command{
		Path: binary,
		Args: []string{"-test.run=^TestExecutorHelper$"},
		Env: map[string]string{
			"GO_WANT_EXECUTOR_HELPER": "1",
			"EXECUTOR_HELPER_MODE":    mode,
		},
		Stdout: stdout,
		Stderr: stderr,
	}
}

func waitForProcess(t *testing.T, process executor.Process) (executor.Result, error) {
	t.Helper()
	waitContext, cancelWait := context.WithTimeout(context.Background(), processTestTimeout)
	defer cancelWait()
	return process.Wait(waitContext)
}

type failingWriter struct {
	err error
}

type nilSafeStream struct{}

func (*nilSafeStream) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (*nilSafeStream) Write(value []byte) (int, error) {
	return len(value), nil
}

func (writer failingWriter) Write(value []byte) (int, error) {
	return len(value), writer.err
}

type overlapWriter struct {
	active     atomic.Int32
	overlapped atomic.Bool
	buffer     bytes.Buffer
}

func (writer *overlapWriter) Write(value []byte) (int, error) {
	if writer.active.Add(1) != 1 {
		writer.overlapped.Store(true)
	}
	runtime.Gosched()
	time.Sleep(50 * time.Microsecond)
	written, err := writer.buffer.Write(value)
	writer.active.Add(-1)
	return written, err
}
