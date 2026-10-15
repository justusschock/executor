package executor

import (
	"bytes"
	"io"
	"reflect"
	"sync"
)

// MaxPrefixLineBytes bounds the partial line a prefix writer buffers. A longer
// line without a newline, such as carriage-return progress output, is emitted
// in chunks of this size, each completed with a newline.
const MaxPrefixLineBytes = 64 << 10

// NewPrefixWriter returns a line-buffered writer that writes every line to w
// as prefix followed by the line. Writes are safe for concurrent use, and all
// prefix writers that share the same w serialize their writes to it, so lines
// from different writers never interleave mid-line. Each underlying write
// contains only complete lines.
//
// Close writes a buffered partial line, completed with a newline, and
// releases the shared destination lock. Close is idempotent; Write after Close
// returns io.ErrClosedPipe. w must be non-nil and is never closed. A w whose
// dynamic value is not comparable cannot be identified, so such writers share
// one process-wide lock instead.
func NewPrefixWriter(w io.Writer, prefix string) io.WriteCloser {
	return &prefixWriter{
		destination: w,
		lock:        acquireDestinationLock(w),
		prefix:      []byte(prefix),
	}
}

type prefixWriter struct {
	mu          sync.Mutex
	destination io.Writer
	lock        *destinationLock
	prefix      []byte
	pending     []byte
	closed      bool
}

func (writer *prefixWriter) Write(value []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.closed {
		return 0, io.ErrClosedPipe
	}

	var lines []byte
	remaining := value
	for {
		newline := bytes.IndexByte(remaining, '\n')
		if newline < 0 {
			writer.pending = append(writer.pending, remaining...)
			break
		}
		if len(writer.pending) == 0 {
			lines = writer.appendLine(lines, remaining[:newline])
		} else {
			writer.pending = append(writer.pending, remaining[:newline]...)
			lines = writer.appendLine(lines, writer.pending)
			writer.pending = writer.pending[:0]
		}
		remaining = remaining[newline+1:]
	}
	for len(writer.pending) >= MaxPrefixLineBytes {
		lines = writer.appendLine(lines, writer.pending[:MaxPrefixLineBytes])
		writer.pending = append(writer.pending[:0], writer.pending[MaxPrefixLineBytes:]...)
	}
	return len(value), writer.emit(lines)
}

func (writer *prefixWriter) Close() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.closed {
		return nil
	}
	writer.closed = true
	defer releaseDestinationLock(writer.destination, writer.lock)

	if len(writer.pending) == 0 {
		return nil
	}
	lines := writer.appendLine(nil, writer.pending)
	writer.pending = nil
	return writer.emit(lines)
}

func (writer *prefixWriter) appendLine(lines, line []byte) []byte {
	lines = append(lines, writer.prefix...)
	lines = append(lines, line...)
	return append(lines, '\n')
}

func (writer *prefixWriter) emit(lines []byte) error {
	if len(lines) == 0 {
		return nil
	}
	writer.lock.mu.Lock()
	defer writer.lock.mu.Unlock()
	written, err := writer.destination.Write(lines)
	if err == nil && written < len(lines) {
		err = io.ErrShortWrite
	}
	return err
}

type destinationLock struct {
	mu         sync.Mutex
	references int
}

var (
	destinationLocksMu sync.Mutex
	destinationLocks   = map[io.Writer]*destinationLock{}
	unkeyedLock        = &destinationLock{}
)

func acquireDestinationLock(destination io.Writer) *destinationLock {
	if !comparableWriter(destination) {
		return unkeyedLock
	}
	destinationLocksMu.Lock()
	defer destinationLocksMu.Unlock()
	lock, ok := destinationLocks[destination]
	if !ok {
		lock = &destinationLock{}
		destinationLocks[destination] = lock
	}
	lock.references++
	return lock
}

func releaseDestinationLock(destination io.Writer, lock *destinationLock) {
	if lock == unkeyedLock {
		return
	}
	destinationLocksMu.Lock()
	defer destinationLocksMu.Unlock()
	lock.references--
	if lock.references == 0 {
		delete(destinationLocks, destination)
	}
}

func comparableWriter(destination io.Writer) bool {
	return destination != nil && reflect.ValueOf(destination).Comparable()
}
