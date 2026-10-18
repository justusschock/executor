# ADR 0008: Process reference export format

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 (FR-EXEC-16, FR-EXEC-17, NFR-EXEC-05), ADR-0003 |

## Context

Recorded retrospectively. Supervisors and lifecycle tooling want a record
of which process a component started, for example to hand it to another
component or to report it. Commands often contain secrets in arguments,
environment and stdin. A PID alone can be reused after the process exits,
and the exporting component still owns the process.

`Process.Export` on a live local process returns a `handoff.Artifact`
(go-lifecycle) with:

- media type `application/vnd.justusschock.process-reference+json`,
  version `1`, metadata `kind=local-process-reference`;
- JSON data with exactly `pid`, `path` and `started_at` (RFC 3339 with
  nanoseconds, UTC).

It returns `ErrNotStarted` for a process that never started and
`ErrProcessExited` once the process has been reaped (checked before and
after encoding). `Remote.Adopt` always returns `ErrUnsupportedAdoption`.

## Decision drivers

- D1 Never leak command material in an exported reference.
- D2 Make the format versioned and self-describing.
- D3 Do not pretend that a reference transfers custody.

## Considered options

1. **Minimal versioned JSON in a `handoff.Artifact`; no adoption in the core
   module**
   - Pro: integrity (ID and digest) and versioning come from the shared
     envelope; no secrets; honest about custody.
   - Con: a reference cannot prove the PID still identifies the same
     process.
2. **Include arguments and environment for reproducibility**
   - Con: leaks secrets into logs and stores.
3. **Implement adoption (re-attaching by PID)**
   - Con: unsafe with PID reuse and impossible to reap a non-child on most
     platforms; belongs in an adapter that controls custody.

## Decision

We will export live local processes as version-1 process-reference
artifacts with only `pid`, `path` and `started_at`, keep custody with the
exporter, and leave adoption to separate adapters (`Remote.Adopt` returns
`ErrUnsupportedAdoption`). Changing the fields or their meaning requires a
new version number and an ADR.

## Consequences

### Positive
- References are safe to log and store.

### Negative
- Consumers must treat references as advisory and possibly stale.

### Follow-ups
- None.

## Links

- [PRD-0001](../prd/0001-go-executor.md)
