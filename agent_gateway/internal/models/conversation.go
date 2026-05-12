package models

import "time"

type Conversation struct {
	BaseModel
	AgentID   string    `json:"agentId" gorm:"size:128;not null;index"`
	SessionID string    `json:"sessionId" gorm:"size:128;not null;uniqueIndex"`
	RunID     string    `json:"runId,omitempty" gorm:"size:128;index"`
	Title     string    `json:"title,omitempty" gorm:"size:512"`
	Status    string    `json:"status,omitempty" gorm:"size:64;index"`
	Model     string    `json:"model,omitempty" gorm:"size:128"`
	CWD       string    `json:"cwd,omitempty" gorm:"size:2048"`
	Summary   string    `json:"summary,omitempty" gorm:"type:text"`
	SafeMeta  SafeMeta  `json:"safeMeta,omitempty" gorm:"serializer:json;type:text"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt,omitempty" gorm:"autoUpdateTime"`
}
