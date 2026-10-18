# ADR 0009: Remote execution through a Starter port

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 (FR-EXEC-15, FR-EXEC-17), ADR-0002, ADR-0004 |

## Context

Recorded retrospectively. Commands may need to run elsewhere: on another
host, in a container, through an agent. Transports differ widely in
authentication, sessions and custody, and each brings heavy dependencies.
The library should let such transports plug in without becoming one.

The root package defines `Starter` (one method, `Start`) and `Remote`, which
turns any `Starter` into an `Executor`: `Start` clones and validates the
command and forwards it with the exact context once; `Run` is one `Start`
plus one `Wait` with the same context; results and errors pass through
unchanged; `Adopt` is unsupported. There is no conformance suite; behaviour
of the returned `Process` is the adapter's responsibility.

## Decision drivers

- D1 No transport code or transport dependencies in this module.
- D2 Adapters should implement as little as possible.
- D3 Every executor, local or remote, must see validated, owned commands.

## Considered options

1. **One-method `Starter` port plus a `Remote` adapter in the root package**
   - Pro: adapters implement `Start` only; validation and ownership are
     shared.
   - Con: adapters still implement the full `Process` interface.
2. **Built-in transports (SSH, Kubernetes)**
   - Con: large dependencies and policies outside the library's scope.
3. **Adapters implement `Executor` directly**
   - Con: each adapter re-implements validation, cloning and `Run`.
4. **Ship a public conformance suite for adapters**
   - Pro: adapters could prove they meet the `Process` contract.
   - Con: no second implementation exists in this module to exercise it;
     left as an open question (PRD-0001 Q1).

## Decision

We will expose `Starter` and `Remote` in the root package and keep
transports in separate modules. `Remote` forwards validated, owned commands
and returns adapter results unchanged.

## Consequences

### Positive
- Calling code is identical for local and remote execution.

### Negative
- Process semantics (wait contexts, signals) of remote adapters are not
  enforced by tests in this module.

### Follow-ups
- Revisit a conformance suite when external adapters exist.

## Links

- [PRD-0001](../prd/0001-executor.md)
