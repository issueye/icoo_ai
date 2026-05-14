package repositories

import (
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"gorm.io/gorm"
)

type AgentRepository struct {
	*Repository[models.Agent]
}

func NewAgentRepository(db *gorm.DB) *AgentRepository {
	return &AgentRepository{Repository: NewRepository[models.Agent](db, WithDefaultSort[models.Agent]("position asc, created_at desc"))}
}
