package store

import (
	"context"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

type ScheduleTask struct {
	db *gorm.DB
}

func NewScheduleTask(db *gorm.DB) *ScheduleTask {
	return &ScheduleTask{db: db}
}

func (s *ScheduleTask) Create(ctx context.Context, item models.ScheduleTask) (models.ScheduleTask, error) {
	err := s.db.WithContext(ctx).Create(&item).Error
	if err != nil {
		return models.ScheduleTask{}, err
	}
	return item, nil
}

func (s *ScheduleTask) Update(ctx context.Context, item models.ScheduleTask) (models.ScheduleTask, error) {
	err := s.db.WithContext(ctx).Save(&item).Error
	if err != nil {
		return models.ScheduleTask{}, err
	}
	return item, nil
}

func (s *ScheduleTask) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.ScheduleTask{}, id).Error
}

func (s *ScheduleTask) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.ScheduleTask], error) {
	var items []models.ScheduleTask
	count := int64(0)
	if err := s.db.WithContext(ctx).Model(&models.ScheduleTask{}).Count(&count).Error; err != nil {
		return models.PageResult[models.ScheduleTask]{}, err
	}
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return models.PageResult[models.ScheduleTask]{}, err
	}
	return page(items, query), nil
}

func (s *ScheduleTask) List(ctx context.Context) ([]models.ScheduleTask, error) {
	var items []models.ScheduleTask
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *ScheduleTask) Get(ctx context.Context, id string) (models.ScheduleTask, bool, error) {
	items, err := s.List(ctx)
	if err != nil {
		return models.ScheduleTask{}, false, err
	}
	idx := indexScheduleTask(items, id)
	if idx < 0 {
		return models.ScheduleTask{}, false, nil
	}
	return items[idx], true, nil
}

func (s *ScheduleTask) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	item, ok, err := s.Get(ctx, id)
	if err != nil {
		return models.ResourceStatus{}, err
	}
	if !ok {
		return models.ResourceStatus{ID: strings.TrimSpace(id), Exists: false}, nil
	}
	return enabledStatus(item.ID, item.Enabled), nil
}

func indexScheduleTask(items []models.ScheduleTask, id string) int {
	id = strings.TrimSpace(id)
	for i, item := range items {
		if item.ID == id {
			return i
		}
	}
	return -1
}
