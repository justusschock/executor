package local

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	executor "github.com/justusschock/go-executor"
)

const processReferenceMediaType = "application/vnd.justusschock.process-reference+json"

func TestExportLiveProcessReference(t *testing.T) {
	command := helperCommand(t, "block", &bytes.Buffer{}, &bytes.Buffer{})
	command.Args = append(command.Args, "private-argument")
	command.Env["PRIVATE_TOKEN"] = "private-environment"
	command.Stdin = strings.NewReader("private-input")

	started, err := New().Start(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	processInstance := started.(*process)
	finished := false
	t.Cleanup(func() {
		if finished {
			return
		}
		_ = started.Kill(context.Background())
		_, _ = waitForProcess(t, started)
	})

	artifact, err := started.Export(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := artifact.Validate(); err != nil {
		t.Fatalf("Artifact.Validate() = %v", err)
	}
	if artifact.MediaType != processReferenceMediaType {
		t.Fatalf("MediaType = %q, want %q", artifact.MediaType, processReferenceMediaType)
	}
	if artifact.Version != 1 {
		t.Fatalf("Version = %d, want 1", artifact.Version)
	}
	if want := map[string]string{"kind": "local-process-reference"}; !reflect.DeepEqual(artifact.Metadata, want) {
		t.Fatalf("Metadata = %#v, want %#v", artifact.Metadata, want)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(artifact.Data, &fields); err != nil {
		t.Fatalf("json.Unmarshal() = %v", err)
	}
	if len(fields) != 3 || fields["pid"] == nil || fields["path"] == nil || fields["started_at"] == nil {
		t.Fatalf("JSON fields = %v, want exactly pid/path/started_at", fields)
	}

	wantData, err := json.Marshal(struct {
		PID       int    `json:"pid"`
		Path      string `json:"path"`
		StartedAt string `json:"started_at"`
	}{
		PID:       processInstance.cmd.Process.Pid,
		Path:      command.Path,
		StartedAt: processInstance.startedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(artifact.Data, wantData) {
		t.Fatalf("Data = %s, want %s", artifact.Data, wantData)
	}
	for _, private := range []string{"private-argument", "PRIVATE_TOKEN", "private-environment", "private-input"} {
		if bytes.Contains(artifact.Data, []byte(private)) {
			t.Fatalf("Data contains private command material %q: %s", private, artifact.Data)
		}
	}

	if _, _, complete := processInstance.wait.Try(); complete {
		t.Fatal("Export ended the process or transferred its custody")
	}
	if err := started.Kill(context.Background()); err != nil {
		t.Fatalf("caller could not kill exported process: %v", err)
	}
	if _, err := waitForProcess(t, started); err == nil {
		t.Fatal("Wait() after Kill() error = nil")
	}
	finished = true
}

func TestExportRejectsCanceledContext(t *testing.T) {
	wantErr := errors.New("stop export")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(wantErr)

	artifact, err := (&process{}).Export(ctx)
	if err != wantErr { //nolint:errorlint // Export returns the context cause itself, not a wrapper.
		t.Fatalf("Export() error = %v, want exact cause %v", err, wantErr)
	}
	if !reflect.ValueOf(artifact).IsZero() {
		t.Fatalf("Export() artifact = %+v, want zero", artifact)
	}
}

func TestExportRejectsUnavailableProcess(t *testing.T) {
	artifact, err := (&process{}).Export(context.Background())
	if !errors.Is(err, executor.ErrNotStarted) {
		t.Fatalf("Export() error = %v, want %v", err, executor.ErrNotStarted)
	}
	if !reflect.ValueOf(artifact).IsZero() {
		t.Fatalf("Export() artifact = %+v, want zero", artifact)
	}
}

func TestExportRejectsExitedProcess(t *testing.T) {
	started, err := New().Start(context.Background(), helperCommand(t, "success", nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := waitForProcess(t, started); err != nil {
		t.Fatal(err)
	}

	artifact, err := started.Export(context.Background())
	if !errors.Is(err, executor.ErrProcessExited) {
		t.Fatalf("Export() error = %v, want %v", err, executor.ErrProcessExited)
	}
	if !reflect.ValueOf(artifact).IsZero() {
		t.Fatalf("Export() artifact = %+v, want zero", artifact)
	}
}
