package store

import (
	"context"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

type Channel struct {
	db *gorm.DB
}

func NewChannel(db *gorm.DB) *Channel {
	return &Channel{db: db}
}

func (s *Channel) Create(ctx context.Context, item models.Channel) (models.Channel, error) {
	err := s.db.WithContext(ctx).Create(&item).Error
	if err != nil {
		return models.Channel{}, err
	}
	return item, nil
}

func (s *Channel) Update(ctx context.Context, item models.Channel) (models.Channel, error) {
	err := s.db.WithContext(ctx).Save(&item).Error
	if err != nil {
		return models.Channel{}, err
	}
	return item, nil
}

func (s *Channel) Delete(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Channel{}, id).Error
}

func (s *Channel) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.Channel], error) {
	var items []models.Channel
	count := int64(0)
	if err := s.db.WithContext(ctx).Model(&models.Channel{}).Count(&count).Error; err != nil {
		return models.PageResult[models.Channel]{}, err
	}
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return models.PageResult[models.Channel]{}, err
	}
	return page(items, query), nil
}

func (s *Channel) List(ctx context.Context) ([]models.Channel, error) {
	var items []models.Channel
	if err := s.db.WithContext(ctx).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Channel) Get(ctx context.Context, id string) (models.Channel, bool, error) {
	items, err := s.List(ctx)
	if err != nil {
		return models.Channel{}, false, err
	}
	idx := indexChannel(items, id)
	if idx < 0 {
		return models.Channel{}, false, nil
	}
	return items[idx], true, nil
}

func (s *Channel) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	item, ok, err := s.Get(ctx, id)
	if err != nil {
		return models.ResourceStatus{}, err
	}
	if !ok {
		return models.ResourceStatus{ID: strings.TrimSpace(id), Exists: false}, nil
	}
	return enabledStatus(item.ID, item.Enabled), nil
}

func indexChannel(items []models.Channel, id string) int {
	id = strings.TrimSpace(id)
	for i, item := range items {
		if item.ID == id {
			return i
		}
	}
	return -1
}
