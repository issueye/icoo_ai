package models

import "time"

type ChannelType string

const (
	ChannelTypeQQ     ChannelType = "qq"
	ChannelTypeWeixin ChannelType = "weixin"
	ChannelTypeFeishu ChannelType = "feishu"
	ChannelTypeMQTT   ChannelType = "mqtt"
)

type ChannelConfig struct {
	ID         string
	Name       string
	Type       ChannelType
	Enabled    bool
	AppID      string
	AppSecret  string
	BotToken   string
	WebhookURL string
}

type LifecycleState string

const (
	StateInitialized LifecycleState = "initialized"
	StateRunning     LifecycleState = "running"
	StateStopped     LifecycleState = "stopped"
	StateDisabled    LifecycleState = "disabled"
	StateError       LifecycleState = "error"
)

type ChannelStatus struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Type        ChannelType    `json:"type"`
	Enabled     bool           `json:"enabled"`
	State       LifecycleState `json:"state"`
	LastError   string         `json:"lastError,omitempty"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	StartedAt   *time.Time     `json:"startedAt,omitempty"`
	StoppedAt   *time.Time     `json:"stoppedAt,omitempty"`
	Initialized bool           `json:"initialized"`
}
