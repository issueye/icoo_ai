package services

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type ScheduleTaskConfigService interface {
	Create(ctx context.Context, item models.ScheduleTaskConfig) (models.ScheduleTaskConfig, error)
	Update(ctx context.Context, item models.ScheduleTaskConfig) (models.ScheduleTaskConfig, error)
	Delete(ctx context.Context, id string) error
	Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.ScheduleTaskConfig], error)
	List(ctx context.Context) ([]models.ScheduleTaskConfig, error)
	GetByID(ctx context.Context, id string) (models.ScheduleTaskConfig, error)
	Status(ctx context.Context, id string) (models.ResourceStatus, error)
}

type ScheduleTaskConfigCRUD struct {
	store store.ScheduleTaskConfigStore
}

func NewScheduleTaskConfigCRUD(st store.ScheduleTaskConfigStore) *ScheduleTaskConfigCRUD {
	return &ScheduleTaskConfigCRUD{store: st}
}

func (s *ScheduleTaskConfigCRUD) Create(ctx context.Context, item models.ScheduleTaskConfig) (models.ScheduleTaskConfig, error) {
	out, err := s.store.CreateScheduleTaskConfig(ctx, item)
	return out, mapStoreError(err)
}

func (s *ScheduleTaskConfigCRUD) Update(ctx context.Context, item models.ScheduleTaskConfig) (models.ScheduleTaskConfig, error) {
	out, err := s.store.UpdateScheduleTaskConfig(ctx, item)
	return out, mapStoreError(err)
}

func (s *ScheduleTaskConfigCRUD) Delete(ctx context.Context, id string) error {
	return mapStoreError(s.store.DeleteScheduleTaskConfig(ctx, id))
}

func (s *ScheduleTaskConfigCRUD) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.ScheduleTaskConfig], error) {
	out, err := s.store.PageScheduleTaskConfigs(ctx, query)
	return out, mapStoreError(err)
}

func (s *ScheduleTaskConfigCRUD) List(ctx context.Context) ([]models.ScheduleTaskConfig, error) {
	out, err := s.store.ListScheduleTaskConfigs(ctx)
	return out, mapStoreError(err)
}

func (s *ScheduleTaskConfigCRUD) GetByID(ctx context.Context, id string) (models.ScheduleTaskConfig, error) {
	out, ok, err := s.store.GetScheduleTaskConfigByID(ctx, id)
	if err != nil {
		return models.ScheduleTaskConfig{}, mapStoreError(err)
	}
	if !ok {
		return models.ScheduleTaskConfig{}, &GatewayError{Code: "schedule_task_config_not_found", Message: "schedule task config not found"}
	}
	return out, nil
}

func (s *ScheduleTaskConfigCRUD) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	out, err := s.store.StatusScheduleTaskConfig(ctx, id)
	return out, mapStoreError(err)
}
