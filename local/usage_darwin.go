//go:build darwin

package local

// Darwin reports ru_maxrss in bytes and ru_inblock/ru_oublock as operation
// counts, which cannot be converted to bytes.
const (
	maxRSSUnitBytes  = 1
	blockIOUnitBytes = 0
)
