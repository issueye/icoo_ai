package scheduler

import (
	"context"
	"log"
	"sync"
	"time"
)

type Store interface {
	ListEnabled(ctx context.Context) ([]Task, error)
	Due(ctx context.Context, now time.Time) ([]Task, error)
	TryLock(ctx context.Context, taskID string, now time.Time) (Task, bool, error)
	Complete(ctx context.Context, completion Completion) error
	UpdateNextRun(ctx context.Context, taskID string, nextRunAt *time.Time) error
}

type Scheduler struct {
	store  Store
	runner *Runner
	logger *log.Logger

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	wake    chan struct{}
}

func New(store Store, runner *Runner, logger *log.Logger) *Scheduler {
	if runner == nil {
		runner = NewRunner(nil)
	}
	return &Scheduler{
		store:  store,
		runner: runner,
		logger: logger,
		wake:   make(chan struct{}, 1),
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	if err := s.recover(runCtx); err != nil {
		s.running = false
		s.cancel = nil
		cancel()
		return err
	}
	go s.loop(runCtx)
	return nil
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.cancel()
	s.cancel = nil
	s.running = false
}

func (s *Scheduler) Wake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Scheduler) recover(ctx context.Context) error {
	if s.store == nil {
		return nil
	}
	tasks, err := s.store.ListEnabled(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, task := range tasks {
		if task.NextRunAt != nil && task.NextRunAt.After(now) {
			continue
		}
		next, err := NextRun(task.Schedule, now)
		if err != nil {
			s.logf("failed to compute next run for task %s: %v", task.ID, err)
			continue
		}
		if err := s.store.UpdateNextRun(ctx, task.ID, next); err != nil {
			return err
		}
	}
	return nil
}

func (s *Scheduler) loop(ctx context.Context) {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for {
		delay := s.delayUntilNext(ctx)
		timer.Reset(delay)
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
			s.CheckDue(ctx, time.Now())
		}
	}
}

func (s *Scheduler) delayUntilNext(ctx context.Context) time.Duration {
	if s.store == nil {
		return time.Hour
	}
	tasks, err := s.store.ListEnabled(ctx)
	if err != nil {
		s.logf("failed to list enabled tasks: %v", err)
		return time.Minute
	}
	var next *time.Time
	for _, task := range tasks {
		if task.NextRunAt == nil {
			continue
		}
		if next == nil || task.NextRunAt.Before(*next) {
			copied := *task.NextRunAt
			next = &copied
		}
	}
	if next == nil {
		return time.Hour
	}
	delay := time.Until(*next)
	if delay < 0 {
		return 0
	}
	return delay
}

func (s *Scheduler) CheckDue(ctx context.Context, now time.Time) {
	if s.store == nil {
		return
	}
	due, err := s.store.Due(ctx, now)
	if err != nil {
		s.logf("failed to query due tasks: %v", err)
		return
	}
	for _, candidate := range due {
		task, locked, err := s.store.TryLock(ctx, candidate.ID, now)
		if err != nil {
			s.logf("failed to lock task %s: %v", candidate.ID, err)
			continue
		}
		if !locked {
			continue
		}
		go s.execute(ctx, task, now)
	}
}

func (s *Scheduler) execute(ctx context.Context, task Task, now time.Time) {
	completion := s.runner.Run(ctx, task, now)
	if err := s.store.Complete(ctx, completion); err != nil {
		s.logf("failed to complete task %s: %v", task.ID, err)
	}
	s.Wake()
}

func (s *Scheduler) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}
