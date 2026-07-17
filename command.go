package executor

import (
	"fmt"
	"io"
	"maps"
	"slices"
)

type Command struct {
	Path   string
	Args   []string
	Dir    string
	Env    map[string]string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

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
	return nil
}

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
