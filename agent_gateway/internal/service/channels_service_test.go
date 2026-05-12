package service

import (
	"context"
	"testing"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

func TestUpdateManagementSettingsAppliesChannelsLifecycle(t *testing.T) {
	svc := NewGatewayServiceWithStore(store.NewMemoryStore())
	ctx := context.Background()

	_, err := svc.UpdateManagementSettings(ctx, models.ManagementSettings{
		Channels: []models.ChannelConfig{
			{BaseModel: models.BaseModel{ID: "qq-1"}, Name: "QQ One", Type: "qq", Enabled: true},
			{BaseModel: models.BaseModel{ID: "wx-1"}, Name: "WX One", Type: "weixin", Enabled: true},
			{BaseModel: models.BaseModel{ID: "fs-1"}, Name: "FS One", Type: "feishu", Enabled: true},
			{BaseModel: models.BaseModel{ID: "mq-1"}, Name: "MQ One", Type: "mqtt", Enabled: false},
		},
	})
	if err != nil {
		t.Fatalf("UpdateManagementSettings() error = %v", err)
	}

	statuses, err := svc.GetChannelStatuses(ctx)
	if err != nil {
		t.Fatalf("GetChannelStatuses() error = %v", err)
	}
	if len(statuses) != 4 {
		t.Fatalf("len(statuses) = %d, want 4", len(statuses))
	}

	states := map[string]string{}
	for _, item := range statuses {
		states[item.ID] = string(item.State)
	}
	if states["qq-1"] != "running" || states["wx-1"] != "running" || states["fs-1"] != "running" {
		t.Fatalf("enabled channel states = %#v", states)
	}
	if states["mq-1"] != "disabled" {
		t.Fatalf("mqtt state = %q, want disabled", states["mq-1"])
	}

	if err := svc.StopChannels(ctx); err != nil {
		t.Fatalf("StopChannels() error = %v", err)
	}

	statuses, err = svc.GetChannelStatuses(ctx)
	if err != nil {
		t.Fatalf("GetChannelStatuses() after stop error = %v", err)
	}
	states = map[string]string{}
	for _, item := range statuses {
		states[item.ID] = string(item.State)
	}
	if states["qq-1"] != "stopped" || states["wx-1"] != "stopped" || states["fs-1"] != "stopped" {
		t.Fatalf("stopped channel states = %#v", states)
	}
	if states["mq-1"] != "disabled" {
		t.Fatalf("disabled channel state after stop = %q", states["mq-1"])
	}
}
