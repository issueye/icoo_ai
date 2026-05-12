package models

type AgentProfile struct {
	BaseModel
	Name        string   `json:"name" gorm:"size:256;not null"`
	Protocol    string   `json:"protocol" gorm:"size:64;not null"`
	Command     string   `json:"command,omitempty" gorm:"size:2048"`
	Args        []string `json:"args,omitempty" gorm:"serializer:json;type:text"`
	Endpoint    string   `json:"endpoint,omitempty" gorm:"size:2048"`
	Models      []string `json:"models,omitempty" gorm:"serializer:json;type:text"`
	Description string   `json:"description,omitempty" gorm:"size:2048"`
}
