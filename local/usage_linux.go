//go:build linux

package local

// Linux reports ru_maxrss in kilobytes and derives ru_inblock/ru_oublock from
// storage I/O bytes in 512-byte units. Page-cache hits are not included.
const (
	maxRSSUnitBytes  = 1024
	blockIOUnitBytes = 512
)
