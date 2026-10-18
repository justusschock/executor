# go-executor

`go-executor` is a Go library for running external commands with precise,
tested semantics: portable command and process contracts, a local `os/exec`
implementation with bounded output capture, process-group control and
graceful cancellation, ordered parallel batches, line-prefixed output
streaming, and a thin adapter for remote transports. It is for Go developers
who build tools, pipeline runners, job agents and CLIs that start other
programs and need to stop them reliably and report what happened.

Programs run directly. A shell is involved only when you ask for one through
`Shell` or `ShellWith`; the module adds no transport, PTY, sandbox or durable
process manager.

## Features

- **Command contract**: `Command` describes path, arguments, working
  directory, environment (inherited with overrides, or cleared with
  `ClearEnv`), optional standard streams and per-stream capture limits.
  `Validate` checks portable invariants; `Clone` copies `Args` and `Env`.
- **Local execution**: `local.New()` returns an `Executor` that runs commands
  with `os/exec`, streams output to your writers while capturing it, and
  reports exit code, timing and resource `Usage`.
- **Start, wait and control**: `Start` returns a `Process` with repeatable
  `Wait`, `Signal`, `Kill` and `Export`. The start context owns the process
  lifetime; wait contexts bound only the wait.
- **Process groups and graceful stop**: `ProcessGroup` makes signals and
  cancellation reach grandchildren; `KillGrace` sends `TERM` first and `KILL`
  after a grace period.
- **Shell commands**: `Shell` and `ShellWith` run scripts through the
  platform shell or an interpreter of your choice, in their own process group.
- **Bounded capture**: `MaxStdoutBytes`/`MaxStderrBytes` cap retained output
  while writers still receive every byte.
- **Ordered batches**: `RunAll` runs commands with bounded concurrency,
  returns results in input order and stops on the first failure.
- **Prefixed output**: `NewPrefixWriter` labels lines and never interleaves
  lines from concurrent commands that share a destination.
- **Remote adapters**: `NewRemote` turns any one-method `Starter` into an
  `Executor`.
- Errors that keep the result: non-zero exits return the populated `Result`
  and an `*ExitError`. No logging; safe for concurrent use.

## Installation

```bash
go get github.com/justusschock/go-executor@latest
```

Requires Go 1.25 or newer. Import the contracts and the local
implementation as:

```go
import (
	executor "github.com/justusschock/go-executor"
	"github.com/justusschock/go-executor/local"
)
```

## Quick start

Run a shell command locally and inspect the result. This is the package
[`Example`](example_test.go), so it is compiled and checked by `go test`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	executor "github.com/justusschock/go-executor"
	"github.com/justusschock/go-executor/local"
)

func main() {
	result, err := local.New().Run(context.Background(), executor.Shell("echo hello"))
	if err != nil {
		var exit *executor.ExitError
		if errors.As(err, &exit) {
			fmt.Println("exit code:", exit.Result.ExitCode)
		}
		log.Fatal(err)
	}
	fmt.Println(strings.TrimSpace(string(result.Stdout)), result.ExitCode) // hello 0
}
```

[`example_test.go`](example_test.go) also has `ExampleRunAll`,
`ExampleNewPrefixWriter` and `ExampleNewRemote`;
[`local/example_test.go`](local/example_test.go) shows `Executor.Run`.

### Run a program directly

```go
result, err := local.New().Run(ctx, executor.Command{
	Path:           "go",
	Args:           []string{"version"},
	Stdout:         os.Stdout, // streamed live and captured in result.Stdout
	Stderr:         os.Stderr,
	MaxStdoutBytes: 1 << 20, // retain at most 1 MiB; os.Stdout still gets everything
	MaxStderrBytes: 1 << 20,
})
```

By default the child inherits the current environment with `Command.Env`
entries overriding it. With `ClearEnv: true`, only `Command.Env` is supplied;
`os/exec` may still add values the platform requires (on Windows,
`SYSTEMROOT`). `ClearEnv` does not change how `Path` is looked up.

A positive capture limit retains that stream's prefix, keeps draining the
child, forwards every byte to the configured writer, and sets
`Result.StdoutTruncated` or `Result.StderrTruncated` when bytes were dropped.
Zero retains everything; negative limits are invalid.

### Start and control a process

```go
processContext, cancelProcess := context.WithCancel(context.Background())
defer cancelProcess()

process, err := local.New().Start(processContext, executor.Command{Path: "go", Args: []string{"version"}})
if err != nil {
	return err
}

waitContext, cancelWait := context.WithTimeout(context.Background(), 5*time.Second)
defer cancelWait()
result, err := process.Wait(waitContext)
```

- The context passed to `Start` owns the child's lifetime. Cancelling it
  stops the child (or its process group); `Run` still waits until the child
  has been reaped.
- A context passed to `Wait` bounds only that call. A timed-out `Wait` leaves
  the process running, and a later `Wait` returns the final result. Every
  `Wait` returns independently owned output.
- `Signal` takes exact uppercase names: `TERM`, `KILL`, `INT` and `HUP` on
  Unix-like systems, `KILL` on Windows. `Kill` stops the process
  unconditionally.

### Shell scripts and process groups

`Shell` runs a script through `/bin/sh -c` on Unix-like systems
(`/system/bin/sh -c` on Android) and `cmd.exe /d /s /c` on Windows.
`ShellWith` selects another interpreter; each zero `ShellOptions` field falls
back to `DefaultShell`:

```go
train := executor.Shell("python train.py --lr 0.1")
train.Dir = "pipelines/model"

strict := executor.ShellWith(executor.ShellOptions{
	Path: "bash",
	Args: []string{"-euo", "pipefail", "-c"},
}, "make data | tee data.log")
```

The shell interprets the script, so never interpolate untrusted input into
it. Shell commands set `ProcessGroup: true` and
`KillGrace: DefaultKillGrace` (10 seconds); any command can set these fields.

- `ProcessGroup` starts the child as the leader of a new process group
  (`Setpgid` on Unix). On Windows it uses `CREATE_NEW_PROCESS_GROUP` and a Job
  Object, falling back to `taskkill /T /F`. `Signal`, `Kill` and start-context
  cancellation then reach grandchildren; after the group has been reaped they
  return `os.ErrProcessDone`. Do not pass a terminal as stdin to a Unix group
  child: it is not in the terminal's foreground group.
- `KillGrace` controls start-context cancellation. Zero kills immediately; a
  positive value sends `TERM` and then `KILL` if the process has not been
  reaped when the grace period ends. Windows has no `TERM` and kills
  immediately.

### Run commands concurrently

```go
commands := []executor.Command{
	{Path: "go", Args: []string{"env", "GOOS"}},
	{Path: "go", Args: []string{"env", "GOARCH"}},
}
results, err := executor.RunAll(ctx, local.New(), commands, 2)
```

`RunAll` makes at most `limit` concurrent `Run` calls and returns results in
input order. On the first failure it stops admitting commands, cancels the
running ones through their contexts, waits for them and returns no results;
failures are labelled with their command index and joined. A non-positive
limit returns `concurrency.ErrInvalidLimit` from
[go-concurrency](https://github.com/justusschock/go-concurrency).

### Label streamed output

```go
stdout := executor.NewPrefixWriter(os.Stdout, "[train] ")
defer stdout.Close()

command := executor.Shell("python train.py")
command.Stdout = stdout
command.Stderr = stdout
```

Prefix writers that share a destination serialize their writes, and each
destination write contains only complete lines. A partial line longer than
`MaxPrefixLineBytes` (64 KiB) is emitted in chunks. Close each writer after
its command finishes to flush the last partial line and release the shared
destination lock.

### Resource usage

`Result.Usage` is filled for successful runs and `ExitError` results; zero
means "not reported":

- `UserCPU` and `SystemCPU` on every platform.
- `MaxRSSBytes` (peak resident set size) on Unix-like systems.
- `ReadBytes` and `WriteBytes` (storage I/O, excluding page-cache hits) on
  Linux and Android.

On Unix-like systems the values include descendants the child waited for, so
a shell's usage includes the programs it ran in the foreground.

### Remote adapters and process references

A transport implements one method and gets a full `Executor`:

```go
type Starter interface {
	Start(context.Context, executor.Command) (executor.Process, error)
}

remote, err := executor.NewRemote(myStarter)
```

`Remote.Start` clones and validates the command before forwarding it;
`Remote.Run` is one `Start` and one `Wait` with the same context. Transport,
authentication and custody are the adapter's responsibility.

`Process.Export` returns a `handoff.Artifact` from
[go-lifecycle](https://github.com/justusschock/go-lifecycle) with media type
`application/vnd.justusschock.process-reference+json`, version `1`, and only
the JSON fields `pid`, `path` and `started_at`. Arguments, environment and
streams are never included. Export does not transfer custody, and a
reference can become stale. `Remote.Adopt` returns `ErrUnsupportedAdoption`.

## Packages

| Package | Import path | Purpose |
|---|---|---|
| `executor` | `github.com/justusschock/go-executor` | Contracts (`Command`, `Executor`, `Process`, `Result`, `ExitError`), shells, `RunAll`, `NewPrefixWriter`, `Remote` |
| `local` | `github.com/justusschock/go-executor/local` | `Executor` implementation for the local machine using `os/exec` |

Anything under `internal/` (none today) is not part of the API.

## Design and guarantees

- **Concurrency**: `local.Executor`, its processes and prefix writers are
  safe for concurrent use; `Remote` is safe when its `Starter` is. `RunAll`
  requires an executor that supports concurrent `Run` calls. Commands are
  cloned before execution, but `Stdin`, `Stdout` and `Stderr` remain shared
  references; synchronize a stream that several commands share.
- **Errors**: a non-zero exit or wait failure (including a failing output
  writer) returns both the populated `Result` and an `*executor.ExitError`;
  use `errors.As` to get it and `errors.Is` to find its cause, including a
  start-context `context.Canceled` or `context.DeadlineExceeded`. Invalid
  commands wrap `ErrInvalidCommand`, typed-nil streams wrap
  `ErrNilDependency`, unsupported signal names wrap `ErrInvalidSignal`.

  ```go
  result, err := local.New().Run(ctx, command)
  if err != nil {
  	var exit *executor.ExitError
  	if errors.As(err, &exit) {
  		// result and exit.Result both hold the captured outcome.
  	}
  	if errors.Is(err, context.DeadlineExceeded) {
  		// The start context expired and the process was stopped.
  	}
  }
  ```

- **Platforms**: the root package contains no OS process code. `local`
  supports Windows and `aix`, `android`, `darwin`, `dragonfly`, `freebsd`,
  `linux`, `netbsd`, `openbsd` and `solaris`. Plan 9, JavaScript and WASI are
  not supported.
- **Stability**: the module is `v0.x`. Minor releases may break the API;
  every break is listed in [CHANGELOG.md](CHANGELOG.md) and backed by an ADR.

## Documentation

- API reference: [pkg.go.dev/github.com/justusschock/go-executor](https://pkg.go.dev/github.com/justusschock/go-executor)
- Requirements and design decisions: [docs/README.md](docs/README.md)
  (PRD and ADRs)

## Contributing

Contributions are welcome through pull requests; nothing is pushed to `main`
directly. Read [AGENTS.md](AGENTS.md) first: it describes the docs-first
process, the public API rules and the checks to run before opening a pull
request.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) and
[NOTICE](NOTICE).
