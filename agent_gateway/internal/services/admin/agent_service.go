package admin

import (
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/repositories"
)

type AgentService struct {
	*Service[models.Agent]
}

func NewAgentService(repo *repositories.AgentRepository) *AgentService {
	return &AgentService{Service: NewService[models.Agent](repo, normalizeAgent)}
}

func normalizeAgent(item *models.Agent) {
	if item.Protocol == "" {
		item.Protocol = models.AgentProtocolACP
	}
}
