package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services/admin"
)

type AgentController struct {
	*CRUDController[models.Agent]
}

func NewAgentController(service *admin.AgentService) *AgentController {
	return &AgentController{CRUDController: NewCRUDController[models.Agent](service)}
}

func (ctl *AgentController) Register(router gin.IRouter) {
	ctl.RegisterRoutes(router.Group("/agents"))
}
