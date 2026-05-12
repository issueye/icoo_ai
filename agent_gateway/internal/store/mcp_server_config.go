package store

import (
	"context"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type MCPServerConfigStore interface {
	CreateMCPServerConfig(ctx context.Context, item models.MCPServerConfig) (models.MCPServerConfig, error)
	UpdateMCPServerConfig(ctx context.Context, item models.MCPServerConfig) (models.MCPServerConfig, error)
	DeleteMCPServerConfig(ctx context.Context, id string) error
	PageMCPServerConfigs(ctx context.Context, query models.PageQuery) (models.PageResult[models.MCPServerConfig], error)
	ListMCPServerConfigs(ctx context.Context) ([]models.MCPServerConfig, error)
	GetMCPServerConfigByID(ctx context.Context, id string) (models.MCPServerConfig, bool, error)
	StatusMCPServerConfig(ctx context.Context, id string) (models.ResourceStatus, error)
}

func (s *ManagementConfigStore) CreateMCPServerConfig(ctx context.Context, item models.MCPServerConfig) (models.MCPServerConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return models.MCPServerConfig{}, err
	}
	item.ID = ensureID(item.ID)
	if indexMCPServerConfig(settings.MCPServers, item.ID) >= 0 {
		return models.MCPServerConfig{}, ErrDuplicateID
	}
	settings.MCPServers = append(settings.MCPServers, item)
	return item, s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) UpdateMCPServerConfig(ctx context.Context, item models.MCPServerConfig) (models.MCPServerConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return models.MCPServerConfig{}, err
	}
	idx := indexMCPServerConfig(settings.MCPServers, item.ID)
	if idx < 0 {
		return models.MCPServerConfig{}, ErrNotFound
	}
	settings.MCPServers[idx] = item
	return item, s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) DeleteMCPServerConfig(ctx context.Context, id string) error {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	idx := indexMCPServerConfig(settings.MCPServers, id)
	if idx < 0 {
		return ErrNotFound
	}
	settings.MCPServers = append(settings.MCPServers[:idx], settings.MCPServers[idx+1:]...)
	return s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) PageMCPServerConfigs(ctx context.Context, query models.PageQuery) (models.PageResult[models.MCPServerConfig], error) {
	items, err := s.ListMCPServerConfigs(ctx)
	if err != nil {
		return models.PageResult[models.MCPServerConfig]{}, err
	}
	return page(items, query), nil
}

func (s *ManagementConfigStore) ListMCPServerConfigs(ctx context.Context) ([]models.MCPServerConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return nil, err
	}
	return append([]models.MCPServerConfig(nil), settings.MCPServers...), nil
}

func (s *ManagementConfigStore) GetMCPServerConfigByID(ctx context.Context, id string) (models.MCPServerConfig, bool, error) {
	items, err := s.ListMCPServerConfigs(ctx)
	if err != nil {
		return models.MCPServerConfig{}, false, err
	}
	idx := indexMCPServerConfig(items, id)
	if idx < 0 {
		return models.MCPServerConfig{}, false, nil
	}
	return items[idx], true, nil
}

func (s *ManagementConfigStore) StatusMCPServerConfig(ctx context.Context, id string) (models.ResourceStatus, error) {
	item, ok, err := s.GetMCPServerConfigByID(ctx, id)
	if err != nil {
		return models.ResourceStatus{}, err
	}
	if !ok {
		return models.ResourceStatus{ID: strings.TrimSpace(id), Exists: false}, nil
	}
	return enabledStatus(item.ID, item.Enabled), nil
}

func indexMCPServerConfig(items []models.MCPServerConfig, id string) int {
	id = strings.TrimSpace(id)
	for i, item := range items {
		if item.ID == id {
			return i
		}
	}
	return -1
}
