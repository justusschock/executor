# go-executor

`go-executor` provides command and process contracts, a local
`os/exec` implementation, bounded ordered batch execution, and a thin adapter
for remote starters.

The module executes programs directly. A shell is involved only when a caller
opts in through `Shell` or `ShellWith`; the module does not add a transport,
PTY, sandbox, or durable process manager.

## Contracts

- `Command` describes a path, arguments, working directory, environment
  behavior, optional standard streams, and independent stdout/stderr capture
  limits. `Env` overrides inherited values by default; `ClearEnv` prevents the
  executor from supplying inherited values. `Validate` checks portable input
  invariants, and `Clone` copies `Args` and `Env` while retaining stream
  references. `ProcessGroup` and `KillGrace` control how far process control
  reaches and how cancellation escalates.
- `Shell` and `ShellWith` build commands that run a script through a shell.
- `Result` contains the exit code, start and finish times, captured stdout and
  stderr, and best-effort resource `Usage`. Truncation flags distinguish
  complete output from a retained prefix.
- `Executor` exposes `Run` and `Start`; `Process` exposes repeatable `Wait`,
  `Signal`, `Kill`, and lifecycle-reference `Export` operations.
- `local.Executor` implements `Executor` with `os/exec`.
- `RunAll` adds bounded concurrent execution without changing result order.
- `NewPrefixWriter` labels streamed output lines without interleaving lines
  from concurrent commands.
- `Remote` turns any `Starter` implementation into an `Executor`; it does not
  implement a network transport itself.

Module path: `github.com/justusschock/go-executor`

## Install the private module

You need repository access for this module and its private dependencies. Use a
Git credential helper or SSH agent; do not place access tokens in Git URLs,
`go env`, source files, or shell history. For example, GitHub CLI can use its
browser login and configure Git's credential helper:

```bash
gh auth login --hostname github.com --git-protocol https --web
gh auth setup-git --hostname github.com
```

Tell Go to fetch the private module family directly and skip the public
checksum database for it:

```bash
go env -w GOPRIVATE='github.com/justusschock/*'
go env -w GONOSUMDB='github.com/justusschock/*'
go get github.com/justusschock/go-executor@latest
```

`GOPRIVATE` already supplies the default private-module behavior;
`GONOSUMDB` is set explicitly here so the checksum policy remains clear. An
existing GitHub SSH setup works as well.

## Run a local command

By default, the local executor inherits the current environment and applies
`Command.Env` overrides. It honors `Dir` and `Stdin`, and always captures stdout
and stderr. Configured output writers receive the same data while the command
runs.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	executor "github.com/justusschock/go-executor"
	"github.com/justusschock/go-executor/local"
)

func main() {
	result, err := local.New().Run(context.Background(), executor.Command{
		Path:           "go",
		Args:           []string{"version"},
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
		MaxStdoutBytes: 1 << 20,
		MaxStderrBytes: 1 << 20,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("exit code: %d\n", result.ExitCode)
}
```

Set `Command.ClearEnv` to run without inherited environment values. In that
mode, `go-executor` supplies only the entries in `Command.Env`; a nil or empty
map produces a non-nil empty `os/exec.Cmd.Env`, which disables normal parent
environment inheritance. The zero value, `false`, preserves the default
inheritance behavior.

The underlying `os/exec` package may still add values required by the platform.
In particular, on Windows it adds `SYSTEMROOT` when that key is missing and is
not explicitly set to an empty string. Callers are responsible for supplying
every other platform variable the child needs. `ClearEnv` controls the child
environment only; executable path lookup still follows `os/exec` lookup rules
before the child starts.

`Result.Stdout` and `Result.Stderr` still contain the output when live writers
are configured. The repository's portable, compile-checked examples use the
current test executable and live in
[`local/example_test.go`](local/example_test.go).

`MaxStdoutBytes` and `MaxStderrBytes` limit only the bytes retained in the
result. A zero value preserves the compatibility behavior of retaining all
output. A positive limit retains that stream's prefix, continues draining the
child, forwards all bytes to its configured writer, and sets the corresponding
`Result.StdoutTruncated` or `Result.StderrTruncated` flag when bytes were
omitted. Negative limits are invalid.

## Start and control a process

`Start` separates process lifetime from an individual wait:

```go
processContext, cancelProcess := context.WithCancel(context.Background())
defer cancelProcess()

process, err := local.New().Start(processContext, executor.Command{
	Path: "go",
	Args: []string{"version"},
})
if err != nil {
	return err
}

waitContext, cancelWait := context.WithTimeout(context.Background(), 5*time.Second)
defer cancelWait()
result, err := process.Wait(waitContext)
```

- For `local.Executor`, the context passed to `Start` owns the child lifetime.
  Canceling it terminates the direct child, or its process group when
  `Command.ProcessGroup` is set. `Run` still waits for the child to be reaped
  before it returns.
- A context passed to `Wait` controls only that call. A canceled or timed-out
  wait does not terminate the process; a later `Wait` can return the final
  result. `Wait` is repeatable and local waits return independently owned
  output slices.
- `Signal` accepts exact uppercase names. Unix-like builds support `TERM`,
  `KILL`, `INT`, and `HUP`; Windows supports `KILL`. `Kill` sends the platform's
  unconditional kill operation. Both methods reject an already-canceled
  operation context before signaling.
- Without `Command.ProcessGroup`, process control targets the direct child
  only; see the next section for whole-tree control.

## Run shell scripts and control process groups

`Shell` runs a script through `/bin/sh -c` on Unix-like systems
(`/system/bin/sh -c` on Android) and `cmd.exe /d /s /c` on Windows.
`ShellWith` selects another interpreter; each zero `ShellOptions` field falls
back to `DefaultShell` independently:

```go
train := executor.Shell("python train.py --lr 0.1")
train.Dir = "pipelines/model"

strict := executor.ShellWith(executor.ShellOptions{
	Path: "bash",
	Args: []string{"-euo", "pipefail", "-c"},
}, "make data | tee data.log")
```

The script is interpreted by the shell, so never interpolate untrusted input
into it. On Windows, the local executor passes a `cmd.exe ... /s /c` script
verbatim inside one pair of quotes instead of applying the standard argument
escaping that `cmd.exe` does not understand.

Shell commands set `ProcessGroup: true` and `KillGrace: DefaultKillGrace`
(10 seconds), because scripts usually start further programs. Any command can
set these fields; their zero values preserve the direct-child behavior.

- `ProcessGroup` starts the child as the leader of a new process group
  (`Setpgid` on Unix). On Windows, it uses `CREATE_NEW_PROCESS_GROUP` and
  assigns the child to a Job Object, falling back to `taskkill /T /F` when the
  job cannot be created or assigned. `Signal`, `Kill`, and start-context
  cancellation then reach grandchildren, such as programs run by `sh -c`.
  After the group has been reaped, they return `os.ErrProcessDone`. A Unix
  group child is not in the terminal's foreground group, so do not pass a
  terminal as its stdin.
- `KillGrace` controls start-context cancellation. Zero kills immediately. A
  positive value sends `TERM` first and sends `KILL` if the process has not
  been reaped when the grace period ends. Windows has no `TERM`, so it kills
  immediately. Negative values are invalid.

In both modes, `Run` and `Wait` still expose the start-context cancellation
through `errors.Is`.

## Record resource usage

`Result.Usage` is populated for successful runs and for `ExitError` results.
Zero fields mean that the platform did not report a value:

- `UserCPU` and `SystemCPU` come from the process state on every platform.
- `MaxRSSBytes` is the peak resident set size from `rusage` on Unix-like
  systems, normalized from kilobytes (Linux and BSDs) or bytes (macOS). Windows
  reports zero.
- `ReadBytes` and `WriteBytes` are reported on Linux and Android, where
  `ru_inblock` and `ru_oublock` count 512-byte units of storage I/O. Reads served
  from the page cache are not included. Other platforms report zero.

On Unix-like systems, `rusage` includes descendants that the child waited for,
so a shell's usage includes the programs it ran in the foreground.

## Label streamed output

`NewPrefixWriter` wraps a destination writer and prefixes every line. All
prefix writers that share a destination serialize their writes, so concurrent
commands can stream into one terminal without splitting lines:

```go
stdout := executor.NewPrefixWriter(os.Stdout, "[train] ")
defer stdout.Close()

command := executor.Shell("python train.py")
command.Stdout = stdout
command.Stderr = stdout
```

Each destination write contains only complete lines. A partial line longer
than `MaxPrefixLineBytes` (64 KiB) is emitted in completed chunks, and `Close`
writes any remaining partial line with a trailing newline. Close each prefix
writer after its command finishes to flush output and release the shared
destination lock.

## Run commands concurrently

`RunAll` invokes at most `limit` concurrent `Run` calls. Successful results are
returned in input order even when commands finish in a different order:

```go
commands := []executor.Command{
	{Path: "go", Args: []string{"env", "GOOS"}},
	{Path: "go", Args: []string{"env", "GOARCH"}},
}

results, err := executor.RunAll(context.Background(), local.New(), commands, 2)
if err != nil {
	return err
}
for index, result := range results {
	fmt.Printf("command %d: %s", index, result.Stdout)
}
```

The supplied executor must support concurrent calls. Commands are cloned
eagerly, but stream references remain shallow; synchronize a writer or reader
that is shared by multiple commands. On the first command failure, `RunAll`
stops further admission, cancels admitted peers through their contexts, waits
for them, and returns no partial results. Admitted failures are wrapped with
their command indexes and joined in admission order. A non-positive limit
returns `concurrency.ErrInvalidLimit` from `go-concurrency`.

## Adapt a remote starter

Remote transports implement the small root interface:

```go
type Starter interface {
	Start(context.Context, executor.Command) (executor.Process, error)
}
```

Pass that implementation to `executor.NewRemote`. `Remote.Start` clones and
validates the command before forwarding it, and `Remote.Run` performs one
`Start` followed by one `Wait` with the same context. Transport, authentication,
session, and custody semantics remain the adapter's responsibility.

## Export process references

`Process.Export` exports a live local process as a `handoff.Artifact` with media
type `application/vnd.justusschock.process-reference+json`, version `1`, and
only these JSON fields:

- `pid`
- `path`
- `started_at` in RFC3339Nano UTC form

Arguments, environment values, stdin, and stream references are deliberately
omitted. Export does not stop the process or transfer its custody: the original
caller remains responsible for signaling and reaping it. The reference can
also become stale after export and is not sufficient to prove that a reused PID
identifies the same process.

Exporting an unavailable process returns `ErrNotStarted`; exporting one that
has exited returns `ErrProcessExited`. The core module does not reattach to or
adopt these references. `Remote.Adopt` always returns
`ErrUnsupportedAdoption`; a transport capable of real custody transfer belongs
in a separate adapter.

## Ownership, cancellation, and errors

- `local.Executor.Start`, `Remote.Start`, and `RunAll` copy `Command.Args` and
  `Command.Env` before execution. `Stdin`, `Stdout`, and `Stderr` are
  interfaces and remain shared references. Typed-nil stream implementations
  are rejected with `ErrNilDependency`; plain nil streams are allowed.
- A non-zero exit or wait failure returns both a populated `Result` and
  `*executor.ExitError`. Inspect it with `errors.As`; its cause remains visible
  through `errors.Is`, including a start-context cancellation or deadline.
- A configured destination-writer failure is a wait failure and may therefore
  produce an `ExitError` even when the program itself exits with code zero.
- Command validation errors wrap `ErrInvalidCommand`. Unsupported signal names
  wrap `ErrInvalidSignal`.
- When the start context terminates the child, that cancellation or deadline
  remains discoverable when the process is reaped. Canceling only a `Wait`
  returns that wait context's cause without changing the process.

```go
result, err := local.New().Run(ctx, command)
if err != nil {
	var exit *executor.ExitError
	if errors.As(err, &exit) {
		// result and exit.Result both contain the captured process outcome.
	}
	if errors.Is(err, context.DeadlineExceeded) {
		// The command's start context expired.
	}
}
```

## Supported platforms and scope

The neutral root contracts and remote adapter contain no OS process code. The
`local` package supports Windows and these Unix-like `GOOS` targets: `aix`,
`android`, `darwin`, `dragonfly`, `freebsd`, `linux`, `netbsd`, `openbsd`, and
`solaris`.

Other targets, including Plan 9, JavaScript, and WASI, are not supported by the
local implementation. There is no implicit shell expansion, PTY, privilege
escalation, SSH/Kubernetes transport, sandbox, or durable execution in this
module. Process-tree control is limited to opt-in process groups.

## Development

Go 1.25 or newer is required. Useful focused checks are:

```bash
go test -race ./local
go test -race -run '^TestRunAll' .
go test ./local -run '^Example'
go vet ./...
go mod verify
pre-commit run --all-files
```

Install the repository hooks once with `pre-commit install`. Local
implementation plans live under `docs/` and are intentionally excluded from
Git.

## License

Apache License 2.0. See [LICENSE](LICENSE).
