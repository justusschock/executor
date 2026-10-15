//go:build !windows && !android

package executor

const defaultShellPath = "/bin/sh"

func defaultShellArgs() []string {
	return []string{"-c"}
}
