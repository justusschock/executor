package local

import "strings"

// windowsShellCommandLine returns a verbatim command line for cmd.exe commands
// whose arguments end in "/c script" and include "/s", such as those built by
// executor.Shell on Windows. Standard argument escaping uses backslashes that
// cmd.exe does not understand; with /s, cmd.exe strips only the outer quotes
// and runs the script unchanged.
func windowsShellCommandLine(path string, arguments []string) (string, bool) {
	if len(arguments) < 2 || !isWindowsShell(path) || hasSpaceOrQuote(path) {
		return "", false
	}
	flags := arguments[:len(arguments)-1]
	if !strings.EqualFold(flags[len(flags)-1], "/c") {
		return "", false
	}
	verbatim := false
	for _, flag := range flags {
		if hasSpaceOrQuote(flag) {
			return "", false
		}
		verbatim = verbatim || strings.EqualFold(flag, "/s")
	}
	if !verbatim {
		return "", false
	}
	script := arguments[len(arguments)-1]
	return path + " " + strings.Join(flags, " ") + ` "` + script + `"`, true
}

func isWindowsShell(path string) bool {
	base := path[strings.LastIndexAny(path, `\/`)+1:]
	return strings.EqualFold(base, "cmd") || strings.EqualFold(base, "cmd.exe")
}

func hasSpaceOrQuote(value string) bool {
	return value == "" || strings.ContainsAny(value, " \t\"")
}
