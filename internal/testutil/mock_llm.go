package testutil

import (
	"context"
	"sync"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/llm"
)

type MockLLMProvider struct {
	name string

	mu      sync.Mutex
	scripts [][]llm.CompletionEvent
	calls   []llm.CompletionRequest

	StreamErr error
	SendDelay time.Duration
}

func NewMockLLMProvider(name string, scripts ...[]llm.CompletionEvent) *MockLLMProvider {
	if name == "" {
		name = "mock"
	}

	p := &MockLLMProvider{name: name}
	for _, script := range scripts {
		p.Enqueue(script...)
	}
	return p
}

func (p *MockLLMProvider) Name() string {
	return p.name
}

func (p *MockLLMProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.CompletionEvent, error) {
	p.mu.Lock()
	p.calls = append(p.calls, req)
	if p.StreamErr != nil {
		err := p.StreamErr
		p.mu.Unlock()
		return nil, err
	}

	script := []llm.CompletionEvent{{Type: llm.CompletionEventCompleted}}
	if len(p.scripts) > 0 {
		script = append([]llm.CompletionEvent(nil), p.scripts[0]...)
		p.scripts = p.scripts[1:]
	}
	p.mu.Unlock()

	out := make(chan llm.CompletionEvent)
	go func() {
		defer close(out)
		for _, event := range script {
			if p.SendDelay > 0 {
				timer := time.NewTimer(p.SendDelay)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}

			select {
			case <-ctx.Done():
				return
			case out <- event:
			}
		}
	}()

	return out, nil
}

func (p *MockLLMProvider) Enqueue(events ...llm.CompletionEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.scripts = append(p.scripts, append([]llm.CompletionEvent(nil), events...))
}

func (p *MockLLMProvider) Calls() []llm.CompletionRequest {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]llm.CompletionRequest(nil), p.calls...)
}

func (p *MockLLMProvider) LastCall() (llm.CompletionRequest, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.calls) == 0 {
		return llm.CompletionRequest{}, false
	}
	return p.calls[len(p.calls)-1], true
}
