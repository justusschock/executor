package local

import (
	"os"

	executor "github.com/justusschock/go-executor"
)

func usageFromState(state *os.ProcessState) executor.Usage {
	if state == nil {
		return executor.Usage{}
	}
	usage := platformUsage(state)
	usage.UserCPU = state.UserTime()
	usage.SystemCPU = state.SystemTime()
	return usage
}
