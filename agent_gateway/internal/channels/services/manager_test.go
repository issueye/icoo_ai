package services

import (
	"context"
	"testing"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/channels/models"
)

func TestManagerInitializeStartStopAndStatus(t *testing.T) {
	manager := NewManager(NewDefaultFactoryRegistry(), nil)
	ctx := context.Background()

	err := manager.Initialize(ctx, []models.ChannelConfig{
		{ID: "qq1", Name: "QQ One", Type: models.ChannelTypeQQ, Enabled: true},
		{ID: "wx1", Name: "Weixin One", Type: models.ChannelTypeWeixin, Enabled: false},
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if err := manager.StartEnabled(ctx); err != nil {
		t.Fatalf("StartEnabled() error = %v", err)
	}

	statuses := manager.Status()
	if len(statuses) != 2 {
		t.Fatalf("len(Status()) = %d, want 2", len(statuses))
	}

	var qqStatus, wxStatus *models.ChannelStatus
	for i := range statuses {
		status := statuses[i]
		if status.ID == "qq1" {
			qqStatus = &status
		}
		if status.ID == "wx1" {
			wxStatus = &status
		}
	}
	if qqStatus == nil || qqStatus.State != models.StateRunning {
		t.Fatalf("qq status = %#v", qqStatus)
	}
	if wxStatus == nil || wxStatus.State != models.StateDisabled {
		t.Fatalf("weixin status = %#v", wxStatus)
	}

	if err := manager.StopAll(ctx); err != nil {
		t.Fatalf("StopAll() error = %v", err)
	}
	statuses = manager.Status()
	for _, status := range statuses {
		if status.ID == "qq1" && status.State != models.StateStopped {
			t.Fatalf("qq stopped state = %s, want %s", status.State, models.StateStopped)
		}
	}
}
