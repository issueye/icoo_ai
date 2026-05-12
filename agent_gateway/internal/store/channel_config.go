package store

import (
	"context"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type ChannelConfigStore interface {
	CreateChannelConfig(ctx context.Context, item models.ChannelConfig) (models.ChannelConfig, error)
	UpdateChannelConfig(ctx context.Context, item models.ChannelConfig) (models.ChannelConfig, error)
	DeleteChannelConfig(ctx context.Context, id string) error
	PageChannelConfigs(ctx context.Context, query models.PageQuery) (models.PageResult[models.ChannelConfig], error)
	ListChannelConfigs(ctx context.Context) ([]models.ChannelConfig, error)
	GetChannelConfigByID(ctx context.Context, id string) (models.ChannelConfig, bool, error)
	StatusChannelConfig(ctx context.Context, id string) (models.ResourceStatus, error)
}

func (s *ManagementConfigStore) CreateChannelConfig(ctx context.Context, item models.ChannelConfig) (models.ChannelConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return models.ChannelConfig{}, err
	}
	item.ID = ensureID(item.ID)
	if indexChannelConfig(settings.Channels, item.ID) >= 0 {
		return models.ChannelConfig{}, ErrDuplicateID
	}
	settings.Channels = append(settings.Channels, item)
	return item, s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) UpdateChannelConfig(ctx context.Context, item models.ChannelConfig) (models.ChannelConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return models.ChannelConfig{}, err
	}
	idx := indexChannelConfig(settings.Channels, item.ID)
	if idx < 0 {
		return models.ChannelConfig{}, ErrNotFound
	}
	settings.Channels[idx] = item
	return item, s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) DeleteChannelConfig(ctx context.Context, id string) error {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	idx := indexChannelConfig(settings.Channels, id)
	if idx < 0 {
		return ErrNotFound
	}
	settings.Channels = append(settings.Channels[:idx], settings.Channels[idx+1:]...)
	return s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) PageChannelConfigs(ctx context.Context, query models.PageQuery) (models.PageResult[models.ChannelConfig], error) {
	items, err := s.ListChannelConfigs(ctx)
	if err != nil {
		return models.PageResult[models.ChannelConfig]{}, err
	}
	return page(items, query), nil
}

func (s *ManagementConfigStore) ListChannelConfigs(ctx context.Context) ([]models.ChannelConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return nil, err
	}
	return append([]models.ChannelConfig(nil), settings.Channels...), nil
}

func (s *ManagementConfigStore) GetChannelConfigByID(ctx context.Context, id string) (models.ChannelConfig, bool, error) {
	items, err := s.ListChannelConfigs(ctx)
	if err != nil {
		return models.ChannelConfig{}, false, err
	}
	idx := indexChannelConfig(items, id)
	if idx < 0 {
		return models.ChannelConfig{}, false, nil
	}
	return items[idx], true, nil
}

func (s *ManagementConfigStore) StatusChannelConfig(ctx context.Context, id string) (models.ResourceStatus, error) {
	item, ok, err := s.GetChannelConfigByID(ctx, id)
	if err != nil {
		return models.ResourceStatus{}, err
	}
	if !ok {
		return models.ResourceStatus{ID: strings.TrimSpace(id), Exists: false}, nil
	}
	return enabledStatus(item.ID, item.Enabled), nil
}

func indexChannelConfig(items []models.ChannelConfig, id string) int {
	id = strings.TrimSpace(id)
	for i, item := range items {
		if item.ID == id {
			return i
		}
	}
	return -1
}
