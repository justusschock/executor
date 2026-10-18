package local

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	executor "github.com/justusschock/executor"
)

var _ executor.Executor = (*Executor)(nil)

// Executor starts commands on the local machine. The zero value is ready to
// use, and an Executor is safe for concurrent use.
type Executor struct{}

// New returns a local command executor.
func New() *Executor {
	return &Executor{}
}

// Start validates and starts command. The supplied context owns process
// cancellation for the lifetime of the child: when it ends, the child (or its
// process group when Command.ProcessGroup is set) is killed, or first asked to
// terminate when Command.KillGrace is positive.
func (executorInstance *Executor) Start(ctx context.Context, command executor.Command) (executor.Process, error) {
	ownedCommand := command.Clone()
	if err := ownedCommand.Validate(); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, ownedCommand.Path, ownedCommand.Args...)
	cmd.Dir = ownedCommand.Dir
	var inheritedEnvironment []string
	if !ownedCommand.ClearEnv {
		inheritedEnvironment = os.Environ()
	}
	cmd.Env = buildEnvironment(inheritedEnvironment, ownedCommand.Env, runtime.GOOS == "windows")
	cmd.Stdin = ownedCommand.Stdin

	stdout := newLockedBuffer(ownedCommand.MaxStdoutBytes)
	stderr := newLockedBuffer(ownedCommand.MaxStderrBytes)
	destinationMu := &sync.Mutex{}
	cmd.Stdout = writeTo(stdout, ownedCommand.Stdout, destinationMu)
	cmd.Stderr = writeTo(stderr, ownedCommand.Stderr, destinationMu)
	configureCommand(cmd, ownedCommand)

	processInstance := newProcess(ownedCommand, cmd, stdout, stderr)
	cmd.Cancel = processInstance.cancel
	startedAt := time.Now()
	if err := cmd.Start(); err != nil {
		processInstance.abandon()
		return nil, err
	}
	processInstance.started(ctx, startedAt)
	return processInstance, nil
}

// Run starts command and waits until it has been reaped. The start context may
// terminate the child, but it does not interrupt the reap operation.
func (executorInstance *Executor) Run(ctx context.Context, command executor.Command) (executor.Result, error) {
	processInstance, err := executorInstance.Start(ctx, command)
	if err != nil {
		return executor.Result{}, err
	}
	return processInstance.Wait(context.WithoutCancel(ctx))
}

func buildEnvironment(inherited []string, overrides map[string]string, caseInsensitive bool) []string {
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	environment := make([]string, 0, len(inherited)+len(keys))
	for _, entry := range inherited {
		if !environmentKeyOverridden(environmentKey(entry), keys, caseInsensitive) {
			environment = append(environment, entry)
		}
	}
	for index, key := range keys {
		if caseInsensitive && environmentKeyOverridden(key, keys[index+1:], true) {
			continue
		}
		environment = append(environment, key+"="+overrides[key])
	}
	return environment
}

func environmentKey(entry string) string {
	searchStart := 0
	if strings.HasPrefix(entry, "=") {
		searchStart = 1
	}
	separator := strings.IndexByte(entry[searchStart:], '=')
	if separator < 0 {
		return entry
	}
	return entry[:searchStart+separator]
}

func environmentKeyOverridden(inherited string, overrides []string, caseInsensitive bool) bool {
	for _, override := range overrides {
		if inherited == override || caseInsensitive && strings.EqualFold(inherited, override) {
			return true
		}
	}
	return false
}
