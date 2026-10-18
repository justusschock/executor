// Package executor defines portable contracts for running external commands:
// a Command description, the Executor and Process interfaces, the final
// Result with captured output and resource Usage, and the ExitError returned
// for non-zero exits and wait failures.
//
// The package contains no operating-system process code. The local
// subpackage implements Executor with os/exec; Remote adapts any Starter (for
// example a network transport) to Executor. RunAll runs commands with bounded
// concurrency and ordered results, Shell and ShellWith build commands that run
// a script through a shell, and NewPrefixWriter labels streamed output lines.
//
// Commands are copied before execution: Args and Env are owned by the
// executor, while Stdin, Stdout and Stderr remain shared references that the
// caller must synchronize when several commands share them.
package executor
