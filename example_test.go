package executor_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	executor "github.com/justusschock/go-executor"
	"github.com/justusschock/go-executor/local"
)

// Run a shell command on the local machine and inspect its result.
func Example() {
	result, err := local.New().Run(context.Background(), executor.Shell("echo hello"))
	if err != nil {
		var exit *executor.ExitError
		if errors.As(err, &exit) {
			fmt.Println("exit code:", exit.Result.ExitCode)
		}
		log.Fatal(err)
	}
	fmt.Println(strings.TrimSpace(string(result.Stdout)), result.ExitCode)
	// Output: hello 0
}

// RunAll runs commands with bounded concurrency and returns results in
// command order.
func ExampleRunAll() {
	commands := []executor.Command{
		executor.Shell("echo first"),
		executor.Shell("echo second"),
	}
	results, err := executor.RunAll(context.Background(), local.New(), commands, 2)
	if err != nil {
		log.Fatal(err)
	}
	for index, result := range results {
		fmt.Println(index, strings.TrimSpace(string(result.Stdout)))
	}
	// Output:
	// 0 first
	// 1 second
}

// NewPrefixWriter labels every line, which keeps output from concurrent
// commands readable when it shares one destination.
func ExampleNewPrefixWriter() {
	var output bytes.Buffer
	writer := executor.NewPrefixWriter(&output, "[train] ")
	_, _ = writer.Write([]byte("epoch 1\nepoch"))
	_, _ = writer.Write([]byte(" 2\npartial"))
	_ = writer.Close()
	fmt.Print(output.String())
	// Output:
	// [train] epoch 1
	// [train] epoch 2
	// [train] partial
}

// NewRemote adapts any Starter to Executor. A local executor is used here as
// the Starter; a real adapter would forward the command over a transport.
func ExampleNewRemote() {
	remote, err := executor.NewRemote(local.New())
	if err != nil {
		log.Fatal(err)
	}
	result, err := remote.Run(context.Background(), executor.Shell("echo remote"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(strings.TrimSpace(string(result.Stdout)))
	// Output: remote
}
