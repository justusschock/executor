package local

import (
	"os"
	"strconv"
	"strings"
)

// isZombie reports whether pid has exited but not been reaped. In a
// container without an init process, killed grandchildren are reparented to
// a PID 1 that never reaps them, so they linger as zombies that still answer
// kill(pid, 0).
func isZombie(pid int) bool {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	// The command name in field 2 may contain spaces and parentheses; the
	// state is the first field after its closing parenthesis.
	fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
	return len(fields) > 0 && fields[0] == "Z"
}
