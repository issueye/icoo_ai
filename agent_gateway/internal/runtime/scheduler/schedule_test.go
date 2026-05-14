package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNextRunEveryAtAndCron(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 30, 0, time.UTC)

	next, err := NextRun(Schedule{Kind: ScheduleEvery, Every: 5 * time.Minute}, now)
	if err != nil {
		t.Fatalf("NextRun(every) error = %v", err)
	}
	if !next.Equal(now.Add(5 * time.Minute)) {
		t.Fatalf("every next = %s", next)
	}

	at := now.Add(time.Hour)
	next, err = NextRun(Schedule{Kind: ScheduleAt, At: &at}, now)
	if err != nil {
		t.Fatalf("NextRun(at) error = %v", err)
	}
	if !next.Equal(at) {
		t.Fatalf("at next = %s", next)
	}

	next, err = NextRun(Schedule{Kind: ScheduleCron, CronExpr: "*/15 * * * *"}, now)
	if err != nil {
		t.Fatalf("NextRun(cron) error = %v", err)
	}
	want := time.Date(2026, 5, 14, 10, 15, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("cron next = %s, want %s", next, want)
	}
}

func TestRunnerCompletesOneTimeTasks(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	runner := NewRunner(agentRunnerFunc(func(context.Context, Payload) error { return nil }))

	completion := runner.Run(context.Background(), Task{
		ID:             "task-1",
		Schedule:       Schedule{Kind: ScheduleAt, At: &now},
		Payload:        Payload{Prompt: "hello"},
		DeleteAfterRun: true,
	}, now)

	if completion.Status != "ok" || !completion.Delete || completion.Disable {
		t.Fatalf("completion = %#v", completion)
	}
}

func TestSchedulerCheckDueLocksAndCompletesOnce(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	store := newMemoryScheduleStore([]Task{{
		ID:        "task-1",
		Enabled:   true,
		Schedule:  Schedule{Kind: ScheduleEvery, Every: time.Minute},
		Payload:   Payload{Prompt: "run"},
		NextRunAt: &now,
	}})
	runner := NewRunner(agentRunnerFunc(func(context.Context, Payload) error { return nil }))
	s := New(store, runner, nil)

	s.CheckDue(context.Background(), now)
	store.wait(t, "task-1")
	s.CheckDue(context.Background(), now)

	if store.runCount("task-1") != 1 {
		t.Fatalf("run count = %d", store.runCount("task-1"))
	}
	if store.tasks["task-1"].LastStatus != "ok" {
		t.Fatalf("task = %#v", store.tasks["task-1"])
	}
}

func TestSchedulerDisabledTaskIsIgnored(t *testing.T) {
	now := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	store := newMemoryScheduleStore([]Task{{
		ID:        "task-1",
		Enabled:   false,
		Schedule:  Schedule{Kind: ScheduleEvery, Every: time.Minute},
		Payload:   Payload{Prompt: "run"},
		NextRunAt: &now,
	}})
	s := New(store, NewRunner(agentRunnerFunc(func(context.Context, Payload) error { return nil })), nil)

	s.CheckDue(context.Background(), now)

	if store.runCount("task-1") != 0 {
		t.Fatalf("run count = %d", store.runCount("task-1"))
	}
}

func TestRunnerRecordsFailure(t *testing.T) {
	runner := NewRunner(agentRunnerFunc(func(context.Context, Payload) error { return errors.New("boom") }))
	completion := runner.Run(context.Background(), Task{
		ID:       "task-1",
		Schedule: Schedule{Kind: ScheduleEvery, Every: time.Minute},
		Payload:  Payload{Prompt: "run"},
	}, time.Now())
	if completion.Status != "error" || completion.Error != "boom" {
		t.Fatalf("completion = %#v", completion)
	}
}

type agentRunnerFunc func(context.Context, Payload) error

func (f agentRunnerFunc) RunAgentPrompt(ctx context.Context, payload Payload) error {
	return f(ctx, payload)
}

type memoryScheduleStore struct {
	mu        sync.Mutex
	tasks     map[string]Task
	locked    map[string]bool
	completed map[string]int
	done      chan string
}

func newMemoryScheduleStore(tasks []Task) *memoryScheduleStore {
	store := &memoryScheduleStore{
		tasks:     map[string]Task{},
		locked:    map[string]bool{},
		completed: map[string]int{},
		done:      make(chan string, len(tasks)+1),
	}
	for _, task := range tasks {
		store.tasks[task.ID] = task
	}
	return store
}

func (s *memoryScheduleStore) ListEnabled(context.Context) ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Task
	for _, task := range s.tasks {
		if task.Enabled {
			out = append(out, task)
		}
	}
	return out, nil
}

func (s *memoryScheduleStore) Due(_ context.Context, now time.Time) ([]Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Task
	for _, task := range s.tasks {
		if task.Enabled && task.NextRunAt != nil && !task.NextRunAt.After(now) {
			out = append(out, task)
		}
	}
	return out, nil
}

func (s *memoryScheduleStore) TryLock(_ context.Context, taskID string, _ time.Time) (Task, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked[taskID] {
		return Task{}, false, nil
	}
	task, ok := s.tasks[taskID]
	if !ok || !task.Enabled {
		return Task{}, false, nil
	}
	s.locked[taskID] = true
	task.NextRunAt = nil
	s.tasks[taskID] = task
	return task, true, nil
}

func (s *memoryScheduleStore) Complete(_ context.Context, completion Completion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task := s.tasks[completion.TaskID]
	task.LastRunAt = &completion.StartedAt
	task.LastStatus = completion.Status
	task.LastError = completion.Error
	task.NextRunAt = completion.NextRunAt
	if completion.Disable {
		task.Enabled = false
	}
	if completion.Delete {
		delete(s.tasks, completion.TaskID)
	} else {
		s.tasks[completion.TaskID] = task
	}
	delete(s.locked, completion.TaskID)
	s.completed[completion.TaskID]++
	s.done <- completion.TaskID
	return nil
}

func (s *memoryScheduleStore) UpdateNextRun(_ context.Context, taskID string, nextRunAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task := s.tasks[taskID]
	task.NextRunAt = nextRunAt
	s.tasks[taskID] = task
	return nil
}

func (s *memoryScheduleStore) runCount(taskID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.completed[taskID]
}

func (s *memoryScheduleStore) wait(t *testing.T, taskID string) {
	t.Helper()
	select {
	case got := <-s.done:
		if got != taskID {
			t.Fatalf("completed task = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for completion")
	}
}
