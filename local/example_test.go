package local

import (
	"context"
	"fmt"
	"os"
	"strings"

	executor "github.com/justusschock/go-executor"
)

func ExampleExecutor_Run() {
	command, err := exampleCommand("success")
	if err != nil {
		fmt.Println("error")
		return
	}
	result, err := New().Run(context.Background(), command)
	if err != nil {
		fmt.Println("error")
		return
	}
	fmt.Print(string(result.Stdout))
	fmt.Println(result.ExitCode)
	// Output:
	// out
	// 0
}

func ExampleExecutor_boundedRun() {
	first, err := exampleCommand("delay")
	if err != nil {
		fmt.Println("error")
		return
	}
	second, err := exampleCommand("success")
	if err != nil {
		fmt.Println("error")
		return
	}
	results, err := executor.RunAll(
		context.Background(),
		New(),
		[]executor.Command{first, second},
		2,
	)
	if err != nil {
		fmt.Println("error")
		return
	}
	for _, result := range results {
		fmt.Printf("%s %d\n", strings.TrimSpace(string(result.Stdout)), result.ExitCode)
	}
	// Output:
	// delayed 0
	// out 0
}

func exampleCommand(mode string) (executor.Command, error) {
	binary, err := os.Executable()
	if err != nil {
		return executor.Command{}, err
	}
	return executor.Command{
		Path: binary,
		Args: []string{"-test.run=^TestExecutorHelper$"},
		Env: map[string]string{
			"GO_WANT_EXECUTOR_HELPER": "1",
			"EXECUTOR_HELPER_MODE":    mode,
		},
	}, nil
}
