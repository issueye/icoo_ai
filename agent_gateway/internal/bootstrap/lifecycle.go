package bootstrap

import "context"

func (c *Container) Start(ctx context.Context) error {
	if c == nil || c.Managers.Scheduler == nil {
		return nil
	}
	return c.Managers.Scheduler.Start(ctx)
}
