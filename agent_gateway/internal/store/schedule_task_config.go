package store

import (
	"context"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type ScheduleTaskConfigStore interface {
	CreateScheduleTaskConfig(ctx context.Context, item models.ScheduleTaskConfig) (models.ScheduleTaskConfig, error)
	UpdateScheduleTaskConfig(ctx context.Context, item models.ScheduleTaskConfig) (models.ScheduleTaskConfig, error)
	DeleteScheduleTaskConfig(ctx context.Context, id string) error
	PageScheduleTaskConfigs(ctx context.Context, query models.PageQuery) (models.PageResult[models.ScheduleTaskConfig], error)
	ListScheduleTaskConfigs(ctx context.Context) ([]models.ScheduleTaskConfig, error)
	GetScheduleTaskConfigByID(ctx context.Context, id string) (models.ScheduleTaskConfig, bool, error)
	StatusScheduleTaskConfig(ctx context.Context, id string) (models.ResourceStatus, error)
}

func (s *ManagementConfigStore) CreateScheduleTaskConfig(ctx context.Context, item models.ScheduleTaskConfig) (models.ScheduleTaskConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return models.ScheduleTaskConfig{}, err
	}
	item.ID = ensureID(item.ID)
	if indexScheduleTaskConfig(settings.ScheduleTasks, item.ID) >= 0 {
		return models.ScheduleTaskConfig{}, ErrDuplicateID
	}
	settings.ScheduleTasks = append(settings.ScheduleTasks, item)
	return item, s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) UpdateScheduleTaskConfig(ctx context.Context, item models.ScheduleTaskConfig) (models.ScheduleTaskConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return models.ScheduleTaskConfig{}, err
	}
	idx := indexScheduleTaskConfig(settings.ScheduleTasks, item.ID)
	if idx < 0 {
		return models.ScheduleTaskConfig{}, ErrNotFound
	}
	settings.ScheduleTasks[idx] = item
	return item, s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) DeleteScheduleTaskConfig(ctx context.Context, id string) error {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	idx := indexScheduleTaskConfig(settings.ScheduleTasks, id)
	if idx < 0 {
		return ErrNotFound
	}
	settings.ScheduleTasks = append(settings.ScheduleTasks[:idx], settings.ScheduleTasks[idx+1:]...)
	return s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) PageScheduleTaskConfigs(ctx context.Context, query models.PageQuery) (models.PageResult[models.ScheduleTaskConfig], error) {
	items, err := s.ListScheduleTaskConfigs(ctx)
	if err != nil {
		return models.PageResult[models.ScheduleTaskConfig]{}, err
	}
	return page(items, query), nil
}

func (s *ManagementConfigStore) ListScheduleTaskConfigs(ctx context.Context) ([]models.ScheduleTaskConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return nil, err
	}
	return append([]models.ScheduleTaskConfig(nil), settings.ScheduleTasks...), nil
}

func (s *ManagementConfigStore) GetScheduleTaskConfigByID(ctx context.Context, id string) (models.ScheduleTaskConfig, bool, error) {
	items, err := s.ListScheduleTaskConfigs(ctx)
	if err != nil {
		return models.ScheduleTaskConfig{}, false, err
	}
	idx := indexScheduleTaskConfig(items, id)
	if idx < 0 {
		return models.ScheduleTaskConfig{}, false, nil
	}
	return items[idx], true, nil
}

func (s *ManagementConfigStore) StatusScheduleTaskConfig(ctx context.Context, id string) (models.ResourceStatus, error) {
	item, ok, err := s.GetScheduleTaskConfigByID(ctx, id)
	if err != nil {
		return models.ResourceStatus{}, err
	}
	if !ok {
		return models.ResourceStatus{ID: strings.TrimSpace(id), Exists: false}, nil
	}
	return enabledStatus(item.ID, item.Enabled), nil
}

func indexScheduleTaskConfig(items []models.ScheduleTaskConfig, id string) int {
	id = strings.TrimSpace(id)
	for i, item := range items {
		if item.ID == id {
			return i
		}
	}
	return -1
}
