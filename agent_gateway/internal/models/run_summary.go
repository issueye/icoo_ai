package models

import "time"

type RunSummary struct {
	BaseModel
	AgentID     string     `json:"agentId" gorm:"size:128;not null;index"`
	SessionID   string     `json:"sessionId" gorm:"size:128;not null;index"`
	RunID       string     `json:"runId" gorm:"size:128;not null;uniqueIndex"`
	Status      string     `json:"status" gorm:"size:64;not null;index"`
	Summary     string     `json:"summary,omitempty" gorm:"type:text"`
	SafeMeta    SafeMeta   `json:"safeMeta,omitempty" gorm:"serializer:json;type:text"`
	CreatedAt   time.Time  `json:"createdAt" gorm:"autoCreateTime;index"`
	UpdatedAt   time.Time  `json:"updatedAt,omitempty" gorm:"autoUpdateTime"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}
