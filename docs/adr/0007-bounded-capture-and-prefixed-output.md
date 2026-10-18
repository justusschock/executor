# ADR 0007: Bounded output capture and prefixed streaming

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 (FR-EXEC-03, FR-EXEC-05, FR-EXEC-13) |

## Context

Recorded retrospectively. Callers want both: live output (to a terminal or
log) and the captured output in the result. Unbounded capture can exhaust
memory for chatty programs; truncating the stream itself would block or
lose data for the live destination. When several commands stream to one
terminal, their lines interleave and lose their origin.

The local executor always captures stdout and stderr into locked buffers.
A positive `MaxStdoutBytes`/`MaxStderrBytes` keeps only the prefix, still
drains the pipe and forwards every byte to the configured writer, and sets
a truncation flag only when bytes were dropped. When stdout and stderr share
a destination, writes are serialized through one mutex.

`NewPrefixWriter` buffers partial lines, writes only complete prefixed lines,
and serializes all prefix writers that share a destination through a
reference-counted lock registry keyed by the destination (one shared lock
for non-comparable destinations). Lines longer than `MaxPrefixLineBytes`
(64 KiB) are emitted in chunks.

## Decision drivers

- D1 Bounded memory per command.
- D2 Live destinations always receive the complete stream.
- D3 Concurrent commands writing to one terminal must not split lines.
- D4 Zero values keep the earlier behaviour (retain everything).

## Considered options

1. **Always capture, bound retention per stream, tee to writers; separate
   prefix writer**
   - Pro: results are always available; memory is bounded when asked.
   - Pro: the prefix writer is an ordinary `io.WriteCloser` usable with any
     executor.
   - Con: output is held twice in memory up to the limit when a writer is
     configured.
2. **Capture only when no writer is configured**
   - Con: callers who stream lose output for error reports.
3. **Ring buffer keeping the tail instead of the prefix**
   - Pro: the end of the output often holds the error.
   - Con: needs copying on every write once full and loses the first lines,
     which identify the run; a prefix plus the truncation flag is simpler.

## Decision

We will always capture both streams, bound retained bytes per stream when a
positive limit is set, forward every byte to configured writers, and provide
`NewPrefixWriter` for line-labelled, non-interleaving streaming.

## Consequences

### Positive
- Predictable memory and complete live output.

### Negative
- A prefix writer must be closed to flush its last partial line and to
  release the shared destination lock.

### Follow-ups
- None.

## Links

- [PRD-0001](../prd/0001-go-executor.md)
