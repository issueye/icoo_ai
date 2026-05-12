package runtime

import (
	"context"
	"fmt"
	"os"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/projection"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type eventProjector struct {
	cancel context.CancelFunc
	sub    *events.Subscription
	done   chan struct{}
}

func startEventProjector(bus *events.Bus, st store.Store) *eventProjector {
	if bus == nil || st == nil {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	sub, buffered := bus.Subscribe(ctx, "")
	p := &eventProjector{
		cancel: cancel,
		sub:    sub,
		done:   make(chan struct{}),
	}
	go p.run(ctx, st, buffered)
	return p
}

func (p *eventProjector) Stop() {
	if p == nil {
		return
	}
	p.cancel()
	p.sub.Close()
	<-p.done
}

func (p *eventProjector) run(ctx context.Context, st store.Store, buffered []models.EventEnvelope) {
	defer close(p.done)

	for _, event := range buffered {
		if _, err := projection.Apply(ctx, st, event); err != nil {
			fmt.Fprintf(os.Stderr, "agent-gateway event projection failed: %v\n", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-p.sub.Events():
			if !ok {
				return
			}
			if _, err := projection.Apply(ctx, st, event); err != nil {
				fmt.Fprintf(os.Stderr, "agent-gateway event projection failed: %v\n", err)
			}
		}
	}
}
