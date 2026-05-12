package services

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type ScheduleTask struct {
	store *store.ScheduleTask
}

func NewScheduleTask(st *store.ScheduleTask) *ScheduleTask {
	return &ScheduleTask{store: st}
}

func (s *ScheduleTask) Create(ctx context.Context, item models.ScheduleTask) (models.ScheduleTask, error) {
	out, err := s.store.Create(ctx, item)
	return out, mapStoreError(err)
}

func (s *ScheduleTask) Update(ctx context.Context, item models.ScheduleTask) (models.ScheduleTask, error) {
	out, err := s.store.Update(ctx, item)
	return out, mapStoreError(err)
}

func (s *ScheduleTask) Delete(ctx context.Context, id string) error {
	return mapStoreError(s.store.Delete(ctx, id))
}

func (s *ScheduleTask) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.ScheduleTask], error) {
	out, err := s.store.Page(ctx, query)
	return out, mapStoreError(err)
}

func (s *ScheduleTask) List(ctx context.Context) ([]models.ScheduleTask, error) {
	out, err := s.store.List(ctx)
	return out, mapStoreError(err)
}

func (s *ScheduleTask) GetByID(ctx context.Context, id string) (models.ScheduleTask, error) {
	out, ok, err := s.store.Get(ctx, id)
	if err != nil {
		return models.ScheduleTask{}, mapStoreError(err)
	}
	if !ok {
		return models.ScheduleTask{}, &GatewayError{Code: SCHEDULE_TASK_NOT_FOUND_CODE, Message: SCHEDULE_TASK_NOT_FOUND_MSG}
	}
	return out, nil
}

func (s *ScheduleTask) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	out, err := s.store.Status(ctx, id)
	return out, mapStoreError(err)
}
