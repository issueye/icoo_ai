package services

import (
	"context"
	"sync"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type stubChannel struct {
	mu      sync.Mutex
	id      string
	name    string
	typ     models.ChannelType
	running bool
}

func NewStubChannel(cfg models.ChannelRuntimeConfig) Channel {
	return &stubChannel{
		id:   cfg.ID,
		name: cfg.Name,
		typ:  cfg.Type,
	}
}

func (c *stubChannel) ID() string {
	return c.id
}

func (c *stubChannel) Name() string {
	return c.name
}

func (c *stubChannel) Type() models.ChannelType {
	return c.typ
}

func (c *stubChannel) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = true
	// TODO: Replace stub with a real channel adapter implementation.
	return nil
}

func (c *stubChannel) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	// TODO: Replace stub with graceful shutdown for the real adapter.
	return nil
}
