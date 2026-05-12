package models

import "time"

type Run struct {
	BaseModel
	SessionID string     `json:"sessionId" gorm:"size:128;not null;index"`
	AgentID   string     `json:"agentId" gorm:"size:128;not null;index"`
	Status    string     `json:"status" gorm:"size:64;not null;index"`
	StartedAt time.Time  `json:"startedAt" gorm:"index"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
}
