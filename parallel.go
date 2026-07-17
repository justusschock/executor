package executor

import (
	"context"
	"fmt"

	"github.com/justusschock/go-concurrency"
)

// RunAll executes commands with at most limit concurrent calls to runner and
// preserves command order in successful results. Runner must support
// concurrent Run calls. Commands are cloned eagerly, but their stream
// references remain shallow; callers sharing streams must synchronize them.
//
// A command error stops further admission, cancels admitted peers through
// their contexts, waits for those peers to finish, and returns nil results.
// Errors from admitted commands are indexed and joined in admission order.
// An invalid limit is returned as concurrency.ErrInvalidLimit.
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
