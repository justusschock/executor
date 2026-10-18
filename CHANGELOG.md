# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
While the major version is `0`, a minor release may break the API.

## [Unreleased]

Preparation for the first public release. No API changes.

### Changed

- Public API boundary reviewed: every exported identifier in `executor` and
  `local` is part of the public API; nothing moved to `internal/`, unexported
  or removed ([ADR-0011](docs/adr/0011-public-api-boundary.md)).
- Minimum Go version stays 1.25, required by `testing/synctest`; CI tests Go
  1.25, `oldstable` and `stable`
  ([ADR-0012](docs/adr/0012-supported-go-versions.md)).
- `RunAll` tests run in `testing/synctest` bubbles instead of using
  wall-clock timeouts.
- CI pins every action to a commit SHA, needs no credentials, vets every
  supported platform and runs golangci-lint; Dependabot keeps Go modules and
  actions current.
- The README describes public installation and the full API.

### Added

- Package documentation and doc comments for every exported identifier.
- Runnable examples for the quick start, `RunAll`, `NewPrefixWriter` and
  `NewRemote`.
- `NOTICE`, `AGENTS.md`, a requirements document and architecture decision
  records under `docs/`.

## [0.4.0] - 2026-10-15

### Added

- `Shell`, `ShellWith`, `ShellOptions`, `DefaultShell` and
  `DefaultKillGrace` for running scripts through a shell.
- `Command.ProcessGroup` to signal and cancel a command's whole process
  group (Job Object on Windows).
- `Command.KillGrace` to send `TERM` before `KILL` on start-context
  cancellation.
- `NewPrefixWriter` and `MaxPrefixLineBytes` for line-prefixed,
  non-interleaving output.
- `Result.Usage` with CPU time, peak RSS and block I/O.

### Changed

- Minimum Go version raised to 1.25 for `testing/synctest`.

## [0.3.0] - 2026-08-11

### Added

- `Command.ClearEnv` to run commands without the inherited environment.

## [0.2.0] - 2026-07-17

### Added

- `Command`, `Executor`, `Process`, `Result`, `ExitError` and sentinel errors.
- `local.Executor` with streaming, capture, signals and process-reference
  export.
- Bounded capture with `MaxStdoutBytes`/`MaxStderrBytes`.
- `RunAll` for bounded, ordered parallel execution.
- `Remote` and `Starter` for remote transports.

[Unreleased]: https://github.com/justusschock/executor/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/justusschock/executor/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/justusschock/executor/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/justusschock/executor/releases/tag/v0.2.0
