package local

import (
	"bytes"
	"io"
	"sync"
)

type lockedBuffer struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	maxBytes  int64
	truncated bool
}

func newLockedBuffer(maxBytes int64) *lockedBuffer {
	return &lockedBuffer{maxBytes: maxBytes}
}

func (buffer *lockedBuffer) Write(value []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if buffer.maxBytes == 0 {
		return buffer.buffer.Write(value)
	}

	remaining := buffer.maxBytes - int64(buffer.buffer.Len())
	retained := len(value)
	if int64(retained) > remaining {
		retained = max(0, int(remaining))
		buffer.truncated = true
	}
	if retained > 0 {
		_, _ = buffer.buffer.Write(value[:retained])
	}
	return len(value), nil
}

func (buffer *lockedBuffer) snapshot() ([]byte, bool) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return bytes.Clone(buffer.buffer.Bytes()), buffer.truncated
}

type serializedWriter struct {
	mu          *sync.Mutex
	destination io.Writer
}

func (writer *serializedWriter) Write(value []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.destination.Write(value)
}

func writeTo(capture *lockedBuffer, destination io.Writer, destinationMu *sync.Mutex) io.Writer {
	if destination == nil {
		return capture
	}
	return io.MultiWriter(capture, &serializedWriter{mu: destinationMu, destination: destination})
}
