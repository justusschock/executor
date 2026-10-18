package executor

import (
	"reflect"
	"runtime"
	"testing"
)

// FR-EXEC-11
func TestDefaultShellMatchesPlatform(t *testing.T) {
	t.Parallel()

	want := ShellOptions{Path: "/bin/sh", Args: []string{"-c"}}
	switch runtime.GOOS {
	case "windows":
		want = ShellOptions{Path: "cmd.exe", Args: []string{"/d", "/s", "/c"}}
	case "android":
		want = ShellOptions{Path: "/system/bin/sh", Args: []string{"-c"}}
	}
	got := DefaultShell()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultShell() = %#v, want %#v", got, want)
	}

	got.Args[0] = "mutated"
	if again := DefaultShell(); !reflect.DeepEqual(again, want) {
		t.Fatalf("DefaultShell() after caller mutation = %#v, want %#v", again, want)
	}
}

// FR-EXEC-11
func TestShellUsesDefaultShellAndProcessGroup(t *testing.T) {
	t.Parallel()

	defaults := DefaultShell()
	command := Shell("python train.py --lr 0.1")
	want := Command{
		Path:         defaults.Path,
		Args:         append(defaults.Args, "python train.py --lr 0.1"),
		ProcessGroup: true,
		KillGrace:    DefaultKillGrace,
	}
	if !reflect.DeepEqual(command, want) {
		t.Fatalf("Shell() = %#v, want %#v", command, want)
	}
	if err := command.Validate(); err != nil {
		t.Fatalf("Shell().Validate() = %v", err)
	}
}

// FR-EXEC-11
func TestShellWithFallsBackPerField(t *testing.T) {
	t.Parallel()

	defaults := DefaultShell()
	tests := []struct {
		name     string
		options  ShellOptions
		wantPath string
		wantArgs []string
	}{
		{name: "zero", wantPath: defaults.Path, wantArgs: append(defaults.Args, "script")},
		{name: "path only", options: ShellOptions{Path: "bash"}, wantPath: "bash", wantArgs: append(defaults.Args, "script")},
		{name: "explicit", options: ShellOptions{Path: "bash", Args: []string{"-euo", "pipefail", "-c"}}, wantPath: "bash", wantArgs: []string{"-euo", "pipefail", "-c", "script"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := ShellWith(test.options, "script")
			if command.Path != test.wantPath || !reflect.DeepEqual(command.Args, test.wantArgs) {
				t.Fatalf("ShellWith() = %q %#v, want %q %#v", command.Path, command.Args, test.wantPath, test.wantArgs)
			}
			if !command.ProcessGroup || command.KillGrace != DefaultKillGrace {
				t.Fatalf("ShellWith() group=%t grace=%v", command.ProcessGroup, command.KillGrace)
			}
		})
	}
}

// FR-EXEC-11
func TestShellWithOwnsArguments(t *testing.T) {
	t.Parallel()

	options := ShellOptions{Path: "bash", Args: make([]string, 1, 4)}
	options.Args[0] = "-c"
	command := ShellWith(options, "script")
	command.Args[0] = "mutated"
	if options.Args[0] != "-c" || len(options.Args[:cap(options.Args)][1]) != 0 {
		t.Fatalf("ShellWith() aliased caller arguments: %#v", options.Args[:cap(options.Args)])
	}
}
