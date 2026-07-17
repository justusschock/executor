# go-executor

`go-executor` provides command and process contracts, a local
`os/exec` implementation, bounded ordered batch execution, and a thin adapter
for remote starters.

The module executes programs directly; it does not add a shell, transport, PTY,
sandbox, or durable process manager.

## Contracts

- `Command` describes a path, arguments, working directory, environment
  overrides, optional standard streams, and independent stdout/stderr capture
  limits. `Validate` checks portable input invariants, and `Clone` copies
  `Args` and `Env` while retaining stream references.
- `Result` contains the exit code, start and finish times, and captured stdout
  and stderr. Truncation flags distinguish complete output from a retained
  prefix.
- `Executor` exposes `Run` and `Start`; `Process` exposes repeatable `Wait`,
  `Signal`, `Kill`, and lifecycle-reference `Export` operations.
- `local.Executor` implements `Executor` with `os/exec`.
- `RunAll` adds bounded concurrent execution without changing result order.
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

The local executor inherits the current environment, applies `Command.Env`
overrides, honors `Dir` and `Stdin`, and always captures stdout and stderr.
Configured output writers receive the same data while the command runs.

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
  Canceling it terminates the direct child. `Run` still waits for the child to
  be reaped before it returns.
- A context passed to `Wait` controls only that call. A canceled or timed-out
  wait does not terminate the process; a later `Wait` can return the final
  result. `Wait` is repeatable and local waits return independently owned
  output slices.
- `Signal` accepts exact uppercase names. Unix-like builds support `TERM`,
  `KILL`, `INT`, and `HUP`; Windows supports `KILL`. `Kill` sends the platform's
  unconditional kill operation. Both methods reject an already-canceled
  operation context before signaling.
- Process control targets the direct child. The local implementation does not
  manage Unix process groups or Windows Job Objects.

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
escalation, process-tree supervision, SSH/Kubernetes transport, sandbox, or
durable execution in this module.

## Development

Go 1.22 or newer is required. Useful focused checks are:

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
