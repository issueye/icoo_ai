package audit

import "time"

type EventType string

const (
	EventSessionCreated EventType = "session_created"
	EventSessionEnded   EventType = "session_ended"
	EventModelCall      EventType = "model_call"
	EventToolCall       EventType = "tool_call"
	EventFileChange     EventType = "file_change"
	EventShellCommand   EventType = "shell_command"
	EventNetworkAccess  EventType = "network_access"
	EventMCPCall        EventType = "mcp_call"
	EventPolicyDecision EventType = "policy_decision"
	EventSkillUse       EventType = "skill_use"
	EventHookUse        EventType = "hook_use"
	EventSubagentRun    EventType = "subagent_run"
)

type Event struct {
	ID        string         `json:"id,omitempty"`
	Type      EventType      `json:"type"`
	SessionID string         `json:"session_id,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Summary   string         `json:"summary,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}
