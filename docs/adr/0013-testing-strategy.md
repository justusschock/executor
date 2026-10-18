# ADR 0013: Testing strategy

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-10-18 |
| Deciders | Justus Perillieux |
| Supersedes | — |
| Related | PRD-0001 (NFR-EXEC-01, NFR-EXEC-02, NFR-EXEC-06), ADR-0012 |

## Context

Recorded retrospectively. The library mixes pure logic (validation,
environment building, Windows command lines, prefix writing, escalation
timing, batch admission) with real operating-system processes (signals,
process groups, rusage, pipes). Real processes cannot run inside a
`testing/synctest` bubble: their pipes and reaping block on the operating
system, not on bubble-managed primitives. Pure timing logic, in contrast,
is slow and flaky on wall-clock time.

## Decision drivers

- D1 Tests must be deterministic and pass with `go test -race -count=3`.
- D2 Real process behaviour (signals, groups, exit codes, environment) must
  be tested against the real OS.
- D3 No dependency on external binaries beyond the platform shell.
- D4 Platforms without CI runners must at least compile and vet.

## Considered options

1. **Layered tests: pure unit tests; synctest for timing and goroutine
   coordination; real-process tests that re-execute the test binary as a
   helper; cross-platform `go vet` in CI**
   - Pro: timing logic is exact (for example "kill at exactly 10 s") and
     instant; process behaviour is verified for real.
   - Con: real-process tests take seconds and use generous timeouts.
2. **Fake clock injected into the implementation**
   - Con: adds an API or hook only for tests; synctest makes it unnecessary.
3. **Mock `os/exec`**
   - Con: would not catch platform behaviour such as signal delivery to
     process groups.

## Decision

We will:

- unit-test pure logic directly, from inside the package where needed;
- run time- and coordination-dependent logic that does not touch real
  processes in `synctest.Test` bubbles: the kill-grace `escalation` tests
  and the `RunAll` tests (which use fake executors and `synctest.Wait`
  instead of wall-clock timeouts);
- test real process behaviour by re-executing the test binary
  (`TestExecutorHelper` modes) and, on Unix, `/bin/sh`; these tests stay on
  real time with a 5-second safety timeout;
- keep the prefix-writer concurrency tests on real time: they hold a
  `sync.Mutex` while the destination yields, and synctest does not treat
  mutex waits as durable blocking, so a virtual sleep would stall the bubble;
- keep runnable examples (`example_test.go`) that the README quick start is
  taken from;
- run every test with the race detector in CI on Linux, and `go vet` for
  every supported `GOOS`;
- cite requirement IDs (`// FR-EXEC-NN`) in tests named by PRD-0001.

## Consequences

### Positive
- Timing logic is tested exactly and quickly; OS behaviour is tested for
  real.

### Negative
- Windows and BSD behaviour is only compiled and vetted in CI, not executed.

### Follow-ups
- Add Windows and macOS test runners if platform regressions appear.

## Links

- [testing/synctest](https://pkg.go.dev/testing/synctest)
