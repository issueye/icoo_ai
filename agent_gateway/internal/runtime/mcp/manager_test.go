package mcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRefreshToolsConnectFailure(t *testing.T) {
	wantErr := errors.New("connect failed")
	manager := NewManager(WithConnector(ConnectorFunc(func(context.Context, ServerConfig) (Client, error) {
		return nil, wantErr
	})))

	_, err := manager.RefreshTools(context.Background(), ServerConfig{
		ID:      "srv-1",
		Name:    "broken",
		Enabled: true,
		Command: "mcp-server",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("RefreshTools error = %v, want %v", err, wantErr)
	}

	status := manager.Status("srv-1")
	if status.State != StateFailed {
		t.Fatalf("status.State = %s, want %s", status.State, StateFailed)
	}
	if status.LastError == "" {
		t.Fatal("status.LastError is empty")
	}
}

func TestRefreshToolsPublishesStatusEvents(t *testing.T) {
	events := make(chan ServerStatus, 4)
	manager := NewManager(
		WithConnector(ConnectorFunc(func(context.Context, ServerConfig) (Client, error) {
			return &fakeClient{tools: []Tool{{Name: "tool"}}}, nil
		})),
		WithStatusListener(func(status ServerStatus) { events <- status }),
	)

	if _, err := manager.RefreshTools(context.Background(), ServerConfig{ID: "srv-1", Enabled: true, Command: "mcp-server"}); err != nil {
		t.Fatalf("RefreshTools() error = %v", err)
	}

	if got := waitStatus(t, events); got.State != StateConnecting {
		t.Fatalf("first state = %s, want connecting", got.State)
	}
	if got := waitStatus(t, events); got.State != StateConnected {
		t.Fatalf("second state = %s, want connected", got.State)
	}
}

func TestRefreshToolsListFailureReconnectsOnce(t *testing.T) {
	first := &fakeClient{listErr: errors.New("session lost")}
	second := &fakeClient{tools: []Tool{{Name: "ok"}}}
	calls := 0
	manager := NewManager(WithConnector(ConnectorFunc(func(context.Context, ServerConfig) (Client, error) {
		calls++
		if calls == 1 {
			return first, nil
		}
		return second, nil
	})))

	tools, err := manager.RefreshTools(context.Background(), ServerConfig{
		ID:      "srv-1",
		Enabled: true,
		Command: "mcp-server",
	})
	if err != nil {
		t.Fatalf("RefreshTools returned error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("connector calls = %d, want 2", calls)
	}
	if !first.closed {
		t.Fatal("first client was not closed after list failure")
	}
	if len(tools) != 1 || tools[0].Name != "ok" {
		t.Fatalf("tools = %#v, want ok tool", tools)
	}
	if status := manager.Status("srv-1"); status.State != StateConnected || status.ToolCount != 1 {
		t.Fatalf("status = %#v, want connected with one tool", status)
	}
}

func TestRefreshToolsDisabledClosesConnection(t *testing.T) {
	client := &fakeClient{tools: []Tool{{Name: "tool"}}}
	manager := NewManager(WithConnector(ConnectorFunc(func(context.Context, ServerConfig) (Client, error) {
		return client, nil
	})))

	cfg := ServerConfig{ID: "srv-1", Name: "server", Enabled: true, Command: "mcp-server"}
	if _, err := manager.RefreshTools(context.Background(), cfg); err != nil {
		t.Fatalf("RefreshTools enabled returned error: %v", err)
	}

	cfg.Enabled = false
	tools, err := manager.RefreshTools(context.Background(), cfg)
	if err != nil {
		t.Fatalf("RefreshTools disabled returned error: %v", err)
	}
	if tools != nil {
		t.Fatalf("tools = %#v, want nil", tools)
	}
	if !client.closed {
		t.Fatal("client was not closed")
	}
	if status := manager.Status("srv-1"); status.State != StateDisabled || status.ToolCount != 0 {
		t.Fatalf("status = %#v, want disabled with no tools", status)
	}
}

func TestCloseClosesClientsAndRejectsRefresh(t *testing.T) {
	client := &fakeClient{tools: []Tool{{Name: "tool"}}}
	manager := NewManager(WithConnector(ConnectorFunc(func(context.Context, ServerConfig) (Client, error) {
		return client, nil
	})))

	if _, err := manager.RefreshTools(context.Background(), ServerConfig{
		ID:      "srv-1",
		Enabled: true,
		Command: "mcp-server",
	}); err != nil {
		t.Fatalf("RefreshTools returned error: %v", err)
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if !client.closed {
		t.Fatal("client was not closed")
	}
	if status := manager.Status("srv-1"); status.State != StateClosed {
		t.Fatalf("status.State = %s, want %s", status.State, StateClosed)
	}
	_, err := manager.RefreshTools(context.Background(), ServerConfig{
		ID:      "srv-2",
		Enabled: true,
		Command: "mcp-server",
	})
	if err == nil {
		t.Fatal("RefreshTools after Close returned nil error")
	}
}

func TestCallToolUsesConnectedClient(t *testing.T) {
	client := &fakeClient{tools: []Tool{{Name: "tool"}}, result: CallResult{Content: "ok"}}
	manager := NewManager(WithConnector(ConnectorFunc(func(context.Context, ServerConfig) (Client, error) {
		return client, nil
	})))
	if _, err := manager.RefreshTools(context.Background(), ServerConfig{ID: "srv-1", Enabled: true, Command: "mcp-server"}); err != nil {
		t.Fatalf("RefreshTools() error = %v", err)
	}

	result, err := manager.CallTool(context.Background(), "srv-1", ToolCall{Name: "tool", Arguments: map[string]any{"x": 1}})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.Content != "ok" {
		t.Fatalf("Content = %q, want ok", result.Content)
	}
	if client.lastCall.Name != "tool" || client.lastCall.Arguments["x"] != 1 {
		t.Fatalf("lastCall = %#v", client.lastCall)
	}
}

func TestCloseWaitsForInflightToolCall(t *testing.T) {
	release := make(chan struct{})
	client := &fakeClient{tools: []Tool{{Name: "slow"}}, blockCall: release}
	manager := NewManager(WithConnector(ConnectorFunc(func(context.Context, ServerConfig) (Client, error) {
		return client, nil
	})))
	if _, err := manager.RefreshTools(context.Background(), ServerConfig{ID: "srv-1", Enabled: true, Command: "mcp-server"}); err != nil {
		t.Fatalf("RefreshTools() error = %v", err)
	}

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		_, err := manager.CallTool(context.Background(), "srv-1", ToolCall{Name: "slow"})
		done <- err
	}()
	<-started
	client.waitCallStarted(t)

	closeDone := make(chan error, 1)
	go func() { closeDone <- manager.Close() }()
	select {
	case <-closeDone:
		t.Fatal("Close returned before in-flight call finished")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestServerConfigEnvironmentLoadsAndOverridesEnvFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	if err := os.WriteFile(envFile, []byte("A=file\nB='quoted'\n"), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	env, err := (ServerConfig{
		EnvFile: envFile,
		Env:     map[string]string{"A": "config"},
	}).Environment()
	if err != nil {
		t.Fatalf("Environment returned error: %v", err)
	}
	if env["A"] != "config" {
		t.Fatalf("env[A] = %q, want config", env["A"])
	}
	if env["B"] != "quoted" {
		t.Fatalf("env[B] = %q, want quoted", env["B"])
	}
}

func TestRefreshToolsExpandsHomeCommand(t *testing.T) {
	var got ServerConfig
	manager := NewManager(WithConnector(ConnectorFunc(func(_ context.Context, cfg ServerConfig) (Client, error) {
		got = cfg
		return &fakeClient{tools: []Tool{{Name: "tool"}}}, nil
	})))

	if _, err := manager.RefreshTools(context.Background(), ServerConfig{
		ID:      "srv-1",
		Enabled: true,
		Command: "~/bin/mcp-server",
	}); err != nil {
		t.Fatalf("RefreshTools() error = %v", err)
	}
	if got.Command == "" || got.Command[0] == '~' {
		t.Fatalf("Command = %q, want expanded home path", got.Command)
	}
}

func TestMark3LabsConnectorValidatesRemoteURL(t *testing.T) {
	connector := Mark3LabsConnector{}
	if _, err := connector.Connect(context.Background(), ServerConfig{Name: "remote", Type: TransportSSE}); err == nil {
		t.Fatal("Connect(sse without url) error = nil, want error")
	}
	if _, err := connector.Connect(context.Background(), ServerConfig{Name: "remote", Type: TransportHTTP}); err == nil {
		t.Fatal("Connect(http without url) error = nil, want error")
	}
}

type fakeClient struct {
	tools       []Tool
	listErr     error
	result      CallResult
	callErr     error
	blockCall   chan struct{}
	callStarted sync.Once
	callCh      chan struct{}
	lastCall    ToolCall
	closed      bool
}

func (c *fakeClient) ListTools(context.Context) ([]Tool, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	return c.tools, nil
}

func (c *fakeClient) Close() error {
	c.closed = true
	return nil
}

func (c *fakeClient) CallTool(_ context.Context, call ToolCall) (CallResult, error) {
	c.lastCall = call
	if c.blockCall != nil {
		c.callStarted.Do(func() {
			c.callCh = make(chan struct{})
			close(c.callCh)
		})
		<-c.blockCall
	}
	if c.callErr != nil {
		return CallResult{}, c.callErr
	}
	return c.result, nil
}

func (c *fakeClient) waitCallStarted(t *testing.T) {
	t.Helper()
	for i := 0; i < 20; i++ {
		if c.callCh != nil {
			<-c.callCh
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for CallTool to start")
}

func waitStatus(t *testing.T, ch <-chan ServerStatus) ServerStatus {
	t.Helper()
	select {
	case status := <-ch:
		return status
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for status")
	}
	return ServerStatus{}
}
