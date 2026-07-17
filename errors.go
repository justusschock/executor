package executor

import "errors"

var (
	ErrInvalidCommand      = errors.New("executor: invalid command")
	ErrInvalidSignal       = errors.New("executor: invalid signal")
	ErrNotStarted          = errors.New("executor: process not started")
	ErrNilDependency       = errors.New("executor: nil dependency")
	ErrUnsupportedAdoption = errors.New("executor: unsupported adoption")
)
