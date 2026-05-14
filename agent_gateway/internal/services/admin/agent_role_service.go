package admin

import (
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/repositories"
)

type AgentRoleService struct {
	*Service[models.AgentRole]
}

func NewAgentRoleService(repo *repositories.AgentRoleRepository) *AgentRoleService {
	return &AgentRoleService{Service: NewService[models.AgentRole](repo, nil)}
}
