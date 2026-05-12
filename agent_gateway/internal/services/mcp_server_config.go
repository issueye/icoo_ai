package services

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type MCPServerConfigService interface {
	Create(ctx context.Context, item models.MCPServerConfig) (models.MCPServerConfig, error)
	Update(ctx context.Context, item models.MCPServerConfig) (models.MCPServerConfig, error)
	Delete(ctx context.Context, id string) error
	Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.MCPServerConfig], error)
	List(ctx context.Context) ([]models.MCPServerConfig, error)
	GetByID(ctx context.Context, id string) (models.MCPServerConfig, error)
	Status(ctx context.Context, id string) (models.ResourceStatus, error)
}

type MCPServerConfigCRUD struct {
	store store.MCPServerConfigStore
}

func NewMCPServerConfigCRUD(st store.MCPServerConfigStore) *MCPServerConfigCRUD {
	return &MCPServerConfigCRUD{store: st}
}

func (s *MCPServerConfigCRUD) Create(ctx context.Context, item models.MCPServerConfig) (models.MCPServerConfig, error) {
	out, err := s.store.CreateMCPServerConfig(ctx, item)
	return out, mapStoreError(err)
}

func (s *MCPServerConfigCRUD) Update(ctx context.Context, item models.MCPServerConfig) (models.MCPServerConfig, error) {
	out, err := s.store.UpdateMCPServerConfig(ctx, item)
	return out, mapStoreError(err)
}

func (s *MCPServerConfigCRUD) Delete(ctx context.Context, id string) error {
	return mapStoreError(s.store.DeleteMCPServerConfig(ctx, id))
}

func (s *MCPServerConfigCRUD) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.MCPServerConfig], error) {
	out, err := s.store.PageMCPServerConfigs(ctx, query)
	return out, mapStoreError(err)
}

func (s *MCPServerConfigCRUD) List(ctx context.Context) ([]models.MCPServerConfig, error) {
	out, err := s.store.ListMCPServerConfigs(ctx)
	return out, mapStoreError(err)
}

func (s *MCPServerConfigCRUD) GetByID(ctx context.Context, id string) (models.MCPServerConfig, error) {
	out, ok, err := s.store.GetMCPServerConfigByID(ctx, id)
	if err != nil {
		return models.MCPServerConfig{}, mapStoreError(err)
	}
	if !ok {
		return models.MCPServerConfig{}, &GatewayError{Code: "mcp_server_config_not_found", Message: "mcp server config not found"}
	}
	return out, nil
}

func (s *MCPServerConfigCRUD) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	out, err := s.store.StatusMCPServerConfig(ctx, id)
	return out, mapStoreError(err)
}
