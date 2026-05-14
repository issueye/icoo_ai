package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services/admin"
)

type MCPController struct {
	*CRUDController[models.MCPServer]
	service *admin.MCPServerService
}

func NewMCPController(service *admin.MCPServerService) *MCPController {
	return &MCPController{CRUDController: NewCRUDController[models.MCPServer](service), service: service}
}

func (ctl *MCPController) Register(router gin.IRouter) {
	group := router.Group("/mcp-servers")
	ctl.RegisterRoutes(group)
	group.POST("/:id/refresh-tools", ctl.RefreshTools)
	group.GET("/:id/runtime-status", ctl.RuntimeStatus)
}

func (ctl *MCPController) RefreshTools(c *gin.Context) {
	out, err := ctl.service.RefreshTools(c.Request.Context(), c.Param("id"))
	writeResult(c, 200, out, err)
}

func (ctl *MCPController) RuntimeStatus(c *gin.Context) {
	writeResult(c, 200, ctl.service.RuntimeStatus(c.Param("id")), nil)
}
