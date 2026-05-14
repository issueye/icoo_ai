package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services/admin"
)

type AgentRoleController struct {
	*CRUDController[models.AgentRole]
}

func NewAgentRoleController(service *admin.AgentRoleService) *AgentRoleController {
	return &AgentRoleController{CRUDController: NewCRUDController[models.AgentRole](service)}
}

func (ctl *AgentRoleController) Register(router gin.IRouter) {
	ctl.RegisterRoutes(router.Group("/agent-roles"))
}
