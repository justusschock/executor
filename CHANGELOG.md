# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
While the major version is `0`, a minor release may break the API.

## [Unreleased]

## [0.1.0] - 2026-10-18

First public release.

### Added

- `Command` contract: path, arguments, working directory, inherited or
  cleared (`ClearEnv`) environment, optional standard streams, `Validate`
  and `Clone`.
- `Executor`, `Process` and `Result` contracts; `Process` offers repeatable
  `Wait`, `Signal`, `Kill` and `Export`.
- Package `local`: `local.New()` runs commands with `os/exec`, streams
  output while capturing it, and reports exit code, timing and resource
  `Usage`.
- Process groups (`ProcessGroup`) and graceful stop (`KillGrace`: `TERM`,
  then `KILL` after a grace period).
- `Shell` and `ShellWith`: scripts through the platform shell or a chosen
  interpreter, in their own process group.
- Bounded capture through `MaxStdoutBytes` and `MaxStderrBytes`, while
  writers still receive every byte.
- `RunAll`: bounded-concurrency batches with results in input order that
  stop on the first failure.
- `NewPrefixWriter`: line-prefixed output that never interleaves lines from
  concurrent commands.
- `NewRemote`: turns any one-method `Starter` into an `Executor`.
- Non-zero exits return the populated `Result` together with an
  `*ExitError`.
- Requirements and architecture decision records under `docs/`.

Requires Go 1.25 or newer.

[Unreleased]: https://github.com/justusschock/executor/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/justusschock/executor/releases/tag/v0.1.0
