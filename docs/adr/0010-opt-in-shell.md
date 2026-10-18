# ADR 0010: Direct execution by default, shells only on request

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 (FR-EXEC-11, NFR-EXEC-05), ADR-0006 |

## Context

Recorded retrospectively. Running programs through a shell enables pipes,
redirection and globbing, but interpolating input into a script is an
injection risk, and shells start further processes that outlive a killed
shell. Windows `cmd.exe` does not understand the backslash escaping that Go
applies to arguments, so a script passed as a normal argument can be
mangled.

`Command` always runs `Path` directly with `Args`. `Shell(script)` and
`ShellWith(options, script)` build a `Command` for `/bin/sh -c`
(`/system/bin/sh -c` on Android), `cmd.exe /d /s /c` on Windows, or a chosen
interpreter, with each zero `ShellOptions` field falling back to
`DefaultShell` independently. Shell commands set `ProcessGroup` and
`DefaultKillGrace`. On Windows the local executor passes a `cmd.exe … /s /c`
script verbatim inside one pair of quotes (`SysProcAttr.CmdLine`).

## Decision drivers

- D1 No implicit shell interpretation of caller data.
- D2 Shell scripts are common in pipelines and should be one call away.
- D3 Cancelling a script must reach its children (ADR-0006).

## Considered options

1. **Direct execution; explicit `Shell`/`ShellWith` constructors that return
   ordinary commands**
   - Pro: shell use is visible at the call site; the result is a normal
     `Command` that callers can adjust.
   - Con: callers who want shell features must choose them explicitly.
2. **A `Shell bool` field on `Command`**
   - Con: hides the interpreter choice and its arguments; harder to
     customize (`bash -euo pipefail -c`).
3. **Always run through a shell**
   - Con: injection risk and extra processes for every command.

## Decision

We will execute programs directly and provide `Shell`, `ShellWith` and
`DefaultShell` to build shell commands that opt into process groups and a
kill grace period, with verbatim script passing for `cmd.exe /s /c`.

## Consequences

### Positive
- Safe default; scripts are easy and cancellable.

### Negative
- Callers must never interpolate untrusted input into scripts; this is
  documented but cannot be enforced.

### Follow-ups
- None.

## Links

- [PRD-0001](../prd/0001-go-executor.md)
