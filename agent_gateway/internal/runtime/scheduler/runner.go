package scheduler

import (
	"context"
	"errors"
	"time"
)

type ScheduleKind string

const (
	ScheduleAt    ScheduleKind = "at"
	ScheduleEvery ScheduleKind = "every"
	ScheduleCron  ScheduleKind = "cron"
)

type Schedule struct {
	Kind     ScheduleKind
	At       *time.Time
	Every    time.Duration
	CronExpr string
	Timezone string
}

type Payload struct {
	AgentID string
	Prompt  string
	Data    map[string]any
}

type Task struct {
	ID             string
	Name           string
	Enabled        bool
	Schedule       Schedule
	Payload        Payload
	NextRunAt      *time.Time
	LastRunAt      *time.Time
	LastStatus     string
	LastError      string
	DeleteAfterRun bool
}

type Completion struct {
	TaskID     string
	StartedAt  time.Time
	FinishedAt time.Time
	Status     string
	Error      string
	NextRunAt  *time.Time
	Disable    bool
	Delete     bool
}

type AgentRunner interface {
	RunAgentPrompt(ctx context.Context, payload Payload) error
}

type Runner struct {
	agent AgentRunner
}

func NewRunner(agent AgentRunner) *Runner {
	return &Runner{agent: agent}
}

func (r *Runner) Run(ctx context.Context, task Task, now time.Time) Completion {
	started := now
	if started.IsZero() {
		started = time.Now()
	}

	status := "ok"
	errText := ""
	if r.agent != nil {
		if err := r.agent.RunAgentPrompt(ctx, task.Payload); err != nil {
			status = "error"
			errText = err.Error()
		}
	} else if task.Payload.Prompt == "" {
		status = "error"
		errText = ErrNoRunner.Error()
	}

	completion := Completion{
		TaskID:     task.ID,
		StartedAt:  started,
		FinishedAt: time.Now(),
		Status:     status,
		Error:      errText,
	}

	if task.Schedule.Kind == ScheduleAt {
		if task.DeleteAfterRun {
			completion.Delete = true
		} else {
			completion.Disable = true
		}
		return completion
	}

	next, err := NextRun(task.Schedule, completion.FinishedAt)
	if err != nil {
		completion.Status = "error"
		if completion.Error == "" {
			completion.Error = err.Error()
		}
		return completion
	}
	completion.NextRunAt = next
	return completion
}

var ErrNoRunner = errors.New("scheduler runner has no agent runner")
