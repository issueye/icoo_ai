package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	runtimemcp "github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/mcp"
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
	group.POST("/:id/tools/:toolName/call", ctl.CallTool)
	group.GET("/:id/runtime-status", ctl.RuntimeStatus)
}

func (ctl *MCPController) RefreshTools(c *gin.Context) {
	out, err := ctl.service.RefreshTools(c.Request.Context(), c.Param("id"))
	writeResult(c, 200, out, err)
}

func (ctl *MCPController) RuntimeStatus(c *gin.Context) {
	writeResult(c, 200, ctl.service.RuntimeStatus(c.Param("id")), nil)
}

func (ctl *MCPController) CallTool(c *gin.Context) {
	var req toolCallRequest
	if !bindOptionalJSON(c, &req) {
		return
	}
	out, err := ctl.service.CallTool(c.Request.Context(), c.Param("id"), runtimemcp.ToolCall{
		Name:      c.Param("toolName"),
		Arguments: req.Arguments,
	})
	writeResult(c, 200, out, err)
}

type toolCallRequest struct {
	Arguments map[string]any `json:"arguments"`
}
