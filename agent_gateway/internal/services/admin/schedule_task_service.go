package admin

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/repositories"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/scheduler"
)

type ScheduleTaskService struct {
	*Service[models.ScheduleTask]
	repo     *repositories.ScheduleTaskRepository
	onChange func()
}

func NewScheduleTaskService(repo *repositories.ScheduleTaskRepository) *ScheduleTaskService {
	return &ScheduleTaskService{Service: NewService[models.ScheduleTask](repo, normalizeScheduleTask), repo: repo}
}

func (s *ScheduleTaskService) SetOnChange(fn func()) {
	s.onChange = fn
}

func normalizeScheduleTask(item *models.ScheduleTask) {
	if item.Type == "" {
		item.Type = "cron"
	}
}

func (s *ScheduleTaskService) Create(ctx context.Context, item models.ScheduleTask) (models.ScheduleTask, error) {
	normalizeScheduleTask(&item)
	s.setInitialNextRun(&item, time.Now())
	out, err := s.repo.Create(ctx, item)
	if err != nil {
		return out, err
	}
	s.notifyChanged()
	return out, nil
}

func (s *ScheduleTaskService) Update(ctx context.Context, id string, item models.ScheduleTask) (models.ScheduleTask, error) {
	normalizeScheduleTask(&item)
	s.setInitialNextRun(&item, time.Now())
	out, err := s.Service.Update(ctx, id, item)
	if err != nil {
		return out, err
	}
	s.notifyChanged()
	return out, nil
}

func (s *ScheduleTaskService) Delete(ctx context.Context, id string) error {
	if err := s.Service.Delete(ctx, id); err != nil {
		return err
	}
	s.notifyChanged()
	return nil
}

func (s *ScheduleTaskService) SetStatus(ctx context.Context, id string, enabled bool) (models.ResourceStatus, error) {
	out, err := s.Service.SetStatus(ctx, id, enabled)
	if err != nil {
		return out, err
	}
	if enabled {
		item, getErr := s.GetByID(ctx, id)
		if getErr == nil && item.NextRunAt == nil {
			s.setInitialNextRun(&item, time.Now())
			_ = s.repo.UpdateNextRun(ctx, id, millisToTime(item.NextRunAt))
		}
	}
	s.notifyChanged()
	return out, nil
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

func (s *ScheduleTaskService) setInitialNextRun(item *models.ScheduleTask, now time.Time) {
	if item == nil || !item.Enabled || item.NextRunAt != nil {
		return
	}
	next, err := scheduler.NextRun(scheduleFromModel(*item), now)
	if err != nil || next == nil {
		return
	}
	ms := next.UnixMilli()
	item.NextRunAt = &ms
}

func (s *ScheduleTaskService) notifyChanged() {
	if s.onChange != nil {
		s.onChange()
	}
}

func scheduleModelsToRuntime(items []models.ScheduleTask) []scheduler.Task {
	out := make([]scheduler.Task, 0, len(items))
	for _, item := range items {
		out = append(out, scheduleModelToRuntime(item))
	}
	return out
}

func scheduleModelToRuntime(item models.ScheduleTask) scheduler.Task {
	payload := payloadFromModel(item)
	return scheduler.Task{
		ID:             item.ID,
		Name:           item.Name,
		Enabled:        item.Enabled,
		Schedule:       scheduleFromModel(item),
		Payload:        payload,
		NextRunAt:      millisToTime(item.NextRunAt),
		LastRunAt:      millisToTime(item.LastRunAt),
		LastStatus:     item.LastStatus,
		LastError:      item.LastError,
		DeleteAfterRun: item.DeleteAfterRun,
	}
}

func scheduleFromModel(item models.ScheduleTask) scheduler.Schedule {
	return scheduler.Schedule{
		Kind:     scheduler.ScheduleKind(item.Type),
		At:       millisToTime(item.NextRunAt),
		Every:    parseDuration(item.Spec),
		CronExpr: item.Cron,
		Timezone: item.Timezone,
	}
}

func payloadFromModel(item models.ScheduleTask) scheduler.Payload {
	payload := scheduler.Payload{
		AgentID: item.AgentID,
		Prompt:  item.Content,
	}
	if item.PayloadJSON == "" {
		return payload
	}
	var raw struct {
		AgentID string         `json:"agentId"`
		Prompt  string         `json:"prompt"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal([]byte(item.PayloadJSON), &raw); err != nil {
		payload.Data = map[string]any{"raw": item.PayloadJSON}
		return payload
	}
	if raw.AgentID != "" {
		payload.AgentID = raw.AgentID
	}
	if raw.Prompt != "" {
		payload.Prompt = raw.Prompt
	}
	payload.Data = raw.Data
	return payload
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
