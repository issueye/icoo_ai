package services

import (
	"fmt"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

func NormalizeConfigs(in []models.ChannelRuntimeConfig) ([]models.ChannelRuntimeConfig, error) {
	out := make([]models.ChannelRuntimeConfig, 0, len(in))
	seenIDs := make(map[string]struct{}, len(in))
	for i, item := range in {
		cfg := item
		cfg.ID = strings.TrimSpace(cfg.ID)
		if cfg.ID == "" {
			cfg.ID = fmt.Sprintf("channel_%d", i+1)
		}
		if _, exists := seenIDs[cfg.ID]; exists {
			return nil, fmt.Errorf("duplicate channel id %q", cfg.ID)
		}
		seenIDs[cfg.ID] = struct{}{}

		cfg.Name = strings.TrimSpace(cfg.Name)
		if cfg.Name == "" {
			cfg.Name = cfg.ID
		}

		channelType := strings.ToLower(strings.TrimSpace(string(cfg.Type)))
		if channelType == "" {
			channelType = string(models.ChannelTypeQQ)
		}
		cfg.Type = models.ChannelType(channelType)

		cfg.AppID = strings.TrimSpace(cfg.AppID)
		cfg.AppSecret = strings.TrimSpace(cfg.AppSecret)
		cfg.BotToken = strings.TrimSpace(cfg.BotToken)
		cfg.WebhookURL = strings.TrimSpace(cfg.WebhookURL)

		if !isSupportedType(cfg.Type) {
			return nil, fmt.Errorf("unsupported channel type %q", cfg.Type)
		}
		out = append(out, cfg)
	}
	return out, nil
}

func isSupportedType(channelType models.ChannelType) bool {
	switch channelType {
	case models.ChannelTypeQQ, models.ChannelTypeWeixin, models.ChannelTypeFeishu, models.ChannelTypeMQTT:
		return true
	default:
		return false
	}
}
