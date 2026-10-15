//go:build aix || android || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package local

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	executor "github.com/justusschock/go-executor"
)

const grandchildScript = `sleep 100 & echo $!; wait`

func TestProcessGroupCancellationKillsGrandchildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		script    string
		killGrace time.Duration
		minimum   time.Duration
	}{
		{name: "immediate kill", script: grandchildScript},
		{name: "graceful terminate", script: grandchildScript, killGrace: executor.DefaultKillGrace},
		{name: "kill after ignored terminate", script: `trap "" TERM; ` + grandchildScript, killGrace: 300 * time.Millisecond, minimum: 300 * time.Millisecond},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			startContext, cancelProcess := context.WithCancel(context.Background())
			defer cancelProcess()
			command := executor.Shell(test.script)
			command.KillGrace = test.killGrace
			stdout := newFirstLineWriter()
			command.Stdout = stdout

			process, err := New().Start(startContext, command)
			if err != nil {
				t.Fatal(err)
			}
			grandchild := stdout.pid(t)
			assertProcessAlive(t, grandchild)

			canceledAt := time.Now()
			cancelProcess()
			result, err := waitForProcess(t, process)
			elapsed := time.Since(canceledAt)
			var exit *executor.ExitError
			if !errors.Is(err, context.Canceled) || !errors.As(err, &exit) {
				t.Fatalf("Wait error = %v, want canceled *ExitError", err)
			}
			if result.ExitCode != -1 {
				t.Fatalf("exit code = %d, want -1 for a signaled shell", result.ExitCode)
			}
			if elapsed < test.minimum || elapsed > processTestTimeout {
				t.Fatalf("cancellation took %v, want between %v and %v", elapsed, test.minimum, processTestTimeout)
			}
			assertProcessGone(t, grandchild)
		})
	}
}

func TestProcessGroupSignalAndKillReachGrandchildren(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		stop func(executor.Process) error
	}{
		{name: "signal", stop: func(process executor.Process) error { return process.Signal(context.Background(), "TERM") }},
		{name: "kill", stop: func(process executor.Process) error { return process.Kill(context.Background()) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			command := executor.Shell(grandchildScript)
			stdout := newFirstLineWriter()
			command.Stdout = stdout
			process, err := New().Start(context.Background(), command)
			if err != nil {
				t.Fatal(err)
			}
			grandchild := stdout.pid(t)

			if err := test.stop(process); err != nil {
				t.Fatal(err)
			}
			if _, err := waitForProcess(t, process); err == nil {
				t.Fatal("Wait after stopping the group returned nil")
			}
			assertProcessGone(t, grandchild)

			if err := process.Kill(context.Background()); !errors.Is(err, os.ErrProcessDone) {
				t.Fatalf("Kill after exit error = %v, want %v", err, os.ErrProcessDone)
			}
			if err := process.Signal(context.Background(), "BOGUS"); !errors.Is(err, executor.ErrInvalidSignal) {
				t.Fatalf("Signal(BOGUS) error = %v, want %v", err, executor.ErrInvalidSignal)
			}
		})
	}
}

func TestShellRunsScriptsThroughShell(t *testing.T) {
	t.Parallel()

	result, err := New().Run(context.Background(), executor.Shell(`printf '%s\n' "a b"; echo err >&2; exit 0`))
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Stdout) != "a b\n" || string(result.Stderr) != "err\n" {
		t.Fatalf("shell output = %q/%q", result.Stdout, result.Stderr)
	}

	strict := executor.ShellWith(executor.ShellOptions{Args: []string{"-e", "-c"}}, "false; echo unreachable")
	result, err = New().Run(context.Background(), strict)
	var exit *executor.ExitError
	if !errors.As(err, &exit) || result.ExitCode != 1 || len(result.Stdout) != 0 {
		t.Fatalf("strict shell = (%+v, %v), want exit 1 without output", result, err)
	}
}

// firstLineWriter publishes the first output line, which the test scripts use
// to report the grandchild PID.
type firstLineWriter struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	once   sync.Once
	lines  chan string
}

func newFirstLineWriter() *firstLineWriter {
	return &firstLineWriter{lines: make(chan string, 1)}
}

func (writer *firstLineWriter) Write(value []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.buffer.Write(value)
	if line, _, found := strings.Cut(writer.buffer.String(), "\n"); found {
		writer.once.Do(func() { writer.lines <- line })
	}
	return len(value), nil
}

func (writer *firstLineWriter) pid(t *testing.T) int {
	t.Helper()
	select {
	case line := <-writer.lines:
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || pid <= 0 {
			t.Fatalf("grandchild PID line = %q", line)
		}
		return pid
	case <-time.After(processTestTimeout):
		t.Fatal("timed out waiting for the grandchild PID")
		return 0
	}
}

func assertProcessAlive(t *testing.T, pid int) {
	t.Helper()
	if err := syscall.Kill(pid, 0); err != nil {
		t.Fatalf("grandchild %d is not running: %v", pid, err)
	}
}

// assertProcessGone polls because an orphaned grandchild is reaped by its new
// parent asynchronously after it has been killed.
func assertProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(processTestTimeout)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("grandchild %d survived its process group: kill(0) = %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
