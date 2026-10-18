# ADR 0004: Error semantics

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 (FR-EXEC-06, FR-EXEC-07, FR-EXEC-14, FR-EXEC-15) |

## Context

Recorded retrospectively. A failed command still has useful output: its
exit code, captured stdout and stderr, timing and resource usage. Callers
also need to tell apart invalid input, unsupported signals, a cancelled or
expired start context, a failing output writer and a non-zero exit.

The code returns:

- sentinel errors (`ErrInvalidCommand`, `ErrInvalidSignal`, `ErrNotStarted`,
  `ErrProcessExited`, `ErrNilDependency`, `ErrUnsupportedAdoption`), wrapped
  with `%w` and a detail message;
- `*ExitError{Result, Cause}` for non-zero exits and wait failures, returned
  *together with* the populated `Result`; for local runs `Cause` joins the
  start context's cause with the `os/exec` wait error;
- index-labelled errors (`command N: …`) joined in admission order from
  `RunAll`;
- the exact error values of a `Starter` and its processes from `Remote`,
  without wrapping.

## Decision drivers

- D1 The result of a failed run must not be lost.
- D2 Callers use `errors.Is`/`errors.As`, never string matching.
- D3 Cancellation must remain discoverable after the process is reaped.
- D4 Adapters must not obscure transport errors.

## Considered options

1. **Result plus `*ExitError` that also carries the result; sentinels for
   input and state errors**
   - Pro: `if err != nil` callers still get output through the return value;
     `errors.As` callers get it from the error.
   - Pro: cause chains keep `context.Canceled`/`DeadlineExceeded` visible.
   - Con: two copies of the result (cloned so neither aliases the other).
2. **Return `(Result, nil)` for non-zero exits; error only for start
   failures**
   - Pro: mirrors shell semantics.
   - Con: easy to ignore failures; writer and cancellation failures need
     another channel.
3. **Return `os/exec`'s `*exec.ExitError` directly**
   - Con: ties the contract to `os/exec`; remote adapters cannot produce it.

## Decision

We will return the populated `Result` alongside an `*ExitError` for every
non-zero exit or wait failure, expose sentinel errors wrapped with `%w`,
keep start-context causes in the `ExitError` cause chain, label `RunAll`
failures by command index, and forward adapter errors unchanged.

## Consequences

### Positive
- No output is lost on failure; errors compose with the standard `errors`
  package.

### Negative
- A destination writer error turns an otherwise successful exit into an
  `ExitError`; this is documented.

### Follow-ups
- None.

## Links

- [PRD-0001](../prd/0001-executor.md)
