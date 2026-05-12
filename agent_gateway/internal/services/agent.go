package services

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type Agent struct {
	store *store.Agent
}

func NewAgent(st *store.Agent) *Agent {
	return &Agent{store: st}
}

func (s *Agent) Create(ctx context.Context, item models.Agent) (models.Agent, error) {
	out, err := s.store.Create(ctx, item)
	return out, mapStoreError(err)
}

func (s *Agent) Update(ctx context.Context, item models.Agent) (models.Agent, error) {
	out, err := s.store.Update(ctx, item)
	return out, mapStoreError(err)
}

func (s *Agent) Delete(ctx context.Context, id string) error {
	return mapStoreError(s.store.Delete(ctx, id))
}

func (s *Agent) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.Agent], error) {
	out, err := s.store.Page(ctx, query)
	return out, mapStoreError(err)
}

func (s *Agent) List(ctx context.Context) ([]models.Agent, error) {
	out, err := s.store.List(ctx)
	return out, mapStoreError(err)
}

func (s *Agent) GetByID(ctx context.Context, id string) (models.Agent, error) {
	out, ok, err := s.store.Get(ctx, id)
	if err != nil {
		return models.Agent{}, mapStoreError(err)
	}
	if !ok {
		return models.Agent{}, &GatewayError{Code: AGENT_NOT_FOUND_CODE, Message: AGENT_NOT_FOUND_MSG}
	}
	return out, nil
}

func (s *Agent) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	out, err := s.store.Status(ctx, id)
	return out, mapStoreError(err)
}
