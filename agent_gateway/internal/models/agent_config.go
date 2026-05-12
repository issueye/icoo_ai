package models

type AgentConfig struct {
	BaseModel
	Name        string   `json:"name" gorm:"size:256;not null"`
	Protocol    string   `json:"protocol,omitempty" gorm:"size:64"`
	Description string   `json:"description,omitempty" gorm:"size:2048"`
	Models      []string `json:"models,omitempty" gorm:"serializer:json;type:text"`
	Enabled     bool     `json:"enabled" gorm:"not null"`
}
