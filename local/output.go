package local

import (
	"bytes"
	"io"
	"sync"
)

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (buffer *lockedBuffer) Write(value []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.Write(value)
}

func (buffer *lockedBuffer) bytes() []byte {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return bytes.Clone(buffer.buffer.Bytes())
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
