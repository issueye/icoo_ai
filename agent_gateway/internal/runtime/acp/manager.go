package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type AgentStatus struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"`
}

type ProcessStarter interface {
	Start(ctx context.Context, agent models.Agent) (AgentProcess, error)
}

type AgentProcess interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Kill() error
	Wait() error
}

type ConnectionFactory interface {
	Connect(ctx context.Context, client *Client, process AgentProcess) (AgentConnection, error)
}

type AgentConnection interface {
	NewSession(ctx context.Context, params acpsdk.NewSessionRequest) (acpsdk.NewSessionResponse, error)
	Prompt(ctx context.Context, params acpsdk.PromptRequest) (acpsdk.PromptResponse, error)
	Cancel(ctx context.Context, params acpsdk.CancelNotification) error
	CloseSession(ctx context.Context, params acpsdk.CloseSessionRequest) (acpsdk.CloseSessionResponse, error)
}

type Manager struct {
	mu        sync.RWMutex
	extension *ExtensionGateway
	starter   ProcessStarter
	connector ConnectionFactory
	events    *events.Bus
	approvals *ApprovalBroker
	statuses  map[string]AgentStatus
	processes map[string]*agentProcess
	closed    bool
}

type Option func(*Manager)

func WithProcessStarter(starter ProcessStarter) Option {
	return func(m *Manager) {
		if starter != nil {
			m.starter = starter
		}
	}
}

func WithConnectionFactory(factory ConnectionFactory) Option {
	return func(m *Manager) {
		if factory != nil {
			m.connector = factory
		}
	}
}

func WithEventBus(bus *events.Bus) Option {
	return func(m *Manager) {
		m.events = bus
	}
}

func NewManager(extension *ExtensionGateway, opts ...Option) *Manager {
	m := &Manager{
		extension: extension,
		starter:   commandStarter{},
		connector: sdkConnectionFactory{},
		statuses:  make(map[string]AgentStatus),
		processes: make(map[string]*agentProcess),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	m.approvals = NewApprovalBroker(m.events)
	return m
}

type agentProcess struct {
	agent   models.Agent
	cancel  context.CancelFunc
	process AgentProcess
	conn    AgentConnection
	key     string
}

func (m *Manager) Client() *Client {
	return m.client("")
}

func (m *Manager) client(agentID string) *Client {
	return NewClient(agentID, m.extension, m.events, m.approvals)
}

func (m *Manager) ApprovalBroker() *ApprovalBroker {
	return m.approvals
}

func (m *Manager) SyncAgents(ctx context.Context, agents []models.Agent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	seen := make(map[string]models.Agent, len(agents))
	for _, agent := range agents {
		seen[agent.ID] = agent
		if agent.Enabled {
			if err := m.StartAgent(ctx, agent); err != nil {
				m.setStatus(agent.ID, AgentStatus{ID: agent.ID, Name: agent.Name, State: "failed", Message: err.Error()})
			}
			continue
		}
		if err := m.StopAgent(agent.ID); err != nil {
			return err
		}
		m.setStatus(agent.ID, AgentStatus{ID: agent.ID, Name: agent.Name, State: "disabled"})
	}

	m.mu.RLock()
	ids := make([]string, 0, len(m.statuses))
	for id := range m.statuses {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		if err := m.StopAgent(id); err != nil {
			return err
		}
		m.mu.Lock()
		delete(m.statuses, id)
		m.mu.Unlock()
	}
	return nil
}

func (m *Manager) StartAgent(ctx context.Context, agent models.Agent) error {
	if agent.ID == "" {
		return errors.New("agent id is required")
	}
	if !agent.Enabled {
		m.setStatus(agent.ID, AgentStatus{ID: agent.ID, Name: agent.Name, State: "disabled"})
		return nil
	}
	if agent.Command == "" {
		m.setStatus(agent.ID, AgentStatus{ID: agent.ID, Name: agent.Name, State: "configured", Message: "command is empty"})
		return nil
	}
	key := agentRuntimeKey(agent)

	m.mu.RLock()
	existing := m.processes[agent.ID]
	closed := m.closed
	m.mu.RUnlock()
	if closed {
		return errors.New("ACP manager is closed")
	}
	if existing != nil && existing.key == key {
		m.setStatus(agent.ID, AgentStatus{ID: agent.ID, Name: agent.Name, State: "running"})
		return nil
	}
	if existing != nil {
		if err := m.StopAgent(agent.ID); err != nil {
			return err
		}
	}

	m.setStatus(agent.ID, AgentStatus{ID: agent.ID, Name: agent.Name, State: "starting"})
	agentCtx, cancel := context.WithCancel(context.Background())
	process, err := m.starter.Start(agentCtx, agent)
	if err != nil {
		cancel()
		return err
	}
	conn, err := m.connector.Connect(ctx, m.client(agent.ID), process)
	if err != nil {
		cancel()
		_ = process.Kill()
		_ = process.Wait()
		return err
	}

	entry := &agentProcess{agent: agent, cancel: cancel, process: process, conn: conn, key: key}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		cancel()
		_ = process.Kill()
		_ = process.Wait()
		return errors.New("ACP manager is closed")
	}
	m.processes[agent.ID] = entry
	m.statuses[agent.ID] = AgentStatus{ID: agent.ID, Name: agent.Name, State: "running"}
	m.mu.Unlock()

	go m.waitAgent(agent.ID, agent.Name, process)
	return nil
}

func (m *Manager) StopAgent(id string) error {
	m.mu.Lock()
	entry := m.processes[id]
	delete(m.processes, id)
	m.mu.Unlock()
	if entry == nil {
		return nil
	}
	entry.cancel()
	_ = entry.process.Kill()
	err := entry.process.Wait()
	m.setStatus(id, AgentStatus{ID: id, Name: entry.agent.Name, State: "stopped"})
	return err
}

func (m *Manager) NewSession(ctx context.Context, agentID string, req acpsdk.NewSessionRequest) (acpsdk.NewSessionResponse, error) {
	conn, err := m.connection(agentID)
	if err != nil {
		return acpsdk.NewSessionResponse{}, err
	}
	if req.Cwd == "" {
		req.Cwd = mustCwd()
	}
	if req.McpServers == nil {
		req.McpServers = []acpsdk.McpServer{}
	}
	return conn.NewSession(ctx, req)
}

func (m *Manager) PromptText(ctx context.Context, agentID string, sessionID acpsdk.SessionId, text string) (acpsdk.PromptResponse, error) {
	conn, err := m.connection(agentID)
	if err != nil {
		return acpsdk.PromptResponse{}, err
	}
	return conn.Prompt(ctx, acpsdk.PromptRequest{
		SessionId: sessionID,
		Prompt:    []acpsdk.ContentBlock{acpsdk.TextBlock(text)},
	})
}

func (m *Manager) Cancel(ctx context.Context, agentID string, sessionID acpsdk.SessionId) error {
	conn, err := m.connection(agentID)
	if err != nil {
		return err
	}
	return conn.Cancel(ctx, acpsdk.CancelNotification{SessionId: sessionID})
}

func (m *Manager) CloseSession(ctx context.Context, agentID string, sessionID acpsdk.SessionId) (acpsdk.CloseSessionResponse, error) {
	conn, err := m.connection(agentID)
	if err != nil {
		return acpsdk.CloseSessionResponse{}, err
	}
	return conn.CloseSession(ctx, acpsdk.CloseSessionRequest{SessionId: sessionID})
}

func (m *Manager) connection(agentID string) (AgentConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry := m.processes[agentID]
	if entry == nil || entry.conn == nil {
		return nil, fmt.Errorf("agent %s is not running", agentID)
	}
	return entry.conn, nil
}

func (m *Manager) waitAgent(id, name string, process AgentProcess) {
	err := process.Wait()
	m.mu.RLock()
	current := m.processes[id]
	m.mu.RUnlock()
	if current != nil && current.process == process {
		m.mu.Lock()
		delete(m.processes, id)
		status := AgentStatus{ID: id, Name: name, State: "exited"}
		if err != nil {
			status.Message = err.Error()
		}
		m.statuses[id] = status
		m.mu.Unlock()
	}
}

func (m *Manager) setStatus(id string, status AgentStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.statuses[id] = status
}

func (m *Manager) Status(id string) AgentStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if status, ok := m.statuses[id]; ok {
		return status
	}
	return AgentStatus{ID: id, State: "unknown"}
}

func (m *Manager) StatusAll() []AgentStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AgentStatus, 0, len(m.statuses))
	for _, status := range m.statuses {
		out = append(out, status)
	}
	return out
}

func (m *Manager) Close() error {
	m.mu.Lock()
	m.closed = true
	entries := make([]*agentProcess, 0, len(m.processes))
	for _, entry := range m.processes {
		entries = append(entries, entry)
	}
	m.processes = make(map[string]*agentProcess)
	for id, status := range m.statuses {
		status.State = "closed"
		m.statuses[id] = status
	}
	m.mu.Unlock()

	var errs []error
	for _, entry := range entries {
		entry.cancel()
		_ = entry.process.Kill()
		if err := entry.process.Wait(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func agentRuntimeKey(agent models.Agent) string {
	data, _ := json.Marshal(struct {
		Command string
		Args    string
		Env     string
		Cwd     string
	}{Command: agent.Command, Args: agent.ArgsJSON, Env: agent.EnvJSON, Cwd: agent.Cwd})
	return string(data)
}

type commandStarter struct{}

func (commandStarter) Start(ctx context.Context, agent models.Agent) (AgentProcess, error) {
	args := decodeStringSlice(agent.ArgsJSON)
	if len(args) == 0 && len(agent.Args) > 0 {
		args = []string(agent.Args)
	}
	cmd := exec.CommandContext(ctx, agent.Command, args...)
	if agent.Cwd != "" {
		cmd.Dir = agent.Cwd
	}
	cmd.Env = os.Environ()
	for key, value := range decodeStringMap(agent.EnvJSON) {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &execAgentProcess{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

type execAgentProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	once   sync.Once
	err    error
}

func (p *execAgentProcess) Stdin() io.WriteCloser { return p.stdin }
func (p *execAgentProcess) Stdout() io.ReadCloser { return p.stdout }
func (p *execAgentProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}
	return p.cmd.Process.Kill()
}
func (p *execAgentProcess) Wait() error {
	p.once.Do(func() { p.err = p.cmd.Wait() })
	return p.err
}

type sdkConnectionFactory struct{}

func (sdkConnectionFactory) Connect(ctx context.Context, client *Client, process AgentProcess) (AgentConnection, error) {
	conn := acpsdk.NewClientSideConnection(client, process.Stdin(), process.Stdout())
	_, err := conn.Initialize(ctx, acpsdk.InitializeRequest{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		ClientInfo:      &acpsdk.Implementation{Name: "icoo-agent-gateway", Version: "0.1.0-dev"},
		ClientCapabilities: acpsdk.ClientCapabilities{
			Meta: map[string]any{
				"extensions": []string{MethodPrefix + "*"},
			},
			Fs:       acpsdk.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: false},
			Terminal: false,
		},
	})
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func decodeStringSlice(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func decodeStringMap(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func mustCwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return string(filepath.Separator)
	}
	abs, err := filepath.Abs(wd)
	if err != nil {
		return wd
	}
	return abs
}
