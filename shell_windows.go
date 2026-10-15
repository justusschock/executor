//go:build windows

package executor

const defaultShellPath = "cmd.exe"

// defaultShellArgs skips AutoRun commands with /d. /s makes cmd.exe strip only
// the outer quotes that the local executor places around the script.
func defaultShellArgs() []string {
	return []string{"/d", "/s", "/c"}
}
