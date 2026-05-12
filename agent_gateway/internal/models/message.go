package models

import "time"

type Message struct {
	BaseModel
	SessionID string    `json:"sessionId" gorm:"size:128;not null;index"`
	RunID     string    `json:"runId,omitempty" gorm:"size:128;index"`
	Role      string    `json:"role" gorm:"size:64;not null"`
	Content   string    `json:"content" gorm:"type:text"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
}
