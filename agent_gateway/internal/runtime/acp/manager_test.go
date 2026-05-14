package acp

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/scheduler"
)

func TestManagerSyncStartsAndStopsEnabledAgents(t *testing.T) {
	starter := &fakeStarter{}
	manager := NewManager(nil, WithProcessStarter(starter), WithConnectionFactory(fakeFactory{}))

	agent := models.Agent{
		BaseModel: models.BaseModel{ID: "agent-1"},
		Name:      "Agent One",
		Command:   "agent-bin",
		Enabled:   true,
	}
	if err := manager.SyncAgents(context.Background(), []models.Agent{agent}); err != nil {
		t.Fatalf("SyncAgents(start) error = %v", err)
	}
	if got := manager.Status("agent-1").State; got != "running" {
		t.Fatalf("state = %q, want running", got)
	}
	if starter.starts != 1 {
		t.Fatalf("starts = %d, want 1", starter.starts)
	}

	agent.Enabled = false
	if err := manager.SyncAgents(context.Background(), []models.Agent{agent}); err != nil {
		t.Fatalf("SyncAgents(stop) error = %v", err)
	}
	if got := manager.Status("agent-1").State; got != "disabled" {
		t.Fatalf("state = %q, want disabled", got)
	}
	if !starter.last.killed {
		t.Fatal("process was not killed on disable")
	}
}

func TestManagerPromptTextUsesConnection(t *testing.T) {
	conn := &fakeConnection{}
	manager := NewManager(nil, WithProcessStarter(&fakeStarter{}), WithConnectionFactory(fakeFactory{conn: conn}))
	agent := models.Agent{
		BaseModel: models.BaseModel{ID: "agent-1"},
		Name:      "Agent One",
		Command:   "agent-bin",
		Enabled:   true,
	}
	if err := manager.StartAgent(context.Background(), agent); err != nil {
		t.Fatalf("StartAgent() error = %v", err)
	}
	if _, err := manager.PromptText(context.Background(), "agent-1", acpsdk.SessionId("session-1"), "hello"); err != nil {
		t.Fatalf("PromptText() error = %v", err)
	}
	if conn.promptText != "hello" {
		t.Fatalf("promptText = %q, want hello", conn.promptText)
	}
}

func TestSchedulerRunnerCreatesPromptsAndClosesSession(t *testing.T) {
	conn := &fakeConnection{}
	manager := NewManager(nil, WithProcessStarter(&fakeStarter{}), WithConnectionFactory(fakeFactory{conn: conn}))
	agent := models.Agent{
		BaseModel: models.BaseModel{ID: "agent-1"},
		Name:      "Agent One",
		Command:   "agent-bin",
		Enabled:   true,
	}
	if err := manager.StartAgent(context.Background(), agent); err != nil {
		t.Fatalf("StartAgent() error = %v", err)
	}

	runner := NewSchedulerRunner(manager)
	err := runner.RunAgentPrompt(context.Background(), scheduler.Payload{
		AgentID: "agent-1",
		Prompt:  "scheduled prompt",
	})
	if err != nil {
		t.Fatalf("RunAgentPrompt() error = %v", err)
	}
	if conn.promptText != "scheduled prompt" {
		t.Fatalf("promptText = %q, want scheduled prompt", conn.promptText)
	}
	if conn.closedSession != "session-1" {
		t.Fatalf("closedSession = %q, want session-1", conn.closedSession)
	}
}

type fakeStarter struct {
	starts int
	last   *fakeProcess
}

func (s *fakeStarter) Start(context.Context, models.Agent) (AgentProcess, error) {
	s.starts++
	p := &fakeProcess{done: make(chan struct{})}
	s.last = p
	return p, nil
}

type fakeProcess struct {
	killed bool
	done   chan struct{}
	once   sync.Once
}

func (p *fakeProcess) Stdin() io.WriteCloser { return nopWriteCloser{} }
func (p *fakeProcess) Stdout() io.ReadCloser { return io.NopCloser(strings.NewReader("")) }
func (p *fakeProcess) Kill() error {
	p.killed = true
	p.once.Do(func() { close(p.done) })
	return nil
}
func (p *fakeProcess) Wait() error {
	<-p.done
	return nil
}

type nopWriteCloser struct{}

func (w nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (w nopWriteCloser) Close() error                { return nil }

type fakeFactory struct {
	conn *fakeConnection
}

func (f fakeFactory) Connect(context.Context, *Client, AgentProcess) (AgentConnection, error) {
	if f.conn != nil {
		return f.conn, nil
	}
	return &fakeConnection{}, nil
}

type fakeConnection struct {
	promptText    string
	closedSession string
}

func (c *fakeConnection) NewSession(context.Context, acpsdk.NewSessionRequest) (acpsdk.NewSessionResponse, error) {
	return acpsdk.NewSessionResponse{SessionId: acpsdk.SessionId("session-1")}, nil
}

func (c *fakeConnection) Prompt(_ context.Context, params acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	if len(params.Prompt) > 0 && params.Prompt[0].Text != nil {
		c.promptText = params.Prompt[0].Text.Text
	}
	return acpsdk.PromptResponse{}, nil
}

func (c *fakeConnection) Cancel(context.Context, acpsdk.CancelNotification) error {
	return nil
}

func (c *fakeConnection) CloseSession(_ context.Context, params acpsdk.CloseSessionRequest) (acpsdk.CloseSessionResponse, error) {
	c.closedSession = string(params.SessionId)
	return acpsdk.CloseSessionResponse{}, nil
}
