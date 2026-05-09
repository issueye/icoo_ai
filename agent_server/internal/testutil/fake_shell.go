package testutil

import (
	"context"
	"sync"
	"time"
)

type ShellCommand struct {
	Command string
	Dir     string
	Env     []string
	Timeout time.Duration
}

type ShellResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type ShellCallRecord struct {
	Command ShellCommand
}

type fakeShellResponse struct {
	result ShellResult
	err    error
}

type FakeShellRunner struct {
	mu        sync.Mutex
	calls     []ShellCallRecord
	responses []fakeShellResponse

	DefaultResult ShellResult
	DefaultErr    error
}

func NewFakeShellRunner(results ...ShellResult) *FakeShellRunner {
	r := &FakeShellRunner{}
	for _, result := range results {
		r.responses = append(r.responses, fakeShellResponse{result: result})
	}
	return r
}

func (r *FakeShellRunner) Run(ctx context.Context, cmd ShellCommand) (ShellResult, error) {
	select {
	case <-ctx.Done():
		return ShellResult{}, ctx.Err()
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.calls = append(r.calls, ShellCallRecord{Command: cloneShellCommand(cmd)})
	index := len(r.calls) - 1

	if index < len(r.responses) {
		response := r.responses[index]
		return response.result, response.err
	}
	return r.DefaultResult, r.DefaultErr
}

func (r *FakeShellRunner) EnqueueResult(result ShellResult) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.responses = append(r.responses, fakeShellResponse{result: result})
}

func (r *FakeShellRunner) EnqueueError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.responses = append(r.responses, fakeShellResponse{err: err})
}

func (r *FakeShellRunner) Calls() []ShellCallRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	calls := make([]ShellCallRecord, len(r.calls))
	for i, call := range r.calls {
		calls[i] = ShellCallRecord{Command: cloneShellCommand(call.Command)}
	}
	return calls
}

func cloneShellCommand(cmd ShellCommand) ShellCommand {
	cmd.Env = append([]string(nil), cmd.Env...)
	return cmd
}
