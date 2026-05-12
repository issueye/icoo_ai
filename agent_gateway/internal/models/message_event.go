package models

import "time"

type MessageEvent struct {
	BaseModel
	Type      string    `json:"type" gorm:"size:128;not null;index"`
	AgentID   string    `json:"agentId" gorm:"size:128;index"`
	SessionID string    `json:"sessionId" gorm:"size:128;not null;index"`
	RunID     string    `json:"runId" gorm:"size:128;index"`
	Role      string    `json:"role,omitempty" gorm:"size:64"`
	Status    string    `json:"status,omitempty" gorm:"size:64;index"`
	Summary   string    `json:"summary,omitempty" gorm:"type:text"`
	SafeMeta  SafeMeta  `json:"safeMeta,omitempty" gorm:"serializer:json;type:text"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime;index"`
}
