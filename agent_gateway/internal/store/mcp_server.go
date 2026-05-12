package store

import (
	"context"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

type MCPServer struct {
	db *gorm.DB
}

func NewMCPServer(db *gorm.DB) *MCPServer {
	return &MCPServer{db: db}
}

func (s *MCPServer) Create(ctx context.Context, item models.MCPServer) (models.MCPServer, error) {
	err := s.db.Create(&item).Error
	// 检查是否存在重复ID
	if err != nil {
		return models.MCPServer{}, err
	}
	return item, nil
}

func (s *MCPServer) Update(ctx context.Context, item models.MCPServer) (models.MCPServer, error) {
	err := s.db.Save(&item).Error
	if err != nil {
		return models.MCPServer{}, err
	}
	return item, nil
}

func (s *MCPServer) Delete(ctx context.Context, id string) error {
	err := s.db.Delete(&models.MCPServer{}, id).Error
	if err != nil {
		return err
	}
	return nil
}

func (s *MCPServer) Page(ctx context.Context, query models.PageQuery) (models.PageResult[models.MCPServer], error) {
	var items []models.MCPServer

	count := int64(0)
	err := s.db.Model(&models.MCPServer{}).Count(&count).Error
	if err != nil {
		return models.PageResult[models.MCPServer]{}, err
	}

	err = s.db.Find(&items).Error
	if err != nil {
		return models.PageResult[models.MCPServer]{}, err
	}
	return page(items, query), nil
}

func (s *MCPServer) List(ctx context.Context) ([]models.MCPServer, error) {
	var items []models.MCPServer
	err := s.db.Find(&items).Error
	if err != nil {
		return nil, err
	}

	return items, nil
}

func (s *MCPServer) Get(ctx context.Context, id string) (models.MCPServer, bool, error) {
	items, err := s.List(ctx)
	if err != nil {
		return models.MCPServer{}, false, err
	}
	idx := indexMCPServer(items, id)
	if idx < 0 {
		return models.MCPServer{}, false, nil
	}
	return items[idx], true, nil
}

func (s *MCPServer) Status(ctx context.Context, id string) (models.ResourceStatus, error) {
	item, ok, err := s.Get(ctx, id)
	if err != nil {
		return models.ResourceStatus{}, err
	}
	if !ok {
		return models.ResourceStatus{ID: strings.TrimSpace(id), Exists: false}, nil
	}
	return enabledStatus(item.ID, item.Enabled), nil
}

func indexMCPServer(items []models.MCPServer, id string) int {
	id = strings.TrimSpace(id)
	for i, item := range items {
		if item.ID == id {
			return i
		}
	}
	return -1
}
