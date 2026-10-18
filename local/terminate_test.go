package local

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	executor "github.com/justusschock/executor"
)

// FR-EXEC-10
func TestEscalationKillsImmediatelyWithoutGrace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		recorder := newEscalationRecorder(nil)
		if err := recorder.escalation(0, make(chan struct{})).run(); err != nil {
			t.Fatal(err)
		}
		recorder.assertEvents(t, "kill@0s")
	})
}

// FR-EXEC-10
func TestEscalationKillsAfterGrace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		recorder := newEscalationRecorder(nil)
		if err := recorder.escalation(10*time.Second, make(chan struct{})).run(); err != nil {
			t.Fatal(err)
		}
		recorder.assertEvents(t, "terminate@0s")

		time.Sleep(10*time.Second - time.Nanosecond)
		synctest.Wait()
		recorder.assertEvents(t, "terminate@0s")

		time.Sleep(time.Nanosecond)
		synctest.Wait()
		recorder.assertEvents(t, "terminate@0s", "kill@10s")
	})
}

// FR-EXEC-10
func TestEscalationStopsWhenProcessExitsDuringGrace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		recorder := newEscalationRecorder(nil)
		exited := make(chan struct{})
		if err := recorder.escalation(10*time.Second, exited).run(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(3 * time.Second)
		close(exited)
		synctest.Wait()

		time.Sleep(time.Hour)
		recorder.assertEvents(t, "terminate@0s")
	})
}

// FR-EXEC-10
func TestEscalationKillsImmediatelyWithoutTerminateSignal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		recorder := newEscalationRecorder(fmt.Errorf("%w: %q", executor.ErrInvalidSignal, "TERM"))
		if err := recorder.escalation(10*time.Second, make(chan struct{})).run(); err != nil {
			t.Fatal(err)
		}
		recorder.assertEvents(t, "terminate@0s", "kill@0s")
	})
}

// FR-EXEC-10
func TestEscalationReturnsTerminateFailureWithoutKilling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		recorder := newEscalationRecorder(os.ErrProcessDone)
		if err := recorder.escalation(10*time.Second, make(chan struct{})).run(); !errors.Is(err, os.ErrProcessDone) {
			t.Fatalf("run() error = %v, want %v", err, os.ErrProcessDone)
		}
		time.Sleep(time.Hour)
		recorder.assertEvents(t, "terminate@0s")
	})
}

type escalationRecorder struct {
	mu           sync.Mutex
	start        time.Time
	events       []string
	terminateErr error
}

func newEscalationRecorder(terminateErr error) *escalationRecorder {
	return &escalationRecorder{start: time.Now(), terminateErr: terminateErr}
}

func (recorder *escalationRecorder) escalation(grace time.Duration, exited <-chan struct{}) escalation {
	return escalation{
		grace: grace,
		terminate: func() error {
			recorder.record("terminate")
			return recorder.terminateErr
		},
		kill: func() error {
			recorder.record("kill")
			return nil
		},
		exited: exited,
	}
}

func (recorder *escalationRecorder) record(event string) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.events = append(recorder.events, fmt.Sprintf("%s@%s", event, time.Since(recorder.start)))
}

func (recorder *escalationRecorder) assertEvents(t *testing.T, want ...string) {
	t.Helper()
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if !reflect.DeepEqual(recorder.events, want) {
		t.Fatalf("events = %q, want %q", recorder.events, want)
	}
}
