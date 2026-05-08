package testutil

import (
	"context"
	"errors"
	"testing"
)

func TestFakeShellRunnerRecordsCallsAndReturnsQueuedResults(t *testing.T) {
	runner := NewFakeShellRunner(
		ShellResult{Stdout: "one", ExitCode: 0},
		ShellResult{Stderr: "two", ExitCode: 2},
	)

	first, err := runner.Run(context.Background(), ShellCommand{Command: "echo one", Dir: "workspace", Env: []string{"A=B"}})
	if err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	second, err := runner.Run(context.Background(), ShellCommand{Command: "echo two"})
	if err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}

	if first.Stdout != "one" || second.ExitCode != 2 {
		t.Fatalf("unexpected results: %#v %#v", first, second)
	}

	calls := runner.Calls()
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(calls))
	}
	if calls[0].Command.Command != "echo one" {
		t.Fatalf("command = %q, want echo one", calls[0].Command.Command)
	}
	if len(calls[0].Command.Env) != 1 || calls[0].Command.Env[0] != "A=B" {
		t.Fatalf("env = %#v, want [A=B]", calls[0].Command.Env)
	}
}

func TestFakeShellRunnerQueuedError(t *testing.T) {
	want := errors.New("shell failed")
	runner := NewFakeShellRunner(ShellResult{Stdout: "first"})
	runner.EnqueueError(want)

	result, err := runner.Run(context.Background(), ShellCommand{Command: "echo first"})
	if err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	if result.Stdout != "first" {
		t.Fatalf("stdout = %q, want first", result.Stdout)
	}

	_, err = runner.Run(context.Background(), ShellCommand{Command: "exit 1"})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
