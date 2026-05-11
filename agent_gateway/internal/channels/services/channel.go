package services

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/channels/models"
)

type Channel interface {
	ID() string
	Name() string
	Type() models.ChannelType
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type ChannelFactory func(cfg models.ChannelConfig) (Channel, error)
