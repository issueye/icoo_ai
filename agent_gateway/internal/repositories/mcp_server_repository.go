package repositories

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

type MCPServerRepository struct {
	*Repository[models.MCPServer]
}

func NewMCPServerRepository(db *gorm.DB) *MCPServerRepository {
	return &MCPServerRepository{Repository: NewRepository[models.MCPServer](db)}
}

func (r *MCPServerRepository) UpdateRuntimeState(ctx context.Context, item models.MCPServer) error {
	return r.db.WithContext(ctx).Model(&models.MCPServer{}).
		Where("id = ?", item.ID).
		Updates(map[string]any{
			"tools_json": item.ToolsJSON,
			"status":     item.Status,
			"last_error": item.LastError,
		}).Error
}
