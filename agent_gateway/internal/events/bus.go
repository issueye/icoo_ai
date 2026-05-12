package events

import (
	"context"
	"errors"
	"sync"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

const defaultBufferSize = 256
const defaultSubscriberBufferSize = 32

var ErrSubscriberQueueFull = errors.New("subscriber queue full")
var defaultBus = NewBus(defaultBufferSize)

type Bus struct {
	mu          sync.RWMutex
	subscribers map[*subscriber]struct{}
	ring        []models.EventEnvelope
	ringStart   int
	ringCount   int
}

type Subscription struct {
	sub  *subscriber
	bus  *Bus
	done chan struct{}
	once sync.Once
}

type subscriber struct {
	mu     sync.Mutex
	ch     chan models.EventEnvelope
	closed bool
}

func NewBus(bufferSize int) *Bus {
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}
	return &Bus{
		subscribers: make(map[*subscriber]struct{}),
		ring:        make([]models.EventEnvelope, bufferSize),
	}
}

func DefaultBus() *Bus {
	return defaultBus
}

func (b *Bus) Publish(event models.EventEnvelope) {
	b.mu.Lock()
	b.pushToRing(event)
	subscribers := make([]*subscriber, 0, len(b.subscribers))
	for sub := range b.subscribers {
		subscribers = append(subscribers, sub)
	}
	b.mu.Unlock()

	for _, sub := range subscribers {
		_ = sub.deliver(event)
	}
}

// Subscribe 创建一个事件订阅，并返回从 lastEventID 之后开始的缓冲事件快照。
// ctx 取消时订阅会自动关闭；调用方也可以主动调用 Subscription.Close。
func (b *Bus) Subscribe(ctx context.Context, lastEventID string) (*Subscription, []models.EventEnvelope) {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub := &subscriber{ch: make(chan models.EventEnvelope, defaultSubscriberBufferSize)}
	b.subscribers[sub] = struct{}{}

	buffered := b.snapshotSince(lastEventID)
	subscription := &Subscription{sub: sub, bus: b, done: make(chan struct{})}
	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				subscription.Close()
			case <-subscription.done:
			}
		}()
	}
	return subscription, buffered
}

func (b *Bus) unsubscribe(sub *subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subscribers[sub]; !ok {
		return
	}
	delete(b.subscribers, sub)
	sub.close()
}

func (s *Subscription) Events() <-chan models.EventEnvelope {
	return s.sub.ch
}

func (s *Subscription) Close() {
	s.once.Do(func() {
		close(s.done)
		s.bus.unsubscribe(s.sub)
	})
}

func (s *subscriber) deliver(event models.EventEnvelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	select {
	case s.ch <- event:
		return nil
	default:
		return ErrSubscriberQueueFull
	}
}

func (s *subscriber) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.ch)
}

func (b *Bus) pushToRing(event models.EventEnvelope) {
	if len(b.ring) == 0 {
		return
	}
	if b.ringCount < len(b.ring) {
		idx := (b.ringStart + b.ringCount) % len(b.ring)
		b.ring[idx] = event
		b.ringCount++
		return
	}

	b.ring[b.ringStart] = event
	b.ringStart = (b.ringStart + 1) % len(b.ring)
}

func (b *Bus) snapshotSince(lastEventID string) []models.EventEnvelope {
	if b.ringCount == 0 {
		return nil
	}
	out := make([]models.EventEnvelope, 0, b.ringCount)
	startCollect := lastEventID == ""
	for i := 0; i < b.ringCount; i++ {
		idx := (b.ringStart + i) % len(b.ring)
		event := b.ring[idx]
		if !startCollect {
			if event.ID == lastEventID {
				startCollect = true
			}
			continue
		}
		out = append(out, event)
	}
	return out
}
