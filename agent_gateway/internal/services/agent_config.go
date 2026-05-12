package services

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type AgentConfigService interface {
	Create(ctx context.Context, item models.AgentConfig) (models.AgentConfig, error)
	Update(ctx context.Context, item models.AgentConfig) (models.AgentConfig, error)
	Delete(ctx context.Context, id string) error
	Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.AgentConfig], error)
	List(ctx context.Context) ([]models.AgentConfig, error)
	GetByID(ctx context.Context, id string) (models.AgentConfig, error)
	Status(ctx context.Context, id string) (models.ResourceStatus, error)
}

type AgentConfigCRUD struct {
	store store.AgentConfigStore
}

func NewAgentConfigCRUD(st store.AgentConfigStore) *AgentConfigCRUD {
	return &AgentConfigCRUD{store: st}
}

func (s *AgentConfigCRUD) Create(ctx context.Context, item models.AgentConfig) (models.AgentConfig, error) {
	out, err := s.store.CreateAgentConfig(ctx, item)
	return out, mapStoreError(err)
}

func (s *AgentConfigCRUD) Update(ctx context.Context, item models.AgentConfig) (models.AgentConfig, error) {
	out, err := s.store.UpdateAgentConfig(ctx, item)
	return out, mapStoreError(err)
}

func (s *AgentConfigCRUD) Delete(ctx context.Context, id string) error {
	return mapStoreError(s.store.DeleteAgentConfig(ctx, id))
}

func (s *AgentConfigCRUD) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.AgentConfig], error) {
	out, err := s.store.PageAgentConfigs(ctx, query)
	return out, mapStoreError(err)
}

func (s *AgentConfigCRUD) List(ctx context.Context) ([]models.AgentConfig, error) {
	out, err := s.store.ListAgentConfigs(ctx)
	return out, mapStoreError(err)
}

func (s *AgentConfigCRUD) GetByID(ctx context.Context, id string) (models.AgentConfig, error) {
	out, ok, err := s.store.GetAgentConfigByID(ctx, id)
	if err != nil {
		return models.AgentConfig{}, mapStoreError(err)
	}
	if !ok {
		return models.AgentConfig{}, &GatewayError{Code: "agent_config_not_found", Message: "agent config not found"}
	}
	return out, nil
}

func (s *AgentConfigCRUD) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	out, err := s.store.StatusAgentConfig(ctx, id)
	return out, mapStoreError(err)
}
