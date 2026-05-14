package repositories

import (
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

type AgentRoleRepository struct {
	*Repository[models.AgentRole]
}

func NewAgentRoleRepository(db *gorm.DB) *AgentRoleRepository {
	return &AgentRoleRepository{Repository: NewRepository[models.AgentRole](db, WithDefaultSort[models.AgentRole]("position asc, created_at desc"))}
}
