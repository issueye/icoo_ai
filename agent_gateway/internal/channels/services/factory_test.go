package services

import (
	"testing"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

func TestDefaultFactoryRegistryCreatesFourChannelTypes(t *testing.T) {
	registry := NewDefaultFactoryRegistry()
	testCases := []models.ChannelType{
		models.ChannelTypeQQ,
		models.ChannelTypeWeixin,
		models.ChannelTypeFeishu,
		models.ChannelTypeMQTT,
	}

	for _, channelType := range testCases {
		channel, err := registry.Create(models.ChannelRuntimeConfig{
			BaseModel: models.BaseModel{ID: "c-" + string(channelType)},
			Name:      "test",
			Type:      channelType,
			Enabled:   true,
		})
		if err != nil {
			t.Fatalf("Create(%s) error = %v", channelType, err)
		}
		if channel.Type() != channelType {
			t.Fatalf("Create(%s) type = %s", channelType, channel.Type())
		}
	}
}
