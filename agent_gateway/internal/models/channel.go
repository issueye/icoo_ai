package models

import "time"

type ChannelType string

const (
	ChannelTypeQQ     ChannelType = "qq"
	ChannelTypeWeixin ChannelType = "weixin"
	ChannelTypeFeishu ChannelType = "feishu"
	ChannelTypeMQTT   ChannelType = "mqtt"
)

type ChannelRuntimeConfig struct {
	BaseModel
	Name       string      `gorm:"size:256;not null"`
	Type       ChannelType `gorm:"size:64;not null"`
	Enabled    bool        `gorm:"not null"`
	AppID      string      `gorm:"size:1024"`
	AppSecret  string      `gorm:"size:2048"`
	BotToken   string      `gorm:"size:2048"`
	WebhookURL string      `gorm:"size:2048"`
}

type LifecycleState string

const (
	StateInitialized LifecycleState = "initialized"
	StateRunning     LifecycleState = "running"
	StateStopped     LifecycleState = "stopped"
	StateDisabled    LifecycleState = "disabled"
	StateError       LifecycleState = "error"
)

type ChannelRuntimeStatus struct {
	BaseModel
	Name        string         `json:"name" gorm:"size:256;not null"`
	Type        ChannelType    `json:"type" gorm:"size:64;not null"`
	Enabled     bool           `json:"enabled" gorm:"not null"`
	State       LifecycleState `json:"state" gorm:"size:64;not null;index"`
	LastError   string         `json:"lastError,omitempty" gorm:"type:text"`
	UpdatedAt   time.Time      `json:"updatedAt" gorm:"autoUpdateTime"`
	StartedAt   *time.Time     `json:"startedAt,omitempty"`
	StoppedAt   *time.Time     `json:"stoppedAt,omitempty"`
	Initialized bool           `json:"initialized" gorm:"not null"`
}
