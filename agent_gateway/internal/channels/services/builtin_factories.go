package services

import "github.com/icoo-ai/icoo-ai/agent_gateway/internal/channels/models"

func NewDefaultFactoryRegistry() *FactoryRegistry {
	registry := NewFactoryRegistry()
	registerBuiltinFactories(registry)
	return registry
}

func registerBuiltinFactories(registry *FactoryRegistry) {
	registry.Register(models.ChannelTypeQQ, func(cfg models.ChannelConfig) (Channel, error) {
		return NewStubChannel(cfg), nil
	})
	registry.Register(models.ChannelTypeWeixin, func(cfg models.ChannelConfig) (Channel, error) {
		return NewStubChannel(cfg), nil
	})
	registry.Register(models.ChannelTypeFeishu, func(cfg models.ChannelConfig) (Channel, error) {
		return NewStubChannel(cfg), nil
	})
	registry.Register(models.ChannelTypeMQTT, func(cfg models.ChannelConfig) (Channel, error) {
		return NewStubChannel(cfg), nil
	})
}
