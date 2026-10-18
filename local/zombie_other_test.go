//go:build aix || darwin || dragonfly || freebsd || netbsd || openbsd || solaris

package local

// isZombie reports false: outside Linux the tests rely on kill(pid, 0)
// alone.
func isZombie(int) bool { return false }
