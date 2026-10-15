//go:build aix || android || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package local

import (
	"os"
	"syscall"

	executor "github.com/justusschock/go-executor"
)

func platformUsage(state *os.ProcessState) executor.Usage {
	rusage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok || rusage == nil {
		return executor.Usage{}
	}
	return executor.Usage{
		MaxRSSBytes: int64(rusage.Maxrss) * maxRSSUnitBytes,
		ReadBytes:   int64(rusage.Inblock) * blockIOUnitBytes,
		WriteBytes:  int64(rusage.Oublock) * blockIOUnitBytes,
	}
}
