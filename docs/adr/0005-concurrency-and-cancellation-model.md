# ADR 0005: Concurrency and cancellation model

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 (FR-EXEC-07, FR-EXEC-08, FR-EXEC-14, NFR-EXEC-01), ADR-0006 |

## Context

Recorded retrospectively. A started process outlives any single call that
observes it. Callers want to bound how long they wait for a result without
killing the process, and separately to bound how long the process may run.
`os/exec` couples both to one context (`CommandContext`) and allows `Wait`
only once.

The local implementation:

- passes the `Start` context to `exec.CommandContext`, so its end runs the
  cancellation hook (`cmd.Cancel`, see ADR-0006);
- reaps the child exactly once in a goroutine whose result is held in a
  `concurrency.Future`; `Wait(ctx)` awaits that future and returns the wait
  context's cause if it ends first, leaving the process running;
- has `Run` call `Start` and then `Wait(context.WithoutCancel(ctx))`, so
  `Run` returns only after the child has been reaped even when `ctx` ends;
- checks the operation context before `Signal`, `Kill` and `Export`;
- clones captured output on every `Wait` so results never alias.

`RunAll` uses `concurrency.ParallelMap`: at most `limit` concurrent `Run`
calls, results in input order, first failure stops admission and cancels
admitted peers through their contexts.

## Decision drivers

- D1 Waiting and running must be bounded independently.
- D2 No goroutine or zombie process may leak; every child is reaped.
- D3 Repeated or concurrent `Wait` calls must be safe and return the same
  outcome.
- D4 Batch failures should stop work quickly but report every admitted
  failure.

## Considered options

1. **Start context owns the lifetime; per-call contexts bound only the call;
   single reaper with a shared future**
   - Pro: callers can poll with short wait timeouts without affecting the
     process.
   - Pro: `Wait` is idempotent and safe for concurrent use.
   - Con: a caller who wants "wait at most N, then kill" must cancel the
     start context explicitly.
2. **One context for both, as in `exec.CommandContext` + `Wait`**
   - Pro: simplest.
   - Con: a timed-out wait kills the process; `Wait` cannot be repeated.
3. **No context on `Start`; explicit `Kill` only**
   - Con: cancellation would not propagate through call trees, and `RunAll`
     could not cancel peers.

## Decision

We will let the `Start` context own the process lifetime, make contexts
passed to `Wait`, `Signal`, `Kill` and `Export` bound only that call, reap
each child once into a shared future, and have `Run` return only after the
reap. `RunAll` delegates admission, ordering and peer cancellation to
`concurrency.ParallelMap`.

## Consequences

### Positive
- Predictable, documented ownership; cancellation causes stay visible in
  `ExitError` (ADR-0004).

### Negative
- Callers must keep the start context alive for as long as the process
  should run; using a request-scoped context for a long-lived process stops
  it when the request ends.

### Follow-ups
- None.

## Links

- [PRD-0001](../prd/0001-go-executor.md)
- [ADR-0006](0006-process-groups-and-kill-escalation.md)
