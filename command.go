package executor

import (
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"time"
)

// DefaultKillGrace is the termination grace period selected by Shell and
// ShellWith.
const DefaultKillGrace = 10 * time.Second

// Command describes one process invocation. Args and Env are owned by Clone;
// stream references are shallow and may require caller synchronization when
// shared across concurrent commands.
type Command struct {
	Path string
	Args []string
	Dir  string
	Env  map[string]string
	// ClearEnv prevents executors from supplying the parent environment. Env
	// entries are still supplied explicitly, and the underlying platform may
	// add critical values. The zero value preserves environment inheritance.
	ClearEnv bool
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	// MaxStdoutBytes and MaxStderrBytes bound retained output independently.
	// Zero keeps all output; configured stream writers still receive every byte.
	MaxStdoutBytes int64
	MaxStderrBytes int64
	// ProcessGroup starts the command as the leader of a new process group
	// (Unix) or process group and Job Object (Windows). Signal, Kill, and
	// start-context cancellation then target the whole group, including
	// grandchildren. The zero value targets only the direct child.
	ProcessGroup bool
	// KillGrace is the delay between the terminate request and the forced
	// kill when the start context ends. Zero kills immediately, which is the
	// compatibility behavior. Platforms without a terminate signal kill
	// immediately regardless of the grace period.
	KillGrace time.Duration
}

// Validate checks portable command and dependency invariants.
func (c Command) Validate() error {
	if c.Path == "" {
		return fmt.Errorf("%w: empty path", ErrInvalidCommand)
	}
	if containsByte(c.Path, 0) {
		return fmt.Errorf("%w: path contains NUL", ErrInvalidCommand)
	}
	for index, argument := range c.Args {
		if containsByte(argument, 0) {
			return fmt.Errorf("%w: argument %d contains NUL", ErrInvalidCommand, index)
		}
	}
	if containsByte(c.Dir, 0) {
		return fmt.Errorf("%w: directory contains NUL", ErrInvalidCommand)
	}
	for key, value := range c.Env {
		if key == "" {
			return fmt.Errorf("%w: empty environment key", ErrInvalidCommand)
		}
		if containsByte(key, 0) {
			return fmt.Errorf("%w: environment key contains NUL", ErrInvalidCommand)
		}
		if containsByte(key, '=') {
			return fmt.Errorf("%w: environment key contains '='", ErrInvalidCommand)
		}
		if containsByte(value, 0) {
			return fmt.Errorf("%w: environment value for %q contains NUL", ErrInvalidCommand, key)
		}
	}
	if c.MaxStdoutBytes < 0 {
		return fmt.Errorf("%w: negative stdout capture limit", ErrInvalidCommand)
	}
	if c.MaxStderrBytes < 0 {
		return fmt.Errorf("%w: negative stderr capture limit", ErrInvalidCommand)
	}
	if c.KillGrace < 0 {
		return fmt.Errorf("%w: negative kill grace period", ErrInvalidCommand)
	}
	if interfaceIsNil(c.Stdin) {
		return fmt.Errorf("%w: stdin", ErrNilDependency)
	}
	if interfaceIsNil(c.Stdout) {
		return fmt.Errorf("%w: stdout", ErrNilDependency)
	}
	if interfaceIsNil(c.Stderr) {
		return fmt.Errorf("%w: stderr", ErrNilDependency)
	}
	return nil
}

// Clone owns Args and Env while preserving the supplied stream references.
func (c Command) Clone() Command {
	clone := c
	clone.Args = slices.Clone(c.Args)
	clone.Env = maps.Clone(c.Env)
	return clone
}

func containsByte(value string, target byte) bool {
	for index := 0; index < len(value); index++ {
		if value[index] == target {
			return true
		}
	}
	return false
}

func interfaceIsNil(value any) bool {
	if value == nil {
		return false
	}
	switch reflected := reflect.ValueOf(value); reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
