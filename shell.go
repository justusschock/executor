package executor

import "slices"

// DefaultShell returns the platform interpreter used by Shell: "/bin/sh -c" on
// Unix-like systems ("/system/bin/sh -c" on Android) and "cmd.exe /d /s /c" on
// Windows. Each call returns independently owned arguments.
func DefaultShell() ShellOptions {
	return ShellOptions{Path: defaultShellPath, Args: defaultShellArgs()}
}

// ShellOptions selects the interpreter used by ShellWith. Each zero field
// falls back to the platform default independently.
type ShellOptions struct {
	// Path is the interpreter program. Empty selects the DefaultShell path.
	Path string
	// Args precede the script, for example []string{"-euo", "pipefail", "-c"}
	// for bash. Empty selects the DefaultShell arguments.
	Args []string
}

// Shell returns a command that runs script through DefaultShell. The command
// runs in its own process group with DefaultKillGrace, so cancellation reaches
// every program the script starts. Callers may change any field of the
// returned command before running it. The script is interpreted by the shell;
// never interpolate untrusted input into it.
func Shell(script string) Command {
	return ShellWith(ShellOptions{}, script)
}

// ShellWith returns a Shell command that uses options to select the
// interpreter. The script is always passed as the final argument.
func ShellWith(options ShellOptions, script string) Command {
	path := options.Path
	if path == "" {
		path = defaultShellPath
	}
	arguments := slices.Clone(options.Args)
	if len(arguments) == 0 {
		arguments = defaultShellArgs()
	}
	return Command{
		Path:         path,
		Args:         append(arguments, script),
		ProcessGroup: true,
		KillGrace:    DefaultKillGrace,
	}
}
