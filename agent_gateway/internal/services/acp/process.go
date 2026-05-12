package acp

import (
	"io"
	"os/exec"
)

type Process interface {
	Start() error
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Wait() error
	Kill() error
}

type cmdProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func newCmdProcess(command string, args []string, stderr io.Writer) (Process, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	// Child process logs must never go to stdout because stdout is reserved for ACP protocol.
	if stderr == nil {
		stderr = io.Discard
	}
	cmd.Stderr = stderr
	return &cmdProcess{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

func (p *cmdProcess) Start() error {
	return p.cmd.Start()
}

func (p *cmdProcess) Stdin() io.WriteCloser {
	return p.stdin
}

func (p *cmdProcess) Stdout() io.ReadCloser {
	return p.stdout
}

func (p *cmdProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *cmdProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}
