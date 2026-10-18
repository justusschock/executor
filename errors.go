package executor

import "errors"

// Sentinel errors. Errors returned by this module wrap them with %w; test
// them with errors.Is.
var (
	// ErrInvalidCommand reports a Command that fails Validate.
	ErrInvalidCommand = errors.New("executor: invalid command")
	// ErrInvalidSignal reports a signal name the platform does not support.
	ErrInvalidSignal = errors.New("executor: invalid signal")
	// ErrNotStarted reports an operation on a process that was never started.
	ErrNotStarted = errors.New("executor: process not started")
	// ErrProcessExited reports an export of a process that has already exited.
	ErrProcessExited = errors.New("executor: process exited")
	// ErrNilDependency reports a nil or typed-nil dependency, such as a nil
	// Starter, Executor or stream implementation.
	ErrNilDependency = errors.New("executor: nil dependency")
	// ErrUnsupportedAdoption reports that an adapter cannot adopt an exported
	// process reference.
	ErrUnsupportedAdoption = errors.New("executor: unsupported adoption")
)
