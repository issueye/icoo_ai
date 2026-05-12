package store

import (
	"context"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type AgentConfigStore interface {
	CreateAgentConfig(ctx context.Context, item models.AgentConfig) (models.AgentConfig, error)
	UpdateAgentConfig(ctx context.Context, item models.AgentConfig) (models.AgentConfig, error)
	DeleteAgentConfig(ctx context.Context, id string) error
	PageAgentConfigs(ctx context.Context, query models.PageQuery) (models.PageResult[models.AgentConfig], error)
	ListAgentConfigs(ctx context.Context) ([]models.AgentConfig, error)
	GetAgentConfigByID(ctx context.Context, id string) (models.AgentConfig, bool, error)
	StatusAgentConfig(ctx context.Context, id string) (models.ResourceStatus, error)
}

func (s *ManagementConfigStore) CreateAgentConfig(ctx context.Context, item models.AgentConfig) (models.AgentConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return models.AgentConfig{}, err
	}
	item.ID = ensureID(item.ID)
	if indexAgentConfig(settings.Agents, item.ID) >= 0 {
		return models.AgentConfig{}, ErrDuplicateID
	}
	settings.Agents = append(settings.Agents, item)
	return item, s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) UpdateAgentConfig(ctx context.Context, item models.AgentConfig) (models.AgentConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return models.AgentConfig{}, err
	}
	idx := indexAgentConfig(settings.Agents, item.ID)
	if idx < 0 {
		return models.AgentConfig{}, ErrNotFound
	}
	settings.Agents[idx] = item
	return item, s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) DeleteAgentConfig(ctx context.Context, id string) error {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return err
	}
	idx := indexAgentConfig(settings.Agents, id)
	if idx < 0 {
		return ErrNotFound
	}
	settings.Agents = append(settings.Agents[:idx], settings.Agents[idx+1:]...)
	return s.repo.Save(ctx, settings)
}

func (s *ManagementConfigStore) PageAgentConfigs(ctx context.Context, query models.PageQuery) (models.PageResult[models.AgentConfig], error) {
	items, err := s.ListAgentConfigs(ctx)
	if err != nil {
		return models.PageResult[models.AgentConfig]{}, err
	}
	return page(items, query), nil
}

func (s *ManagementConfigStore) ListAgentConfigs(ctx context.Context) ([]models.AgentConfig, error) {
	settings, err := s.repo.Load(ctx)
	if err != nil {
		return nil, err
	}
	return append([]models.AgentConfig(nil), settings.Agents...), nil
}

func (s *ManagementConfigStore) GetAgentConfigByID(ctx context.Context, id string) (models.AgentConfig, bool, error) {
	items, err := s.ListAgentConfigs(ctx)
	if err != nil {
		return models.AgentConfig{}, false, err
	}
	idx := indexAgentConfig(items, id)
	if idx < 0 {
		return models.AgentConfig{}, false, nil
	}
	return items[idx], true, nil
}

func (s *ManagementConfigStore) StatusAgentConfig(ctx context.Context, id string) (models.ResourceStatus, error) {
	item, ok, err := s.GetAgentConfigByID(ctx, id)
	if err != nil {
		return models.ResourceStatus{}, err
	}
	if !ok {
		return models.ResourceStatus{ID: strings.TrimSpace(id), Exists: false}, nil
	}
	return enabledStatus(item.ID, item.Enabled), nil
}

func indexAgentConfig(items []models.AgentConfig, id string) int {
	id = strings.TrimSpace(id)
	for i, item := range items {
		if item.ID == id {
			return i
		}
	}
	return -1
}
