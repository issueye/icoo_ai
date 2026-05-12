package services

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type Channel struct {
	store *store.Channel
}

func NewChannel(st *store.Channel) *Channel {
	return &Channel{store: st}
}

func (s *Channel) Create(ctx context.Context, item models.Channel) (models.Channel, error) {
	out, err := s.store.Create(ctx, item)
	return out, mapStoreError(err)
}

func (s *Channel) Update(ctx context.Context, item models.Channel) (models.Channel, error) {
	out, err := s.store.Update(ctx, item)
	return out, mapStoreError(err)
}

func (s *Channel) Delete(ctx context.Context, id string) error {
	return mapStoreError(s.store.Delete(ctx, id))
}

func (s *Channel) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.Channel], error) {
	out, err := s.store.Page(ctx, query)
	return out, mapStoreError(err)
}

func (s *Channel) List(ctx context.Context) ([]models.Channel, error) {
	out, err := s.store.List(ctx)
	return out, mapStoreError(err)
}

func (s *Channel) GetByID(ctx context.Context, id string) (models.Channel, error) {
	out, ok, err := s.store.Get(ctx, id)
	if err != nil {
		return models.Channel{}, mapStoreError(err)
	}
	if !ok {
		return models.Channel{}, &GatewayError{Code: CHANNEL_NOT_FOUND_CODE, Message: CHANNEL_NOT_FOUND_MSG}
	}
	return out, nil
}

func (s *Channel) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	out, err := s.store.Status(ctx, id)
	return out, mapStoreError(err)
}
