package models

type MCPServerConfig struct {
	BaseModel
	Name    string   `json:"name" gorm:"size:256;not null"`
	Command string   `json:"command,omitempty" gorm:"size:2048"`
	Args    []string `json:"args,omitempty" gorm:"serializer:json;type:text"`
	Enabled bool     `json:"enabled" gorm:"not null"`
}
