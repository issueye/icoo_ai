package models

import "time"

type AuditEvent struct {
	BaseModel
	Type      string    `json:"type" gorm:"size:128;not null;index"`
	Level     string    `json:"level,omitempty" gorm:"size:64;index"`
	AgentID   string    `json:"agentId" gorm:"size:128;index"`
	SessionID string    `json:"sessionId,omitempty" gorm:"size:128;index"`
	RunID     string    `json:"runId,omitempty" gorm:"size:128;index"`
	Summary   string    `json:"summary" gorm:"type:text;not null"`
	SafeMeta  SafeMeta  `json:"safeMeta,omitempty" gorm:"serializer:json;type:text"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime;index"`
}
