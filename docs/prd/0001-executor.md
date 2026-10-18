# PRD 0001: executor

| Field | Value |
|---|---|
| Status | Accepted |
| Implementation | Complete |
| Owners | Justus Perillieux |
| Related ADRs | ADR-0001 to ADR-0013 |
| Last updated | 2026-10-18 |

Recorded retrospectively for the first public release; describes the
library as implemented.

## 1. Problem and context

Programs that orchestrate work (build tools, pipeline runners, training
launchers, job agents) start external commands, stream and capture their
output, bound how long they run, stop them together with everything they
started, and report what happened. `os/exec` provides the mechanism but
leaves many decisions to every caller: how cancellation escalates, whether a
shell's grandchildren are stopped, how much output is retained, how output
from parallel commands is kept readable, how a non-zero exit is reported
together with its output, and how the same calling code can run commands
locally or through a remote transport.

executor answers these once, behind small contracts: a `Command` value, an
`Executor`/`Process` interface pair, a local `os/exec` implementation, and
helpers for shells, bounded parallel runs, prefixed output and remote
adapters.

## 2. Users and use cases

- As a **tool author**, I want to run a program with arguments, a working
  directory, an environment and optional stdin, and get its exit code and
  captured output, so that I do not re-implement `os/exec` plumbing.
- As a **pipeline author**, I want to run a shell script and have
  cancellation stop every program the script started, so that a cancelled
  step leaves no orphans.
- As an **operator of long-running jobs**, I want cancellation to ask a
  process to terminate and force-kill it after a grace period, so that
  programs can clean up but cannot hang shutdown.
- As a **CLI author**, I want to stream output from several concurrent
  commands to one terminal with a label per command and without interleaved
  lines.
- As a **batch runner**, I want to run many commands with bounded
  concurrency, get results in input order, and stop the batch on the first
  failure.
- As a **transport author**, I want to implement one `Start` method and get
  a validated, owned command and a full `Executor`, so that callers can switch
  between local and remote execution.
- As a **supervisor**, I want to export a reference to a live process for
  bookkeeping without leaking its arguments or environment.

## 3. Goals and non-goals

### Goals
- G1 Portable, implementation-neutral contracts for commands, processes and
  results in the root package, with no operating-system process code.
- G2 A local implementation for Windows and the common Unix-like systems.
- G3 Predictable cancellation: the start context owns the process lifetime;
  wait contexts bound only the wait.
- G4 Bounded memory for captured output without losing streamed output.
- G5 Errors that carry the full result and keep their causes inspectable.

### Non-goals
- NG1 Network transports (SSH, Kubernetes, RPC). The `Starter` port lets
  adapters provide them outside this module.
- NG2 PTYs, sandboxing, privilege changes and resource limits.
- NG3 Implicit shell interpretation. A shell is used only through `Shell` or
  `ShellWith`.
- NG4 Durable execution or re-attaching to processes after a restart;
  exported references do not transfer custody.
- NG5 Retries and backoff; callers compose them around `Run`.

## 4. Functional requirements

| ID | Requirement | Priority (MUST/SHOULD/MAY) |
|---|---|---|
| FR-EXEC-01 | `Command` describes path, arguments, directory, environment, `ClearEnv`, optional stdin/stdout/stderr, per-stream capture limits, `ProcessGroup` and `KillGrace`. `Validate` rejects an empty path, NUL bytes in path, arguments, directory or environment, empty or `=`-containing environment keys, negative limits and a negative grace period with `ErrInvalidCommand`, and typed-nil streams with `ErrNilDependency`. Executors validate before starting anything. | MUST |
| FR-EXEC-02 | `Command.Clone` copies `Args` and `Env` and keeps stream references; nil collections stay nil. Executors and `RunAll` clone before execution so later caller mutations have no effect. | MUST |
| FR-EXEC-03 | `local.Executor` runs the program directly with `os/exec`, honours `Dir` and `Stdin`, always captures stdout and stderr, and forwards the same bytes to configured writers while the command runs. Writes from both streams to a shared destination are serialized. | MUST |
| FR-EXEC-04 | By default the child inherits the parent environment with `Env` entries overriding it (case-insensitively on Windows, deterministic order). With `ClearEnv`, only `Env` entries are supplied, and an empty `Env` yields an empty environment apart from values the platform adds. | MUST |
| FR-EXEC-05 | `MaxStdoutBytes`/`MaxStderrBytes` bound retained output per stream: zero keeps everything, a positive limit keeps the prefix, sets `StdoutTruncated`/`StderrTruncated` only when bytes were dropped, keeps draining the child and forwards every byte to the configured writer. | MUST |
| FR-EXEC-06 | `Result` reports exit code, start and finish time, captured output and truncation flags. A non-zero exit or wait failure (including a failing destination writer) returns the populated `Result` and an `*ExitError` that holds the same result and whose cause is reachable through `errors.Is`/`errors.As`. Sentinel errors `ErrInvalidCommand`, `ErrInvalidSignal`, `ErrNotStarted`, `ErrProcessExited`, `ErrNilDependency` and `ErrUnsupportedAdoption` are stable API. | MUST |
| FR-EXEC-07 | The context passed to `Start` owns the child's lifetime; its end terminates the child and its cause is discoverable from the final error. A context passed to `Wait` bounds only that call. `Wait` is repeatable and every call returns independently owned output. `Run` waits until the child has been reaped. | MUST |
| FR-EXEC-08 | `Signal` accepts exact uppercase names (`TERM`, `KILL`, `INT`, `HUP` on Unix-like systems; `KILL` on Windows) and rejects others with `ErrInvalidSignal`; `Kill` stops the process unconditionally. Both return an already-ended operation context's cause without signalling. | MUST |
| FR-EXEC-09 | With `ProcessGroup`, the child leads a new process group (Unix) or process group plus Job Object (Windows, `taskkill /T` fallback), and `Signal`, `Kill` and start-context cancellation reach its descendants. After the group has been reaped they return `os.ErrProcessDone`. | MUST |
| FR-EXEC-10 | On start-context cancellation, a zero `KillGrace` kills immediately; a positive grace sends `TERM`, then `KILL` when the process has not been reaped by the end of the grace period. Platforms without `TERM` kill immediately. | MUST |
| FR-EXEC-11 | `Shell` and `ShellWith` build commands that run a script through `/bin/sh -c` (`/system/bin/sh -c` on Android, `cmd.exe /d /s /c` on Windows) or a chosen interpreter, with per-field fallback to `DefaultShell`, `ProcessGroup` set and `KillGrace = DefaultKillGrace`. On Windows the script is passed verbatim inside one pair of quotes. | MUST |
| FR-EXEC-12 | `Result.Usage` reports user and system CPU time on every platform, peak RSS in bytes on Unix-like systems, and storage read/write bytes on Linux and Android, for successful runs and `ExitError` results. Unknown values are zero. | SHOULD |
| FR-EXEC-13 | `NewPrefixWriter` prefixes every line, writes only complete lines to the destination, serializes all prefix writers that share a destination, emits partial lines longer than `MaxPrefixLineBytes` in chunks, flushes a trailing partial line on `Close`, is idempotent on `Close`, rejects writes after `Close` with `io.ErrClosedPipe`, and reports destination errors and short writes. | MUST |
| FR-EXEC-14 | `RunAll` runs commands with at most `limit` concurrent `Run` calls, returns results in input order, and on the first failure stops admission, cancels admitted peers, waits for them and returns no results with index-labelled errors joined in admission order. A nil executor returns `ErrNilDependency`; a non-positive limit returns `concurrency.ErrInvalidLimit`; parent cancellation propagates. | MUST |
| FR-EXEC-15 | `NewRemote` rejects a nil or typed-nil `Starter`. `Remote.Start` clones and validates the command and forwards it with the exact context exactly once, returning the starter's process and error unchanged. `Remote.Run` performs one `Start` and one `Wait` with the same context and returns their results unchanged. | MUST |
| FR-EXEC-16 | `Process.Export` of a live local process returns a valid `handoff.Artifact` with media type `application/vnd.justusschock.process-reference+json`, version 1, metadata `kind=local-process-reference`, and JSON data with exactly `pid`, `path` and `started_at` (RFC 3339, UTC). It omits arguments, environment and streams, does not stop the process or transfer custody, and returns the context cause, `ErrNotStarted` or `ErrProcessExited` as applicable. | MUST |
| FR-EXEC-17 | `Remote.Adopt` returns `ErrUnsupportedAdoption`; adopting exported references is out of scope for the core module. | MUST |

## 5. Non-functional requirements

| ID | Requirement | Measure |
|---|---|---|
| NFR-EXEC-01 | Concurrency safety: `local.Executor`, `Remote` (given a safe `Starter`), `Process` methods and prefix writers are safe for concurrent use. | `go test -race -count=3 ./...` passes. |
| NFR-EXEC-02 | Portability: the root package contains no OS process code; `local` builds on Windows, aix, android, darwin, dragonfly, freebsd, linux, netbsd, openbsd and solaris. | CI `cross` job runs `go vet` for each target. |
| NFR-EXEC-03 | Dependencies: standard library plus `concurrency` and `lifecycle` only. | `go.mod` require list. |
| NFR-EXEC-04 | Minimum Go version 1.25 (for `testing/synctest`); no `replace` directives or committed `go.work`. | `TestRepositoryCompatibilityPolicy`; CI matrix `1.25.x`, `oldstable`, `stable`. |
| NFR-EXEC-05 | Security: no implicit shell; exported references carry no arguments, environment or input; the library never logs. | `TestExportLiveProcessReference`; code review. |
| NFR-EXEC-06 | Determinism: time-dependent logic that does not involve real processes is tested with virtual time. | Escalation and `RunAll` tests run in `synctest` bubbles. |

## 6. API sketch

```go
package executor

type Command struct {
	Path string; Args []string; Dir string; Env map[string]string
	ClearEnv bool
	Stdin io.Reader; Stdout, Stderr io.Writer
	MaxStdoutBytes, MaxStderrBytes int64
	ProcessGroup bool; KillGrace time.Duration
}
func (Command) Validate() error
func (Command) Clone() Command

type Executor interface {
	Run(context.Context, Command) (Result, error)
	Start(context.Context, Command) (Process, error)
}
type Process interface {
	Wait(context.Context) (Result, error)
	Signal(context.Context, string) error
	Kill(context.Context) error
	Export(context.Context) (handoff.Artifact, error)
}
type Result struct { ExitCode int; StartedAt, FinishedAt time.Time; Stdout, Stderr []byte
	StdoutTruncated, StderrTruncated bool; Usage Usage }
type ExitError struct { Result Result; Cause error }

func Shell(script string) Command
func ShellWith(ShellOptions, string) Command
func DefaultShell() ShellOptions
func RunAll(ctx context.Context, runner Executor, commands []Command, limit int) ([]Result, error)
func NewPrefixWriter(w io.Writer, prefix string) io.WriteCloser

type Starter interface { Start(context.Context, Command) (Process, error) }
func NewRemote(Starter) (*Remote, error)

package local

func New() *Executor // implements executor.Executor
```

## 7. Acceptance criteria

| Criterion | Requirement(s) | Test(s) |
|---|---|---|
| Valid commands pass; each invalid field and typed-nil stream is rejected before start | FR-EXEC-01 | `TestCommandValidateAcceptsValidCommand`, `TestCommandValidateRejectsInvalidFields`, `TestCommandValidateRejectsTypedNilStreams`, `TestStartRejectsTypedNilStreamsBeforeStarting`, `TestStartValidatesAndOwnsCommand` |
| Clone and executors own `Args`/`Env` | FR-EXEC-02 | `TestCommandCloneCopiesMutableFields`, `TestCommandClonePreservesNilCollections`, `TestStartValidatesAndOwnsCommand` |
| Local runs stream and capture, honour directory and stdin, serialize shared destinations | FR-EXEC-03 | `TestRunStreamsAndCaptures`, `TestRunUsesDirectoryAndStdin`, `TestSharedDestinationWritesAreSerializedAcrossStreams` |
| Environment inheritance, overrides and `ClearEnv` | FR-EXEC-04 | `TestRunEnvironmentInheritance`, `TestRunClearEnvironmentAllowsEmptyExplicitEnvironment`, `TestBuildEnvironmentSortsOverridesAndReplacesPlatformKeys`, `TestBuildEnvironmentCollapsesCaseEquivalentWindowsOverrides`, `TestBuildEnvironmentWithoutInheritanceSortsAndFoldsOverrides` |
| Bounded capture and exact truncation flags | FR-EXEC-05 | `TestRunBoundsCaptureWithoutTruncatingDestinations`, `TestRunDoesNotReportExactLimitAsTruncated` |
| Exit errors carry result and cause; sentinels exist | FR-EXEC-06 | `TestRunReturnsPopulatedExitError`, `TestExitErrorPreservesCauseAndResult`, `TestExitErrorWithoutCause`, `TestCallerWriterErrorIsWaitErrorAndCaptureRemainsOwned`, `TestStableErrorsExist` |
| Start context owns lifetime; Wait is repeatable, owned and bounded per call | FR-EXEC-07 | `TestWaitIsRepeatableAndReturnsOwnedResults`, `TestExitWaitClonesResultAndExitErrorOutput`, `TestWaitCancellationDoesNotStopProcess`, `TestStartAndRunCancellationReturnDiscoverableExitError` |
| Signal names and Kill per platform | FR-EXEC-08 | `TestSignalAndKill`, `TestPlatformSignals` |
| Process groups reach grandchildren | FR-EXEC-09 | `TestProcessGroupCancellationKillsGrandchildren`, `TestProcessGroupSignalAndKillReachGrandchildren` |
| Kill-grace escalation | FR-EXEC-10 | `TestEscalationKillsImmediatelyWithoutGrace`, `TestEscalationKillsAfterGrace`, `TestEscalationStopsWhenProcessExitsDuringGrace`, `TestEscalationKillsImmediatelyWithoutTerminateSignal`, `TestEscalationReturnsTerminateFailureWithoutKilling`, `TestProcessGroupCancellationKillsGrandchildren` |
| Shell commands and Windows command lines | FR-EXEC-11 | `TestDefaultShellMatchesPlatform`, `TestShellUsesDefaultShellAndProcessGroup`, `TestShellWithFallsBackPerField`, `TestShellWithOwnsArguments`, `TestShellRunsScriptsThroughShell`, `TestWindowsShellCommandLine` |
| Resource usage for success and exit errors | FR-EXEC-12 | `TestRunReportsResourceUsage` |
| Prefix writer behaviour | FR-EXEC-13 | `TestPrefixWriterBuffersLinesAndFlushesOnClose`, `TestPrefixWriterCloseWithoutPendingWritesNothing`, `TestPrefixWritersSharingDestinationNeverInterleaveLines`, `TestPrefixWriterIsSafeForConcurrentWrites`, `TestPrefixWriterBoundsPartialLines`, `TestPrefixWriterReportsDestinationErrors`, `TestPrefixWriterNestsAndAcceptsIncomparableDestinations` |
| Bounded, ordered, fail-fast batches | FR-EXEC-14 | `TestRunAllPreservesOrderAndBoundsConcurrency`, `TestRunAllReturnsIndexedErrorWithoutPartialResults`, `TestRunAllCancelsPeersStopsAdmissionAndJoinsIndexedErrors`, `TestRunAllOwnsCommandsBeforeExecution`, `TestRunAllRejectsInvalidDependenciesAndLimits`, `TestRunAllPropagatesCancellation` |
| Remote adapter forwarding | FR-EXEC-15 | `TestNewRemoteRejectsNilStarter`, `TestRemoteStartValidatesBeforeForwarding`, `TestRemoteStartForwardsOwnedCommandExactlyOnce`, `TestRemoteRunStartsAndWaitsExactlyOnce`, `TestRemoteRunReturnsStartErrorWithoutWaiting` |
| Process reference export | FR-EXEC-16, NFR-EXEC-05 | `TestExportLiveProcessReference`, `TestExportRejectsCanceledContext`, `TestExportRejectsUnavailableProcess`, `TestExportRejectsExitedProcess` |
| Adoption unsupported | FR-EXEC-17 | `TestRemoteAdoptionIsUnsupported` |
| Go floor and module hygiene | NFR-EXEC-04 | `TestRepositoryCompatibilityPolicy` |

## 8. Source references

None.

## 9. Open questions

- Q1 Should a public conformance suite for `Executor`/`Process`
  implementations be added once a second implementation (a remote adapter)
  exists outside this module?
- Q2 Should Windows report peak memory and I/O counters, which would require
  querying the process before its handle is closed?
