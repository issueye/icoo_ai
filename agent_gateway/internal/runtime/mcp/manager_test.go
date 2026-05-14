package mcp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
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

type fakeClient struct {
	tools   []Tool
	listErr error
	closed  bool
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
