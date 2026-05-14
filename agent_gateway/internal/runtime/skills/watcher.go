package skills

import (
	"context"
	"sync"
	"time"
)

type Watcher struct {
	scanner  *Scanner
	interval time.Duration
	cancel   context.CancelFunc
	mu       sync.Mutex
}

func NewWatcher(scanner *Scanner, interval time.Duration) *Watcher {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Watcher{scanner: scanner, interval: interval}
}

func (w *Watcher) Start(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil || w.scanner == nil {
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	go w.loop(runCtx)
}

func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel == nil {
		return
	}
	w.cancel()
	w.cancel = nil
}

func (w *Watcher) Trigger(ctx context.Context) (ScanResult, error) {
	if w.scanner == nil {
		return ScanResult{}, nil
	}
	return w.scanner.Scan(ctx)
}

func (w *Watcher) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = w.scanner.Scan(ctx)
		}
	}
}
