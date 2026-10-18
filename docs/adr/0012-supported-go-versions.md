# ADR 0012: Supported Go versions

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 (NFR-EXEC-04), ADR-0013 |

## Context

Recorded retrospectively. The module once supported Go 1.22 through 1.26.
Its escalation tests use `testing/synctest`, whose stable API
(`synctest.Test`, `synctest.Wait`) first shipped in Go 1.25, so `go.mod`
already declares `go 1.25`. The Go project supports the two most recent
major releases.

## Decision drivers

- D1 Tests must run on every supported toolchain.
- D2 Users on the oldest still-supported Go release should be able to use
  the library.
- D3 The CI matrix should not need edits for every Go release.

## Considered options

1. **Floor at Go 1.25; test `1.25.x`, `oldstable` and `stable`**
   - Pro: synctest is available everywhere; the matrix follows Go releases
     automatically.
   - Con: users on Go 1.24 or older cannot use the module.
2. **Lower floor with build-tagged tests**
   - Con: two test suites, one of them on real time; more maintenance.
3. **Track only the two upstream releases**
   - Con: the floor would move on every Go release, breaking users without a
     code reason.

## Decision

We will require Go 1.25 (`go 1.25` in `go.mod`) and test in CI with
`1.25.x`, `oldstable` and `stable`. The floor is raised only when a newer
language or library feature is needed, through a new ADR and a changelog
entry. `TestRepositoryCompatibilityPolicy` guards the directive.

## Consequences

### Positive
- Every test, including synctest bubbles, runs on every tested toolchain.

### Negative
- When 1.25 drops out of upstream support, CI keeps testing it until the
  floor is raised.

### Follow-ups
- Revisit the floor when a change needs a newer feature.

## Links

- [testing/synctest](https://pkg.go.dev/testing/synctest)
