package acp

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func TestCmdProcessStderrSinkDoesNotPolluteStdout(t *testing.T) {
	if os.Getenv("ACP_PROCESS_HELPER") == "1" {
		_, _ = os.Stdout.WriteString("protocol\n")
		_, _ = os.Stderr.WriteString("child-log\n")
		os.Exit(0)
	}

	var stderr bytes.Buffer
	proc, err := newCmdProcess(os.Args[0], []string{"-test.run=TestCmdProcessStderrSinkDoesNotPolluteStdout"}, &stderr)
	if err != nil {
		t.Fatalf("newCmdProcess() error = %v", err)
	}
	t.Setenv("ACP_PROCESS_HELPER", "1")
	if cmdProc, ok := proc.(*cmdProcess); ok {
		cmdProc.cmd.Env = append(os.Environ(), "ACP_PROCESS_HELPER=1")
	}
	if err := proc.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	stdout, err := io.ReadAll(proc.Stdout())
	if err != nil {
		t.Fatalf("ReadAll(stdout) error = %v", err)
	}
	if err := proc.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	if got := string(stdout); got != "protocol\n" {
		t.Fatalf("stdout = %q", got)
	}
	if got := stderr.String(); got != "child-log\n" {
		t.Fatalf("stderr sink = %q", got)
	}
}
