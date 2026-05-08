package acp

import (
	"encoding/json"
	"testing"

	sdk "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/internal/agent"
)

func TestMapPromptRequestTextAndMetadata(t *testing.T) {
	messageID := "11111111-1111-1111-1111-111111111111"
	req := sdk.PromptRequest{
		SessionId: sdk.SessionId("s1"),
		MessageId: &messageID,
		Meta:      map[string]any{"source": "test"},
		Prompt: []sdk.ContentBlock{
			sdk.TextBlock("hello"),
			sdk.ResourceLinkBlock("README", "file:///repo/README.md"),
		},
	}

	got := mapPromptRequest(req)
	if got.SessionID != "s1" {
		t.Fatalf("SessionID = %q", got.SessionID)
	}
	if got.Prompt != "hello\n\n[README](file:///repo/README.md)" {
		t.Fatalf("Prompt = %q", got.Prompt)
	}
	if got.Metadata["source"] != "test" {
		t.Fatalf("source metadata = %#v", got.Metadata["source"])
	}
	if got.Metadata["message_id"] != messageID {
		t.Fatalf("message_id metadata = %#v", got.Metadata["message_id"])
	}
}

func TestMapNewSessionRequest(t *testing.T) {
	req := sdk.NewSessionRequest{
		Cwd:                   "E:/repo",
		AdditionalDirectories: []string{"E:/other"},
		Meta:                  map[string]any{"client": "unit"},
	}

	got := mapNewSessionRequest(req)
	if got.CWD != "E:/repo" {
		t.Fatalf("CWD = %q", got.CWD)
	}
	dirs, ok := got.Metadata["additional_directories"].([]string)
	if !ok || len(dirs) != 1 || dirs[0] != "E:/other" {
		t.Fatalf("additional_directories = %#v", got.Metadata["additional_directories"])
	}
	if got.Metadata["client"] != "unit" {
		t.Fatalf("client metadata = %#v", got.Metadata["client"])
	}
}

func TestMapSessionEventMessageDelta(t *testing.T) {
	update, ok := mapSessionEvent(agent.Event{
		Type:      agent.EventMessageDelta,
		SessionID: "s1",
		Content:   "hello",
	})
	if !ok {
		t.Fatal("expected update")
	}
	if update.AgentMessageChunk == nil {
		t.Fatalf("AgentMessageChunk is nil: %#v", update)
	}
	if update.AgentMessageChunk.Content.Text == nil || update.AgentMessageChunk.Content.Text.Text != "hello" {
		t.Fatalf("agent message content = %#v", update.AgentMessageChunk.Content)
	}
}

func TestMapSessionEventToolLifecycle(t *testing.T) {
	start, ok := mapSessionEvent(agent.Event{
		Type: agent.EventToolCallStarted,
		Data: map[string]any{
			"id":   "tc1",
			"name": "read_file",
			"path": "README.md",
		},
	})
	if !ok {
		t.Fatal("expected start update")
	}
	if start.ToolCall == nil {
		t.Fatalf("ToolCall is nil: %#v", start)
	}
	if start.ToolCall.ToolCallId != "tc1" || start.ToolCall.Kind != sdk.ToolKindRead {
		t.Fatalf("tool call = %#v", start.ToolCall)
	}
	if len(start.ToolCall.Locations) != 1 || start.ToolCall.Locations[0].Path != "README.md" {
		t.Fatalf("locations = %#v", start.ToolCall.Locations)
	}

	done, ok := mapSessionEvent(agent.Event{
		Type:    agent.EventToolCallCompleted,
		Content: "file content",
		Data: map[string]any{
			"id":     "tc1",
			"result": map[string]any{"ok": true},
		},
	})
	if !ok {
		t.Fatal("expected done update")
	}
	if done.ToolCallUpdate == nil {
		t.Fatalf("ToolCallUpdate is nil: %#v", done)
	}
	if done.ToolCallUpdate.Status == nil || *done.ToolCallUpdate.Status != sdk.ToolCallStatusCompleted {
		t.Fatalf("status = %#v", done.ToolCallUpdate.Status)
	}
	if len(done.ToolCallUpdate.Content) != 1 {
		t.Fatalf("content = %#v", done.ToolCallUpdate.Content)
	}
}

func TestMapSessionEventPlanUpdated(t *testing.T) {
	update, ok := mapSessionEvent(agent.Event{
		Type: agent.EventPlanUpdated,
		Data: map[string]any{
			"entries": []any{
				map[string]any{"content": "Read code", "status": "completed", "priority": "high"},
				map[string]any{"step": "Write tests", "status": "in_progress"},
			},
		},
	})
	if !ok {
		t.Fatal("expected plan update")
	}
	if update.Plan == nil || len(update.Plan.Entries) != 2 {
		t.Fatalf("plan = %#v", update.Plan)
	}
	if update.Plan.Entries[0].Status != sdk.PlanEntryStatusCompleted {
		t.Fatalf("first status = %s", update.Plan.Entries[0].Status)
	}
	if update.Plan.Entries[1].Status != sdk.PlanEntryStatusInProgress {
		t.Fatalf("second status = %s", update.Plan.Entries[1].Status)
	}
}

func TestMapSessionEventMarshalsACPUnion(t *testing.T) {
	update, ok := mapSessionEvent(agent.Event{Type: agent.EventMessageDelta, Content: "hello"})
	if !ok {
		t.Fatal("expected update")
	}
	raw, err := json.Marshal(update)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("invalid json: %s", raw)
	}
}
