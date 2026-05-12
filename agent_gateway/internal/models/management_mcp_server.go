package models

type ManagementMCPServer struct {
	BaseModel
	Name     string `gorm:"size:256;not null"`
	Command  string `gorm:"size:2048"`
	ArgsJSON string `gorm:"type:text"`
	Enabled  bool   `gorm:"not null"`
	Position int    `gorm:"not null;index"`
}

func (ManagementMCPServer) TableName() string { return "management_mcp_servers" }
