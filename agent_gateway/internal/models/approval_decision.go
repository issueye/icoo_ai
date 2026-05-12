package models

import "time"

type ApprovalDecision struct {
	BaseModel
	AgentID            string     `json:"agentId" gorm:"size:128;not null;index"`
	SessionID          string     `json:"sessionId" gorm:"size:128;not null;index"`
	RunID              string     `json:"runId" gorm:"size:128;not null;index"`
	ConnectorRequestID string     `json:"connectorRequestId" gorm:"size:128;not null;index"`
	Status             string     `json:"status" gorm:"size:64;not null;index"`
	Decision           string     `json:"decision,omitempty" gorm:"size:64"`
	Actor              string     `json:"actor,omitempty" gorm:"size:128"`
	Summary            string     `json:"summary,omitempty" gorm:"type:text"`
	SafeMeta           SafeMeta   `json:"safeMeta,omitempty" gorm:"serializer:json;type:text"`
	CreatedAt          time.Time  `json:"createdAt" gorm:"autoCreateTime;index"`
	UpdatedAt          time.Time  `json:"updatedAt,omitempty" gorm:"autoUpdateTime"`
	DecidedAt          *time.Time `json:"decidedAt,omitempty"`
}
