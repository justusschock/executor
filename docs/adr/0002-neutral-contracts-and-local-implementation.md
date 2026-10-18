# ADR 0002: Neutral root contracts with a separate local implementation

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 (FR-EXEC-01, FR-EXEC-02, FR-EXEC-07), ADR-0009, ADR-0011 |

## Context

Recorded retrospectively. Callers want to write orchestration code once and
run commands either on the local machine or through some remote mechanism.
The root package `executor` declares a `Command` value type, the `Executor`
(`Run`, `Start`) and `Process` (`Wait`, `Signal`, `Kill`, `Export`)
interfaces, the `Result`/`Usage`/`ExitError` types, and helpers built only on
those contracts (`Shell`, `RunAll`, `NewPrefixWriter`, `Remote`). All
operating-system process code lives in the `local` package, whose `Executor`
implements the root interface with `os/exec`.

## Decision drivers

- D1 Callers should depend on contracts, not on `os/exec`.
- D2 The contracts must compile on every platform, including ones the local
  implementation does not support.
- D3 A command must be inspectable, copyable and comparable in tests.
- D4 Small surface: few types, no configuration framework.

## Considered options

1. **Root contracts plus a `local` implementation package; `Command` as a
   plain struct**
   - Pro: callers and adapters import only the root package; build tags and
     syscalls stay in `local`.
   - Pro: a struct literal is self-documenting; zero values are the
     compatible defaults (inherit environment, retain all output, direct
     child only, kill immediately).
   - Con: new fields extend the struct; callers using unkeyed literals would
     break (none are expected).
2. **One package with the local implementation and the contracts together**
   - Pro: fewer imports.
   - Con: every importer pulls in process and syscall code; adapters cannot
     depend on contracts alone.
3. **Functional options (`Run(ctx, path, WithArgs(...), WithEnv(...))`)**
   - Pro: extensible without struct changes.
   - Con: commands are no longer values that can be validated, cloned,
     stored and forwarded to a remote `Starter` unchanged.

## Decision

We will keep the implementation-neutral contracts in the root package and
the `os/exec` implementation in `local`. A command is the `Command` struct
with `Validate` and `Clone`; executors clone and validate before execution.
Process control is the `Process` interface returned by `Start`; `Run` is
`Start` followed by `Wait`.

## Consequences

### Positive
- The root package has no OS process code and no build tags except the
  default shell selection.
- Commands can be built, inspected and forwarded as values.

### Negative
- Adding a `Command` field is an API change that every implementation must
  honour or document as unsupported.

### Follow-ups
- A public conformance suite for `Executor`/`Process` implementations is an
  open question in PRD-0001 (Q1).

## Links

- [PRD-0001](../prd/0001-executor.md)
