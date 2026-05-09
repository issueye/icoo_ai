package events

import (
	"context"
	"errors"
	"sync"
)

const defaultBufferSize = 256

var ErrSubscriberQueueFull = errors.New("subscriber queue full")
var defaultBus = NewBus(defaultBufferSize)

type Bus struct {
	mu          sync.RWMutex
	subscribers map[chan Envelope]struct{}
	ring        []Envelope
	ringStart   int
	ringCount   int
}

type Subscription struct {
	ch   chan Envelope
	bus  *Bus
	once sync.Once
}

func NewBus(bufferSize int) *Bus {
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}
	return &Bus{
		subscribers: make(map[chan Envelope]struct{}),
		ring:        make([]Envelope, bufferSize),
	}
}

func DefaultBus() *Bus {
	return defaultBus
}

func (b *Bus) Publish(event Envelope) {
	b.mu.Lock()
	b.pushToRing(event)

	for ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	b.mu.Unlock()
}

// Subscribe returns a subscription and a snapshot of buffered events.
// lastEventID is reserved for replay semantics in later phases.
func (b *Bus) Subscribe(_ context.Context, lastEventID string) (*Subscription, []Envelope) {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan Envelope, 32)
	b.subscribers[ch] = struct{}{}

	buffered := b.snapshotSince(lastEventID)
	return &Subscription{ch: ch, bus: b}, buffered
}

func (b *Bus) unsubscribe(ch chan Envelope) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subscribers[ch]; !ok {
		return
	}
	delete(b.subscribers, ch)
	close(ch)
}

func (s *Subscription) Events() <-chan Envelope {
	return s.ch
}

func (s *Subscription) Close() {
	s.once.Do(func() {
		s.bus.unsubscribe(s.ch)
	})
}

func (b *Bus) pushToRing(event Envelope) {
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

func (b *Bus) snapshotSince(lastEventID string) []Envelope {
	if b.ringCount == 0 {
		return nil
	}
	out := make([]Envelope, 0, b.ringCount)
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
