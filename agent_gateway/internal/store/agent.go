package store

import (
	"context"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

type Agent struct {
	db *gorm.DB
}

func NewAgent(db *gorm.DB) *Agent {
	return &Agent{db: db}
}

func (s *Agent) Create(ctx context.Context, item models.Agent) (models.Agent, error) {
	err := s.db.WithContext(ctx).Create(&item).Error
	if err != nil {
		return models.Agent{}, err
	}
	return item, nil
}

func (s *Agent) Update(ctx context.Context, item models.Agent) (models.Agent, error) {
	err := s.db.WithContext(ctx).Save(&item).Error
	if err != nil {
		return models.Agent{}, err
	}
	return item, nil
}

func (s *Agent) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Agent{}, id).Error
}

func (s *Agent) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.Agent], error) {
	var items []models.Agent
	count := int64(0)
	if err := s.db.WithContext(ctx).Model(&models.Agent{}).Count(&count).Error; err != nil {
		return models.PageResult[models.Agent]{}, err
	}
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return models.PageResult[models.Agent]{}, err
	}
	return page(items, query), nil
}

func (s *Agent) List(ctx context.Context) ([]models.Agent, error) {
	var items []models.Agent
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Agent) Get(ctx context.Context, id string) (models.Agent, bool, error) {
	items, err := s.List(ctx)
	if err != nil {
		return models.Agent{}, false, err
	}
	idx := indexAgent(items, id)
	if idx < 0 {
		return models.Agent{}, false, nil
	}
	return items[idx], true, nil
}

func (s *Agent) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	item, ok, err := s.Get(ctx, id)
	if err != nil {
		return models.ResourceStatus{}, err
	}
	if !ok {
		return models.ResourceStatus{ID: strings.TrimSpace(id), Exists: false}, nil
	}
	return enabledStatus(item.ID, item.Enabled), nil
}

func indexAgent(items []models.Agent, id string) int {
	id = strings.TrimSpace(id)
	for i, item := range items {
		if item.ID == id {
			return i
		}
	}
	return -1
}
