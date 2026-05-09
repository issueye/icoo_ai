package subagent

import (
	"context"
	"sync"
	"testing"
	"time"
)

type blockingRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r blockingRunner) Run(ctx context.Context, req Request) (Result, error) {
	select {
	case r.started <- struct{}{}:
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	select {
	case <-r.release:
		return Result{Content: req.Task}, nil
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
}

func TestPoolLimitsConcurrency(t *testing.T) {
	runner := blockingRunner{started: make(chan struct{}, 2), release: make(chan struct{})}
	pool, err := NewPool(PoolOptions{Runner: runner, Concurrency: 1, QueueSize: 2})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, _ = pool.Run(context.Background(), Request{Task: "task"})
		}()
	}

	<-runner.started
	select {
	case <-runner.started:
		t.Fatal("second subagent started before a worker was released")
	case <-time.After(50 * time.Millisecond):
	}

	close(runner.release)
	wg.Wait()
}

func TestPoolRejectsWhenQueueFull(t *testing.T) {
	runner := blockingRunner{started: make(chan struct{}, 3), release: make(chan struct{})}
	pool, err := NewPool(PoolOptions{Runner: runner, Concurrency: 1, QueueSize: 2})
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	firstErr := make(chan error, 1)
	go func() {
		_, err := pool.Run(context.Background(), Request{Task: "first"})
		firstErr <- err
	}()
	<-runner.started

	secondDone := make(chan error, 1)
	go func() {
		_, err := pool.Run(context.Background(), Request{Task: "second"})
		secondDone <- err
	}()
	time.Sleep(20 * time.Millisecond)

	thirdDone := make(chan error, 1)
	go func() {
		_, err := pool.Run(context.Background(), Request{Task: "third"})
		thirdDone <- err
	}()
	time.Sleep(20 * time.Millisecond)

	_, err = pool.Run(context.Background(), Request{Task: "fourth"})
	if err == nil {
		t.Fatal("Run() error = nil, want queue full")
	}

	close(runner.release)
	if err := <-firstErr; err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if err := <-thirdDone; err != nil {
		t.Fatalf("third Run() error = %v", err)
	}
}
