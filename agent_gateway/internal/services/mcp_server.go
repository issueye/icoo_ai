package services

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type MCPServer struct {
	store *store.MCPServer
}

func NewMCPServer(st *store.MCPServer) *MCPServer {
	return &MCPServer{store: st}
}

func (s *MCPServer) Create(ctx context.Context, item models.MCPServer) (models.MCPServer, error) {
	out, err := s.store.Create(ctx, item)
	return out, mapStoreError(err)
}

func (s *MCPServer) Update(ctx context.Context, item models.MCPServer) (models.MCPServer, error) {
	out, err := s.store.Update(ctx, item)
	return out, mapStoreError(err)
}

func (s *MCPServer) Delete(ctx context.Context, id string) error {
	return mapStoreError(s.store.Delete(ctx, id))
}

func (s *MCPServer) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.MCPServer], error) {
	out, err := s.store.Page(ctx, query)
	return out, mapStoreError(err)
}

func (s *MCPServer) List(ctx context.Context) ([]models.MCPServer, error) {
	out, err := s.store.List(ctx)
	return out, mapStoreError(err)
}

func (s *MCPServer) GetByID(ctx context.Context, id string) (models.MCPServer, error) {
	out, ok, err := s.store.Get(ctx, id)
	if err != nil {
		return models.MCPServer{}, mapStoreError(err)
	}
	if !ok {
		return models.MCPServer{}, &GatewayError{Code: MCP_NOT_FOUND_CODE, Message: MCP_NOT_FOUND_MSG}
	}
	return out, nil
}

func (s *MCPServer) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	out, err := s.store.Status(ctx, id)
	return out, mapStoreError(err)
}
