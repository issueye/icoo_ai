package models

type Channel struct {
	BaseModel
	Name          string `gorm:"size:256;not null;comment:渠道名称"`
	Type          string `gorm:"size:64;not null;comment:渠道类型"`
	Enabled       bool   `gorm:"not null;comment:是否启用渠道"`
	AppID         string `gorm:"size:-1;comment:渠道应用ID"`
	AppSecret     string `gorm:"size:-1;comment:渠道应用密钥"`
	BotToken      string `gorm:"size:-1;comment:渠道应用机器人令牌"`
	BotWebhookURL string `gorm:"size:-1;comment:渠道应用机器人URL"`
}

func (Channel) TableName() string { return "channels" }
