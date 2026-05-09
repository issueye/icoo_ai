package events

import "time"

// Envelope is the unified event payload for gateway event streaming.
type Envelope struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	AgentID   string    `json:"agentId,omitempty"`
	SessionID string    `json:"sessionId,omitempty"`
	RunID     string    `json:"runId,omitempty"`
	Payload   any       `json:"payload"`
	CreatedAt time.Time `json:"createdAt"`
}
