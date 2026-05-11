package services

import (
	"fmt"
	"sync"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/channels/models"
)

type FactoryRegistry struct {
	mu        sync.RWMutex
	factories map[models.ChannelType]ChannelFactory
}

func NewFactoryRegistry() *FactoryRegistry {
	return &FactoryRegistry{
		factories: map[models.ChannelType]ChannelFactory{},
	}
}

func (r *FactoryRegistry) Register(channelType models.ChannelType, factory ChannelFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[channelType] = factory
}

func (r *FactoryRegistry) Create(cfg models.ChannelConfig) (Channel, error) {
	r.mu.RLock()
	factory, ok := r.factories[cfg.Type]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("channel type %q is not supported", cfg.Type)
	}
	return factory(cfg)
}

func (r *FactoryRegistry) Types() []models.ChannelType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]models.ChannelType, 0, len(r.factories))
	for t := range r.factories {
		out = append(out, t)
	}
	return out
}
