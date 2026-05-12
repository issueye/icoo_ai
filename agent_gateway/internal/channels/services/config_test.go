package services

import (
	"testing"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

func TestNormalizeConfigsFillsDefaultsAndTrims(t *testing.T) {
	configs, err := NormalizeConfigs([]models.ChannelRuntimeConfig{
		{
			BaseModel:  models.BaseModel{ID: "  "},
			Name:       "  ",
			Type:       "  ",
			Enabled:    true,
			AppID:      "  app  ",
			AppSecret:  "  secret  ",
			BotToken:   " token ",
			WebhookURL: " https://example.com/hook ",
		},
	})
	if err != nil {
		t.Fatalf("NormalizeConfigs() error = %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("len(configs) = %d, want 1", len(configs))
	}
	got := configs[0]
	if got.ID != "channel_1" {
		t.Fatalf("ID = %q, want channel_1", got.ID)
	}
	if got.Name != "channel_1" {
		t.Fatalf("Name = %q, want channel_1", got.Name)
	}
	if got.Type != models.ChannelTypeQQ {
		t.Fatalf("Type = %q, want qq", got.Type)
	}
	if got.AppID != "app" || got.AppSecret != "secret" || got.BotToken != "token" || got.WebhookURL != "https://example.com/hook" {
		t.Fatalf("trim mismatch: %+v", got)
	}
}

func TestNormalizeConfigsRejectsUnsupportedType(t *testing.T) {
	_, err := NormalizeConfigs([]models.ChannelRuntimeConfig{
		{BaseModel: models.BaseModel{ID: "c1"}, Type: "unknown"},
	})
	if err == nil {
		t.Fatal("NormalizeConfigs() error = nil, want unsupported type error")
	}
}

func TestNormalizeConfigsRejectsDuplicateIDs(t *testing.T) {
	_, err := NormalizeConfigs([]models.ChannelRuntimeConfig{
		{BaseModel: models.BaseModel{ID: "dup"}, Type: models.ChannelTypeQQ},
		{BaseModel: models.BaseModel{ID: "dup"}, Type: models.ChannelTypeMQTT},
	})
	if err == nil {
		t.Fatal("NormalizeConfigs() error = nil, want duplicate id error")
	}
}
