package local

import (
	"context"
	"encoding/json"
	"time"

	executor "github.com/justusschock/go-executor"
	"github.com/justusschock/go-lifecycle/handoff"
)

const localProcessReferenceMediaType = "application/vnd.justusschock.process-reference+json"

type processReference struct {
	PID       int    `json:"pid"`
	Path      string `json:"path"`
	StartedAt string `json:"started_at"`
}

func (processInstance *process) Export(ctx context.Context) (handoff.Artifact, error) {
	if err := context.Cause(ctx); err != nil {
		return handoff.Artifact{}, err
	}
	if processInstance == nil || processInstance.cmd == nil || processInstance.cmd.Process == nil || processInstance.wait == nil {
		return handoff.Artifact{}, executor.ErrNotStarted
	}
	if _, _, complete := processInstance.wait.Try(); complete {
		return handoff.Artifact{}, executor.ErrProcessExited
	}

	data, err := json.Marshal(processReference{
		PID:       processInstance.cmd.Process.Pid,
		Path:      processInstance.command.Path,
		StartedAt: processInstance.startedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return handoff.Artifact{}, err
	}
	if err := context.Cause(ctx); err != nil {
		return handoff.Artifact{}, err
	}
	if _, _, complete := processInstance.wait.Try(); complete {
		return handoff.Artifact{}, executor.ErrProcessExited
	}

	return handoff.NewArtifact(
		localProcessReferenceMediaType,
		1,
		data,
		map[string]string{"kind": "local-process-reference"},
	)
}
