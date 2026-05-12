package models

type ManagementAgent struct {
	BaseModel
	Name        string `gorm:"size:256;not null"`
	Protocol    string `gorm:"size:64"`
	Description string `gorm:"size:2048"`
	ModelsJSON  string `gorm:"type:text"`
	Enabled     bool   `gorm:"not null"`
	Position    int    `gorm:"not null;index"`
}

func (ManagementAgent) TableName() string { return "management_agents" }
