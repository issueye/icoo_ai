package models

type ManagementChannel struct {
	BaseModel
	Name       string `gorm:"size:256;not null"`
	Type       string `gorm:"size:64;not null"`
	Enabled    bool   `gorm:"not null"`
	AppID      string `gorm:"size:1024"`
	AppSecret  string `gorm:"size:2048"`
	BotToken   string `gorm:"size:2048"`
	WebhookURL string `gorm:"size:2048"`
	Position   int    `gorm:"not null;index"`
}

func (ManagementChannel) TableName() string { return "management_channels" }
