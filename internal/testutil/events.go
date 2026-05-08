package testutil

import (
	"context"
	"sync"

	"github.com/icoo-ai/icoo-ai/internal/agent"
)

type RuntimeEventCollector struct {
	mu     sync.Mutex
	events []agent.Event
}

func NewRuntimeEventCollector() *RuntimeEventCollector {
	return &RuntimeEventCollector{}
}

func CollectRuntimeEvents(ctx context.Context, events <-chan agent.Event) ([]agent.Event, error) {
	collector := NewRuntimeEventCollector()
	return collector.Collect(ctx, events)
}

func (c *RuntimeEventCollector) Collect(ctx context.Context, events <-chan agent.Event) ([]agent.Event, error) {
	for {
		select {
		case <-ctx.Done():
			return c.Events(), ctx.Err()
		case event, ok := <-events:
			if !ok {
				return c.Events(), nil
			}
			c.Add(event)
		}
	}
}

func (c *RuntimeEventCollector) Add(event agent.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.events = append(c.events, event)
}

func (c *RuntimeEventCollector) Events() []agent.Event {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]agent.Event(nil), c.events...)
}

func (c *RuntimeEventCollector) Contents() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	contents := make([]string, 0, len(c.events))
	for _, event := range c.events {
		if event.Content != "" {
			contents = append(contents, event.Content)
		}
	}
	return contents
}
