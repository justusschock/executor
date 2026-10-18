# ADR 0011: Public API boundary

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001, ADR-0002, ADR-0009 |

## Context

Recorded retrospectively while preparing the first public release. Every
exported identifier becomes a compatibility promise. The review covered
every package and exported identifier and checked which ones downstream
modules use (a command-line tool and a run package in a dependent
application use `Command`, `Executor`, `ExitError`, `Usage`, `Shell`,
`ShellWith`, `ShellOptions`, `DefaultShell`, `NewPrefixWriter` and
`local.New`).

Public surface after the review:

- `executor` (root): `Command` (`Validate`, `Clone`), `DefaultKillGrace`,
  `Executor`, `Process`, `Result`, `Usage`, `ExitError`, the six sentinel
  errors, `Shell`, `ShellWith`, `ShellOptions`, `DefaultShell`, `RunAll`,
  `NewPrefixWriter`, `MaxPrefixLineBytes`, `Starter`, `Remote`
  (`Start`, `Run`, `Adopt`), `NewRemote`.
- `local`: `Executor` (`Start`, `Run`) and `New`.

Implementation details were already unexported: the local `process` type and
its reaper, `escalation`, `processGroup` and its platform variants,
`lockedBuffer`, `serializedWriter`, `buildEnvironment`,
`windowsShellCommandLine`, the process-reference struct and media-type
constant, rusage conversion, the prefix-writer lock registry, and
`interfaceIsNil`.

## Decision drivers

- D1 The public API should be exactly what callers and adapter authors need.
- D2 Nothing a consumer uses may be removed.
- D3 Platform code must not leak into the API.

## Considered options

1. **Keep the current two-package surface unchanged; no `internal/`
   packages**
   - Pro: every exported identifier is part of a documented contract; no
     consumer breaks.
   - Con: none identified.
2. **Move `local` under `internal/` and expose a constructor from the root**
   - Con: the root package would depend on OS process code, contradicting
     ADR-0002, and consumers import `local` directly.
3. **Unexport `MaxPrefixLineBytes` and `DefaultKillGrace`**
   - Con: both document observable behaviour (chunk size, shell default)
     that callers may rely on; `DefaultKillGrace` is part of `Shell`'s
     contract.

## Decision

We will keep the public API as listed above. No package is moved to
`internal/` and no identifier is unexported or removed. Every exported
identifier is documented. There is no clock abstraction to remove: the
code uses `time` directly and tests use `testing/synctest` (ADR-0013).
Future implementation-only packages go under `internal/`.

## Consequences

### Positive
- The first public release breaks no existing consumer.

### Negative
- `Remote.Adopt` stays public although it always fails, because it makes
  `Remote` satisfy `handoff.Adopter[Process]`.

### Follow-ups
- Any change to the listed surface needs an ADR and a changelog entry.

## Links

- [PRD-0001](../prd/0001-go-executor.md)
