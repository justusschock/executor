// Package local implements executor.Executor for the local machine with
// os/exec.
//
// The context passed to Start owns the child's lifetime: when it ends the
// child is killed, or first asked to terminate when Command.KillGrace is
// positive. With Command.ProcessGroup set, Signal, Kill and cancellation reach
// the whole process group (Unix) or Job Object (Windows). Contexts passed to
// Wait, Signal, Kill and Export bound only that call.
//
// The package supports Windows and the Unix-like targets aix, android,
// darwin, dragonfly, freebsd, linux, netbsd, openbsd and solaris.
package local
