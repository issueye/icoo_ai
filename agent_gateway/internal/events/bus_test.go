package events

import (
	"context"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

func TestSubscribeReplaysEventsAfterLastEventID(t *testing.T) {
	bus := NewBus(4)
	bus.Publish(models.EventEnvelope{BaseModel: models.BaseModel{ID: "evt_1"}, Type: "one"})
	bus.Publish(models.EventEnvelope{BaseModel: models.BaseModel{ID: "evt_2"}, Type: "two"})
	bus.Publish(models.EventEnvelope{BaseModel: models.BaseModel{ID: "evt_3"}, Type: "three"})

	sub, buffered := bus.Subscribe(context.Background(), "evt_1")
	defer sub.Close()

	if len(buffered) != 2 {
		t.Fatalf("buffered length = %d, want 2", len(buffered))
	}
	if buffered[0].ID != "evt_2" || buffered[1].ID != "evt_3" {
		t.Fatalf("buffered = %#v, want evt_2 and evt_3", buffered)
	}
}

func TestSubscribeClosesWhenContextDone(t *testing.T) {
	bus := NewBus(4)
	ctx, cancel := context.WithCancel(context.Background())
	sub, _ := bus.Subscribe(ctx, "")
	cancel()

	select {
	case _, ok := <-sub.Events():
		if ok {
			t.Fatal("subscription channel is open, want closed")
		}
	case <-time.After(time.Second):
		t.Fatal("subscription did not close after context cancellation")
	}
}

func TestPublishDoesNotBlockOnFullSubscriber(t *testing.T) {
	bus := NewBus(4)
	sub, _ := bus.Subscribe(context.Background(), "")
	defer sub.Close()

	for i := 0; i < defaultSubscriberBufferSize+16; i++ {
		bus.Publish(models.EventEnvelope{BaseModel: models.BaseModel{ID: "evt"}})
	}
}

func TestPublishAfterCloseIsSafe(t *testing.T) {
	bus := NewBus(4)
	sub, _ := bus.Subscribe(context.Background(), "")
	sub.Close()

	bus.Publish(models.EventEnvelope{BaseModel: models.BaseModel{ID: "evt_1"}})
}
