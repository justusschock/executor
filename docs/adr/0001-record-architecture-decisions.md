# ADR 0001: Record architecture decisions

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 |

## Context

Recorded retrospectively for the first public release. executor's design
decisions (who owns a process's lifetime, how cancellation escalates, what an
exit error carries, how output is bounded, what an exported process
reference contains) were visible only in code, tests, the README and commit
messages. Implementation plans were kept outside the repository. Outside
contributors need a place to read why the library is shaped the way it is
and a process for changing it.

## Decision drivers

- D1 Contributors and reviewers must find the reasons behind a behaviour
  without reading the whole history.
- D2 Changes to public API, errors, wire formats or cancellation semantics
  need an explicit, reviewable record.
- D3 The process must stay lightweight for a small library.

## Considered options

1. **MADR-style ADRs plus PRDs in `docs/`, docs first**
   - Pro: plain Markdown next to the code, reviewed in the same pull request.
   - Pro: MADR's options/consequences structure forces trade-offs to be
     written down.
   - Con: some overhead for small changes.
2. **Design notes in the README only**
   - Pro: one file.
   - Con: no history of decisions; the README describes usage, not rationale.
3. **Commit messages and issues only**
   - Pro: no extra files.
   - Con: rationale is scattered and hard to find; not versioned with the
     code.

## Decision

We will record requirements as PRDs in `docs/prd/` and decisions as
MADR-style ADRs in `docs/adr/`, both from the templates in those
directories. Documents are written before the behaviour they govern (docs
first); new documents start as `Proposed` and become `Accepted` when merged.
Accepted ADRs are immutable except for their status line; a changed decision
is a new ADR that `Supersedes` the old one, which is then marked
`Superseded by ADR-NNNN`. `docs/README.md` indexes every document with its
status.

## Consequences

### Positive
- Rationale is versioned with the code and reviewed with it.
- Requirement IDs (`FR-EXEC-NN`) link the PRD to tests.

### Negative
- Pull requests that change behaviour carry a documentation update.

### Follow-ups
- ADR-0002 to ADR-0013 record the decisions evident in the code at the first
  public release.

## Links

- [MADR](https://adr.github.io/madr/)
- [docs/README.md](../README.md)
