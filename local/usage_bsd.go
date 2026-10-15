//go:build aix || dragonfly || freebsd || netbsd || openbsd || solaris

package local

// These systems report ru_maxrss in kilobytes and ru_inblock/ru_oublock as
// operation counts, which cannot be converted to bytes.
const (
	maxRSSUnitBytes  = 1024
	blockIOUnitBytes = 0
)
