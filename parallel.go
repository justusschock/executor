package executor

import (
	"context"
	"fmt"

	"github.com/justusschock/go-concurrency"
)

// RunAll executes commands with at most limit calls to runner in flight. It
// preserves command order in the returned results and owns a clone of every
// command before execution starts.
func RunAll(ctx context.Context, runner Executor, commands []Command, limit int) ([]Result, error) {
	if runner == nil || interfaceIsNil(runner) {
		return nil, ErrNilDependency
	}

	owned := make([]Command, len(commands))
	indexes := make([]int, len(commands))
	for index := range commands {
		owned[index] = commands[index].Clone()
		indexes[index] = index
	}

	return concurrency.ParallelMap(ctx, indexes, limit, func(ctx context.Context, index int) (Result, error) {
		result, err := runner.Run(ctx, owned[index])
		if err != nil {
			return Result{}, fmt.Errorf("command %d: %w", index, err)
		}
		return result, nil
	})
}
