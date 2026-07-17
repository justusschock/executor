package executor

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestCommandCloneCopiesMutableFields(t *testing.T) {
	stdin := strings.NewReader("input")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command := Command{
		Path:           "tool",
		Args:           []string{"one"},
		Dir:            "/work",
		Env:            map[string]string{"KEY": "value"},
		Stdin:          stdin,
		Stdout:         stdout,
		Stderr:         stderr,
		MaxStdoutBytes: 5,
		MaxStderrBytes: 6,
	}

	clone := command.Clone()
	clone.Args[0] = "changed"
	clone.Env["KEY"] = "changed"

	if command.Args[0] != "one" || command.Env["KEY"] != "value" {
		t.Fatalf("source mutated: %+v", command)
	}
	if clone.Path != command.Path || clone.Dir != command.Dir || clone.MaxStdoutBytes != command.MaxStdoutBytes || clone.MaxStderrBytes != command.MaxStderrBytes {
		t.Fatalf("immutable fields changed: clone=%+v source=%+v", clone, command)
	}
	if clone.Stdin != stdin || clone.Stdout != stdout || clone.Stderr != stderr {
		t.Fatal("Clone did not retain I/O interfaces")
	}
}

func TestCommandClonePreservesNilCollections(t *testing.T) {
	clone := (Command{Path: "tool"}).Clone()
	if clone.Args != nil || clone.Env != nil {
		t.Fatalf("Clone() = %+v, want nil Args and Env", clone)
	}
}

func TestCommandValidateAcceptsValidCommand(t *testing.T) {
	command := Command{
		Path: "tool",
		Args: []string{"", "one"},
		Env:  map[string]string{"KEY": "", "OTHER": "value"},
	}
	if err := command.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestCommandValidateRejectsTypedNilStreams(t *testing.T) {
	var stream *bytes.Buffer
	tests := []struct {
		name string
		set  func(*Command)
	}{
		{name: "stdin", set: func(command *Command) { command.Stdin = stream }},
		{name: "stdout", set: func(command *Command) { command.Stdout = stream }},
		{name: "stderr", set: func(command *Command) { command.Stderr = stream }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := Command{Path: "tool"}
			test.set(&command)
			if err := command.Validate(); !errors.Is(err, ErrNilDependency) {
				t.Fatalf("Validate() error = %v, want %v", err, ErrNilDependency)
			}
		})
	}
}

func TestCommandValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name    string
		command Command
	}{
		{name: "empty path", command: Command{}},
		{name: "NUL path", command: Command{Path: "bad\x00path"}},
		{name: "NUL argument", command: Command{Path: "tool", Args: []string{"bad\x00argument"}}},
		{name: "NUL directory", command: Command{Path: "tool", Dir: "bad\x00directory"}},
		{name: "empty environment key", command: Command{Path: "tool", Env: map[string]string{"": "value"}}},
		{name: "equals in environment key", command: Command{Path: "tool", Env: map[string]string{"BAD=KEY": "value"}}},
		{name: "NUL environment key", command: Command{Path: "tool", Env: map[string]string{"BAD\x00KEY": "value"}}},
		{name: "NUL environment value", command: Command{Path: "tool", Env: map[string]string{"KEY": "bad\x00value"}}},
		{name: "negative stdout capture limit", command: Command{Path: "tool", MaxStdoutBytes: -1}},
		{name: "negative stderr capture limit", command: Command{Path: "tool", MaxStderrBytes: -1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.command.Validate(); !errors.Is(err, ErrInvalidCommand) {
				t.Fatalf("Validate(%+v) = %v, want %v", test.command, err, ErrInvalidCommand)
			}
		})
	}
}
