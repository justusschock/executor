package executor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPrefixWriterBuffersLinesAndFlushesOnClose(t *testing.T) {
	t.Parallel()

	var destination bytes.Buffer
	writer := NewPrefixWriter(&destination, "[a] ")
	for _, chunk := range []string{"hel", "lo\nwor", "ld\n\npar", "tial"} {
		if written, err := writer.Write([]byte(chunk)); err != nil || written != len(chunk) {
			t.Fatalf("Write(%q) = (%d, %v)", chunk, written, err)
		}
	}
	if got, want := destination.String(), "[a] hello\n[a] world\n[a] \n"; got != want {
		t.Fatalf("before Close = %q, want %q", got, want)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if got, want := destination.String(), "[a] hello\n[a] world\n[a] \n[a] partial\n"; got != want {
		t.Fatalf("after Close = %q, want %q", got, want)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("second Close() = %v", err)
	}
	if _, err := writer.Write([]byte("late\n")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Write after Close error = %v, want %v", err, io.ErrClosedPipe)
	}
}

func TestPrefixWriterCloseWithoutPendingWritesNothing(t *testing.T) {
	t.Parallel()

	var destination bytes.Buffer
	writer := NewPrefixWriter(&destination, "[a] ")
	if _, err := writer.Write([]byte("done\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if got, want := destination.String(), "[a] done\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestPrefixWritersSharingDestinationNeverInterleaveLines(t *testing.T) {
	t.Parallel()

	const stages = 8
	const lines = 200
	destination := &lineCheckingWriter{}
	var group sync.WaitGroup
	for stage := range stages {
		writer := NewPrefixWriter(destination, fmt.Sprintf("[stage-%d] ", stage))
		group.Add(1)
		go func() {
			defer group.Done()
			var payload strings.Builder
			for line := range lines {
				fmt.Fprintf(&payload, "stage-%d line-%d\n", stage, line)
			}
			data := []byte(payload.String())
			// Split lines across writes at varying offsets.
			for chunk := 1; len(data) > 0; chunk = chunk%13 + 1 {
				size := min(chunk, len(data))
				if _, err := writer.Write(data[:size]); err != nil {
					t.Error(err)
					return
				}
				data = data[size:]
			}
			if err := writer.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	group.Wait()

	if destination.overlapped.Load() {
		t.Fatal("writes overlapped at the shared destination")
	}
	if destination.partial.Load() {
		t.Fatal("a destination write contained a partial line")
	}
	pattern := regexp.MustCompile(`^\[stage-(\d+)\] stage-(\d+) line-(\d+)$`)
	next := make([]int, stages)
	output := strings.TrimSuffix(destination.buffer.String(), "\n")
	for _, line := range strings.Split(output, "\n") {
		match := pattern.FindStringSubmatch(line)
		if match == nil || match[1] != match[2] {
			t.Fatalf("interleaved or corrupted line %q", line)
		}
		stage, _ := strconv.Atoi(match[1])
		number, _ := strconv.Atoi(match[3])
		if number != next[stage] {
			t.Fatalf("stage %d line %d, want %d", stage, number, next[stage])
		}
		next[stage]++
	}
	for stage, count := range next {
		if count != lines {
			t.Fatalf("stage %d emitted %d lines, want %d", stage, count, lines)
		}
	}
	assertNoDestinationLocks(t, destination)
}

func TestPrefixWriterIsSafeForConcurrentWrites(t *testing.T) {
	t.Parallel()

	destination := &lineCheckingWriter{}
	writer := NewPrefixWriter(destination, "> ")
	var group sync.WaitGroup
	for range 4 {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 100 {
				if _, err := writer.Write([]byte("line\n")); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	group.Wait()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if got, want := destination.buffer.String(), strings.Repeat("> line\n", 400); got != want {
		t.Fatalf("output has %d bytes, want %d", len(got), len(want))
	}
}

func TestPrefixWriterBoundsPartialLines(t *testing.T) {
	t.Parallel()

	var destination bytes.Buffer
	writer := NewPrefixWriter(&destination, "p ")
	long := bytes.Repeat([]byte{'x'}, MaxPrefixLineBytes+10)
	if _, err := writer.Write(long); err != nil {
		t.Fatal(err)
	}
	if got, want := destination.String(), "p "+string(long[:MaxPrefixLineBytes])+"\n"; got != want {
		t.Fatalf("bounded output has %d bytes, want %d", len(got), len(want))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimPrefix(destination.String(), "p "+string(long[:MaxPrefixLineBytes])+"\n"), "p xxxxxxxxxx\n"; got != want {
		t.Fatalf("remainder = %q, want %q", got, want)
	}
}

func TestPrefixWriterReportsDestinationErrors(t *testing.T) {
	t.Parallel()

	want := errors.New("destination failed")
	writer := NewPrefixWriter(funcWriter(func([]byte) (int, error) { return 0, want }), "")
	if _, err := writer.Write([]byte("line\n")); !errors.Is(err, want) {
		t.Fatalf("Write() error = %v, want %v", err, want)
	}

	short := NewPrefixWriter(funcWriter(func(value []byte) (int, error) { return len(value) - 1, nil }), "")
	if _, err := short.Write([]byte("partial")); err != nil {
		t.Fatalf("buffered Write() error = %v", err)
	}
	if err := short.Close(); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("Close() error = %v, want %v", err, io.ErrShortWrite)
	}
}

func TestPrefixWriterNestsAndAcceptsIncomparableDestinations(t *testing.T) {
	t.Parallel()

	var destination bytes.Buffer
	var mu sync.Mutex
	incomparable := funcWriter(func(value []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return destination.Write(value)
	})
	inner := NewPrefixWriter(incomparable, "a ")
	outer := NewPrefixWriter(inner, "b ")
	if _, err := outer.Write([]byte("line\npartial")); err != nil {
		t.Fatal(err)
	}
	if err := outer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := inner.Close(); err != nil {
		t.Fatal(err)
	}
	if got, want := destination.String(), "a b line\na b partial\n"; got != want {
		t.Fatalf("nested output = %q, want %q", got, want)
	}
	assertNoDestinationLocks(t, inner)
}

func assertNoDestinationLocks(t *testing.T, destination io.Writer) {
	t.Helper()
	destinationLocksMu.Lock()
	defer destinationLocksMu.Unlock()
	if _, ok := destinationLocks[destination]; ok {
		t.Fatal("closed prefix writers left a destination lock registered")
	}
}

type funcWriter func([]byte) (int, error)

func (writer funcWriter) Write(value []byte) (int, error) {
	return writer(value)
}

// lineCheckingWriter is deliberately unsynchronized; it records overlapping
// calls and calls that do not end in a complete line.
type lineCheckingWriter struct {
	active     atomic.Int32
	overlapped atomic.Bool
	partial    atomic.Bool
	buffer     bytes.Buffer
}

func (writer *lineCheckingWriter) Write(value []byte) (int, error) {
	if writer.active.Add(1) != 1 {
		writer.overlapped.Store(true)
	}
	defer writer.active.Add(-1)
	if len(value) == 0 || value[len(value)-1] != '\n' {
		writer.partial.Store(true)
	}
	runtime.Gosched()
	time.Sleep(10 * time.Microsecond)
	return writer.buffer.Write(value)
}
