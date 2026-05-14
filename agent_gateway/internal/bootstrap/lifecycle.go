package bootstrap

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

func (c *Container) Start(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if c.Managers.ACP != nil && c.Services.Agents != nil {
		agents, err := c.Services.Agents.List(ctx, c.ConfiguredListQuery())
		if err != nil {
			return err
		}
		if err := c.Managers.ACP.SyncAgents(ctx, agents); err != nil {
			return err
		}
	}
	if c.Managers.Scheduler != nil {
		return c.Managers.Scheduler.Start(ctx)
	}
	return nil
}

func (c *Container) ConfiguredListQuery() models.PageQuery {
	return models.PageQuery{Page: 1, PageSize: 200}
}
