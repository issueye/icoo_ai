package models

import "time"

type Session struct {
	BaseModel
	Title                 string    `json:"title" gorm:"size:512;not null"`
	WorkspaceID           string    `json:"workspaceId,omitempty" gorm:"size:256;index"`
	CWD                   string    `json:"cwd,omitempty" gorm:"size:2048"`
	AdditionalDirectories []string  `json:"additionalDirectories,omitempty" gorm:"serializer:json;type:text"`
	StartupCommand        string    `json:"startupCommand,omitempty" gorm:"type:text"`
	Mode                  string    `json:"mode,omitempty" gorm:"size:128;index"`
	AgentID               string    `json:"agentId" gorm:"size:128;not null;index"`
	Model                 string    `json:"model,omitempty" gorm:"size:128"`
	Status                string    `json:"status" gorm:"size:64;not null;index"`
	CreatedAt             time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt             time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}
