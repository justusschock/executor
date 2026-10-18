# ADR 0006: Opt-in process groups and kill-grace escalation

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 (FR-EXEC-09, FR-EXEC-10, FR-EXEC-11), ADR-0005, ADR-0010 |

## Context

Recorded retrospectively. Signalling only the direct child leaves the
programs a shell started running after the shell is killed. Killing
immediately gives programs no chance to flush or clean up, while waiting
indefinitely for a graceful exit can hang shutdown. Platform mechanisms
differ: Unix has process groups and `SIGTERM`; Windows has Job Objects, no
portable terminate signal, and `taskkill /T` as a tree-kill tool.

The code adds two `Command` fields:

- `ProcessGroup`: on Unix-like systems `Setpgid` and signals to `-pgid`
  (`ESRCH` maps to `os.ErrProcessDone`); on Windows
  `CREATE_NEW_PROCESS_GROUP` plus assignment to a Job Object that is
  terminated on kill, falling back to `taskkill /T /F` when the job cannot be
  created or assigned. After the reap, group operations return
  `os.ErrProcessDone`.
- `KillGrace`: when the start context ends, zero kills immediately; a
  positive value sends `TERM` and starts a timer that kills unless the
  process is reaped first. Where `TERM` is unsupported
  (`ErrInvalidSignal`), it kills immediately.

Both zero values preserve the earlier behaviour (direct child, immediate
kill). `Shell` sets `ProcessGroup` and `DefaultKillGrace` (10 s).

## Decision drivers

- D1 Cancelling a shell command must stop everything it started.
- D2 Programs should get a chance to exit cleanly.
- D3 Existing callers must see no behaviour change.
- D4 Windows must get the strongest tree control available without extra
  dependencies.

## Considered options

1. **Opt-in fields on `Command`, with platform-specific implementations**
   - Pro: zero values are backward compatible; shells opt in by default.
   - Con: different mechanisms per platform with slightly different
     guarantees (no graceful terminate on Windows).
2. **Always use a process group**
   - Con: changes terminal job-control behaviour (a group child is not in
     the foreground group) for callers that pass a terminal as stdin.
3. **Leave tree control to callers**
   - Con: every caller re-implements platform code; most would not.

## Decision

We will provide `ProcessGroup` and `KillGrace` as opt-in `Command` fields,
implement them per platform as described above, and have shell commands
opt in. The escalation logic is isolated in an unexported `escalation` type
so that it can be tested with virtual time.

## Consequences

### Positive
- Cancelled shell scripts leave no orphans; programs get a grace period.

### Negative
- Unix group children cannot read from the controlling terminal.
- Windows always kills without a grace period.

### Follow-ups
- None.

## Links

- [PRD-0001](../prd/0001-go-executor.md)
