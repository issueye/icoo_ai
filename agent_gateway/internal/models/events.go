package models

import "time"

// EventEnvelope is the unified event payload for gateway event streaming.
type EventEnvelope struct {
	BaseModel
	Type      string    `json:"type" gorm:"size:128;not null;index"`
	AgentID   string    `json:"agentId,omitempty" gorm:"size:128;index"`
	SessionID string    `json:"sessionId,omitempty" gorm:"size:128;index"`
	RunID     string    `json:"runId,omitempty" gorm:"size:128;index"`
	Payload   any       `json:"payload" gorm:"serializer:json;type:text"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime;index"`
}
