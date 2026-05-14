package admin

import (
	"context"
	"strconv"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/repositories"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/scheduler"
)

type ScheduleTaskService struct {
	*Service[models.ScheduleTask]
	repo *repositories.ScheduleTaskRepository
}

func NewScheduleTaskService(repo *repositories.ScheduleTaskRepository) *ScheduleTaskService {
	return &ScheduleTaskService{Service: NewService[models.ScheduleTask](repo, normalizeScheduleTask), repo: repo}
}

func normalizeScheduleTask(item *models.ScheduleTask) {
	if item.Type == "" {
		item.Type = "cron"
	}
}

func (s *ScheduleTaskService) ListEnabled(ctx context.Context) ([]scheduler.Task, error) {
	items, err := s.repo.ListEnabledTasks(ctx)
	if err != nil {
		return nil, err
	}
	return scheduleModelsToRuntime(items), nil
}

func (s *ScheduleTaskService) Due(ctx context.Context, now time.Time) ([]scheduler.Task, error) {
	items, err := s.repo.DueTasks(ctx, now)
	if err != nil {
		return nil, err
	}
	return scheduleModelsToRuntime(items), nil
}

func (s *ScheduleTaskService) TryLock(ctx context.Context, taskID string, now time.Time) (scheduler.Task, bool, error) {
	item, ok, err := s.repo.TryLockTask(ctx, taskID, now)
	if err != nil || !ok {
		return scheduler.Task{}, ok, err
	}
	return scheduleModelToRuntime(item), true, nil
}

func (s *ScheduleTaskService) Complete(ctx context.Context, completion scheduler.Completion) error {
	return s.repo.CompleteTask(ctx, completion)
}

func (s *ScheduleTaskService) UpdateNextRun(ctx context.Context, taskID string, nextRunAt *time.Time) error {
	return s.repo.UpdateNextRun(ctx, taskID, nextRunAt)
}

func scheduleModelsToRuntime(items []models.ScheduleTask) []scheduler.Task {
	out := make([]scheduler.Task, 0, len(items))
	for _, item := range items {
		out = append(out, scheduleModelToRuntime(item))
	}
	return out
}

func scheduleModelToRuntime(item models.ScheduleTask) scheduler.Task {
	return scheduler.Task{
		ID:      item.ID,
		Name:    item.Name,
		Enabled: item.Enabled,
		Schedule: scheduler.Schedule{
			Kind:     scheduler.ScheduleKind(item.Type),
			At:       millisToTime(item.NextRunAt),
			Every:    parseDuration(item.Spec),
			CronExpr: item.Cron,
			Timezone: item.Timezone,
		},
		Payload: scheduler.Payload{
			AgentID: item.AgentID,
			Prompt:  item.Content,
		},
		NextRunAt:  millisToTime(item.NextRunAt),
		LastRunAt:  millisToTime(item.LastRunAt),
		LastStatus: item.LastStatus,
		LastError:  item.LastError,
	}
}

func millisToTime(value *int64) *time.Time {
	if value == nil {
		return nil
	}
	t := time.UnixMilli(*value)
	return &t
}

func parseDuration(raw string) time.Duration {
	if raw == "" {
		return 0
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	if ms, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Duration(ms) * time.Millisecond
	}
	return 0
}
