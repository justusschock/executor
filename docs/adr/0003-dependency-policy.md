# ADR 0003: Dependency policy

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 (NFR-EXEC-03), ADR-0005, ADR-0009 |

## Context

Recorded retrospectively. executor depends on the standard library and
two sibling modules:

- `github.com/justusschock/concurrency`: `ParallelMap` implements
  `RunAll`'s bounded, ordered, fail-fast admission, and `Future` holds each
  local process's single reap result so that `Wait` can be called any number
  of times with independent contexts.
- `github.com/justusschock/lifecycle/handoff`: `handoff.Artifact` is the
  envelope for exported process references, and `handoff.Adopter` is the
  interface `Remote.Adopt` satisfies.

There are no third-party dependencies. Windows Job Objects are reached
through `syscall.NewLazyDLL` rather than `golang.org/x/sys/windows`.

## Decision drivers

- D1 A process library is a low-level dependency; its own dependency graph
  should be minimal.
- D2 Do not re-implement bounded parallelism or once-only results that a
  sibling library already provides and tests.
- D3 Exported process references should use the same envelope as other
  hand-off artifacts in the library family.

## Considered options

1. **Standard library plus the two sibling modules**
   - Pro: shared, tested primitives for concurrency and hand-off; no
     third-party code.
   - Con: users pull in two more modules.
2. **Standard library only**
   - Pro: zero dependencies.
   - Con: duplicates `ParallelMap`/`Future` logic and invents a separate
     artifact format.
3. **Use `golang.org/x/sys` for platform calls**
   - Pro: typed wrappers for Job Objects.
   - Con: a large dependency for three kernel32 calls.

## Decision

We will depend on the standard library, concurrency and lifecycle
only. A new dependency, including another sibling module, needs an ADR.

## Consequences

### Positive
- Small, auditable dependency graph.
- `RunAll` inherits concurrency's documented admission and error-joining
  semantics.

### Negative
- Errors such as `concurrency.ErrInvalidLimit` and the `handoff.Artifact`
  type appear in this module's API, so major changes in the sibling modules
  propagate here.

### Follow-ups
- None.

## Links

- [concurrency](https://github.com/justusschock/concurrency)
- [lifecycle](https://github.com/justusschock/lifecycle)
