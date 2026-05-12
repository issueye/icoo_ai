package models

type ChannelConfig struct {
	BaseModel
	Name       string `json:"name" gorm:"size:256;not null"`
	Type       string `json:"type" gorm:"size:64;not null"`
	Enabled    bool   `json:"enabled" gorm:"not null"`
	AppID      string `json:"appId,omitempty" gorm:"size:1024"`
	AppSecret  string `json:"appSecret,omitempty" gorm:"size:2048"`
	BotToken   string `json:"botToken,omitempty" gorm:"size:2048"`
	WebhookURL string `json:"webhookUrl,omitempty" gorm:"size:2048"`
}
