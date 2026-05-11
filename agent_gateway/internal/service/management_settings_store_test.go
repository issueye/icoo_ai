package service

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSQLiteManagementSettingsStore_SaveLoad(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "management.db")
	store, err := NewSQLiteManagementSettingsStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManagementSettingsStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("store.Close() error = %v", err)
		}
	})

	input := ManagementSettings{
		Channels: []ChannelConfig{
			{ID: "qq_1", Name: "QQ 1", Type: "qq", Enabled: true, AppID: "a1", AppSecret: "s1", BotToken: "b1", WebhookURL: "w1"},
		},
		MCPServers: []MCPServerConfig{
			{ID: "mcp_1", Name: "MCP 1", Command: "node", Args: []string{"server.js"}, Enabled: true},
		},
		ScheduleTasks: []ScheduleTaskConfig{
			{ID: "task_1", Name: "Task 1", Spec: "*/5 * * * *", Content: "每5分钟检查一次任务队列", Enabled: true},
		},
		Agents: []AgentConfig{
			{ID: "agent_1", Name: "Agent 1", Protocol: "acp", Description: "d1", Models: []string{"gpt-5.4"}, Enabled: true},
		},
	}
	if err := store.Save(context.Background(), input); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Channels) != 1 || got.Channels[0].ID != "qq_1" {
		t.Fatalf("unexpected channels: %#v", got.Channels)
	}
	if len(got.MCPServers) != 1 || got.MCPServers[0].ID != "mcp_1" || len(got.MCPServers[0].Args) != 1 {
		t.Fatalf("unexpected mcp servers: %#v", got.MCPServers)
	}
	if len(got.ScheduleTasks) != 1 || got.ScheduleTasks[0].ID != "task_1" || got.ScheduleTasks[0].Content == "" {
		t.Fatalf("unexpected schedule tasks: %#v", got.ScheduleTasks)
	}
	if len(got.Agents) != 1 || got.Agents[0].ID != "agent_1" || len(got.Agents[0].Models) != 1 {
		t.Fatalf("unexpected agents: %#v", got.Agents)
	}
}

func TestSQLiteManagementSettingsStore_LoadNotFound(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "management.db")
	store, err := NewSQLiteManagementSettingsStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManagementSettingsStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("store.Close() error = %v", err)
		}
	})

	_, err = store.Load(context.Background())
	if err == nil {
		t.Fatal("Load() error = nil, want ErrManagementSettingsNotFound")
	}
	if err != ErrManagementSettingsNotFound {
		t.Fatalf("Load() error = %v, want %v", err, ErrManagementSettingsNotFound)
	}
}

func TestSQLiteManagementSettingsStore_SaveEmptyThenLoadEmpty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "management.db")
	store, err := NewSQLiteManagementSettingsStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteManagementSettingsStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("store.Close() error = %v", err)
		}
	})

	if err := store.Save(context.Background(), ManagementSettings{}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(got.Agents) != 0 || len(got.MCPServers) != 0 || len(got.ScheduleTasks) != 0 || len(got.Channels) != 0 {
		t.Fatalf("expected empty management settings, got %#v", got)
	}
}
