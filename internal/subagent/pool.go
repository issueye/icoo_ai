package subagent

import (
	"context"
	"errors"
	"fmt"
)

type PoolOptions struct {
	Runner      Runner
	Concurrency int
	QueueSize   int
}

type Pool struct {
	runner Runner
	sem    chan struct{}
	queue  chan struct{}
}

func NewPool(opts PoolOptions) (*Pool, error) {
	if opts.Runner == nil {
		return nil, errors.New("subagent pool requires runner")
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	if opts.QueueSize < 0 {
		return nil, errors.New("subagent pool queue size must not be negative")
	}
	return &Pool{
		runner: opts.Runner,
		sem:    make(chan struct{}, concurrency),
		queue:  make(chan struct{}, opts.QueueSize),
	}, nil
}

func (p *Pool) Run(ctx context.Context, req Request) (Result, error) {
	if p == nil || p.runner == nil {
		return Result{}, errors.New("subagent pool is not configured")
	}
	queued, err := p.enterQueue(ctx)
	if err != nil {
		return Result{}, err
	}

	select {
	case p.sem <- struct{}{}:
		p.leaveQueue(queued)
		defer func() { <-p.sem }()
		return p.runner.Run(ctx, req)
	case <-ctx.Done():
		p.leaveQueue(queued)
		return Result{}, ctx.Err()
	}
}

func (p *Pool) enterQueue(ctx context.Context) (bool, error) {
	if cap(p.queue) == 0 {
		return false, nil
	}
	select {
	case p.queue <- struct{}{}:
		return true, nil
	default:
		return false, fmt.Errorf("subagent pool queue is full")
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (p *Pool) leaveQueue(queued bool) {
	if queued {
		<-p.queue
	}
}
