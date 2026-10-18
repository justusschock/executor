# executor documentation

Requirements (PRDs) say *what* the library does; architecture decision
records (ADRs) say *how and why*. The process is described in
[ADR-0001](adr/0001-record-architecture-decisions.md) and
[AGENTS.md](../AGENTS.md). New documents start from the templates:
[PRD template](prd/TEMPLATE.md), [ADR template](adr/TEMPLATE.md).

Requirement area code: `EXEC` (`FR-EXEC-NN`, `NFR-EXEC-NN`).

## Product requirements

| PRD | Title | Status | Implementation |
|---|---|---|---|
| [0001](prd/0001-executor.md) | executor | Accepted | Complete |

## Architecture decisions

| ADR | Title | Status |
|---|---|---|
| [0001](adr/0001-record-architecture-decisions.md) | Record architecture decisions | Accepted |
| [0002](adr/0002-neutral-contracts-and-local-implementation.md) | Neutral root contracts with a separate local implementation | Accepted |
| [0003](adr/0003-dependency-policy.md) | Dependency policy | Accepted |
| [0004](adr/0004-error-semantics.md) | Error semantics | Accepted |
| [0005](adr/0005-concurrency-and-cancellation-model.md) | Concurrency and cancellation model | Accepted |
| [0006](adr/0006-process-groups-and-kill-escalation.md) | Opt-in process groups and kill-grace escalation | Accepted |
| [0007](adr/0007-bounded-capture-and-prefixed-output.md) | Bounded output capture and prefixed streaming | Accepted |
| [0008](adr/0008-process-reference-format.md) | Process reference export format | Accepted |
| [0009](adr/0009-remote-starter-port.md) | Remote execution through a Starter port | Accepted |
| [0010](adr/0010-opt-in-shell.md) | Direct execution by default, shells only on request | Accepted |
| [0011](adr/0011-public-api-boundary.md) | Public API boundary | Accepted |
| [0012](adr/0012-supported-go-versions.md) | Supported Go versions | Accepted |
| [0013](adr/0013-testing-strategy.md) | Testing strategy | Accepted |
