//go:build windows

package local

import (
	"os"

	executor "github.com/justusschock/executor"
)

// platformUsage reports no memory or I/O counters on Windows: the process
// handle is closed once the child has been waited for.
func platformUsage(*os.ProcessState) executor.Usage {
	return executor.Usage{}
}
