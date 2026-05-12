package services

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type ChannelConfigService interface {
	Create(ctx context.Context, item models.ChannelConfig) (models.ChannelConfig, error)
	Update(ctx context.Context, item models.ChannelConfig) (models.ChannelConfig, error)
	Delete(ctx context.Context, id string) error
	Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.ChannelConfig], error)
	List(ctx context.Context) ([]models.ChannelConfig, error)
	GetByID(ctx context.Context, id string) (models.ChannelConfig, error)
	Status(ctx context.Context, id string) (models.ResourceStatus, error)
}

type ChannelConfigCRUD struct {
	store store.ChannelConfigStore
}

func NewChannelConfigCRUD(st store.ChannelConfigStore) *ChannelConfigCRUD {
	return &ChannelConfigCRUD{store: st}
}

func (s *ChannelConfigCRUD) Create(ctx context.Context, item models.ChannelConfig) (models.ChannelConfig, error) {
	out, err := s.store.CreateChannelConfig(ctx, item)
	return out, mapStoreError(err)
}

func (s *ChannelConfigCRUD) Update(ctx context.Context, item models.ChannelConfig) (models.ChannelConfig, error) {
	out, err := s.store.UpdateChannelConfig(ctx, item)
	return out, mapStoreError(err)
}

func (s *ChannelConfigCRUD) Delete(ctx context.Context, id string) error {
	return mapStoreError(s.store.DeleteChannelConfig(ctx, id))
}

func (s *ChannelConfigCRUD) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.ChannelConfig], error) {
	out, err := s.store.PageChannelConfigs(ctx, query)
	return out, mapStoreError(err)
}

func (s *ChannelConfigCRUD) List(ctx context.Context) ([]models.ChannelConfig, error) {
	out, err := s.store.ListChannelConfigs(ctx)
	return out, mapStoreError(err)
}

func (s *ChannelConfigCRUD) GetByID(ctx context.Context, id string) (models.ChannelConfig, error) {
	out, ok, err := s.store.GetChannelConfigByID(ctx, id)
	if err != nil {
		return models.ChannelConfig{}, mapStoreError(err)
	}
	if !ok {
		return models.ChannelConfig{}, &GatewayError{Code: "channel_config_not_found", Message: "channel config not found"}
	}
	return out, nil
}

func (s *ChannelConfigCRUD) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	out, err := s.store.StatusChannelConfig(ctx, id)
	return out, mapStoreError(err)
}
