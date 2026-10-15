//go:build android

package executor

const defaultShellPath = "/system/bin/sh"

func defaultShellArgs() []string {
	return []string{"-c"}
}
