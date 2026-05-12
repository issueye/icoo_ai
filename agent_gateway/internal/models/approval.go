package models

import "time"

type Approval struct {
	BaseModel
	AgentID            string     `json:"agentId" gorm:"size:128;not null;index"`
	SessionID          string     `json:"sessionId" gorm:"size:128;not null;index"`
	RunID              string     `json:"runId" gorm:"size:128;not null;index"`
	ConnectorRequestID string     `json:"connectorRequestId" gorm:"size:128;not null;index"`
	Status             string     `json:"status" gorm:"size:64;not null;index"`
	Action             string     `json:"action" gorm:"size:128;not null"`
	Message            string     `json:"message" gorm:"type:text"`
	Decision           string     `json:"decision,omitempty" gorm:"size:64"`
	DecidedAt          *time.Time `json:"decidedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt" gorm:"autoCreateTime"`
}

type ApprovalDecisionRequest struct {
	Decision string `json:"decision"`
	Message  string `json:"message,omitempty"`
}
