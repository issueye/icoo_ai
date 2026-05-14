package controllers

import (
	"errors"
	"io"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/gin-gonic/gin"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	runtimeacp "github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/acp"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services/admin"
)

type AgentController struct {
	*CRUDController[models.Agent]
	service *admin.AgentService
	runtime *runtimeacp.Manager
}

func NewAgentController(service *admin.AgentService, runtime ...*runtimeacp.Manager) *AgentController {
	var manager *runtimeacp.Manager
	if len(runtime) > 0 {
		manager = runtime[0]
	}
	return &AgentController{CRUDController: NewCRUDController[models.Agent](service), service: service, runtime: manager}
}

func (ctl *AgentController) Register(router gin.IRouter) {
	group := router.Group("/agents")
	if ctl.runtime != nil {
		group.GET("/runtime-status", ctl.RuntimeStatusAll)
		group.POST("/sync-runtime", ctl.SyncRuntime)
		group.GET("/:id/runtime-status", ctl.RuntimeStatus)
		group.POST("/:id/start", ctl.StartAgent)
		group.POST("/:id/stop", ctl.StopAgent)
		group.POST("/:id/sessions", ctl.NewSession)
		group.POST("/:id/sessions/:sessionId/prompts", ctl.PromptText)
		group.POST("/:id/sessions/:sessionId/cancel", ctl.CancelSession)
		group.DELETE("/:id/sessions/:sessionId", ctl.CloseSession)
	}
	ctl.RegisterRoutes(group)
}

func (ctl *AgentController) RuntimeStatusAll(c *gin.Context) {
	writeResult(c, 200, ctl.runtime.StatusAll(), nil)
}

func (ctl *AgentController) RuntimeStatus(c *gin.Context) {
	writeResult(c, 200, ctl.runtime.Status(c.Param("id")), nil)
}

func (ctl *AgentController) SyncRuntime(c *gin.Context) {
	agents, err := ctl.service.List(c.Request.Context(), pageQuery(c))
	if err != nil {
		writeResult(c, 200, nil, err)
		return
	}
	err = ctl.runtime.SyncAgents(c.Request.Context(), agents)
	writeResult(c, 200, ctl.runtime.StatusAll(), err)
}

func (ctl *AgentController) StartAgent(c *gin.Context) {
	agent, err := ctl.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeResult(c, 200, nil, err)
		return
	}
	err = ctl.runtime.StartAgent(c.Request.Context(), agent)
	writeResult(c, 200, ctl.runtime.Status(agent.ID), err)
}

func (ctl *AgentController) StopAgent(c *gin.Context) {
	id := c.Param("id")
	err := ctl.runtime.StopAgent(id)
	writeResult(c, 200, ctl.runtime.Status(id), err)
}

func (ctl *AgentController) NewSession(c *gin.Context) {
	var req acpsdk.NewSessionRequest
	if !bindOptionalJSON(c, &req) {
		return
	}
	out, err := ctl.runtime.NewSession(c.Request.Context(), c.Param("id"), req)
	writeResult(c, 200, out, err)
}

func (ctl *AgentController) PromptText(c *gin.Context) {
	var req promptTextRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := ctl.runtime.PromptText(
		c.Request.Context(),
		c.Param("id"),
		acpsdk.SessionId(c.Param("sessionId")),
		req.Text,
	)
	writeResult(c, 200, out, err)
}

func (ctl *AgentController) CancelSession(c *gin.Context) {
	err := ctl.runtime.Cancel(c.Request.Context(), c.Param("id"), acpsdk.SessionId(c.Param("sessionId")))
	writeResult(c, 200, gin.H{"sessionId": c.Param("sessionId")}, err)
}

func (ctl *AgentController) CloseSession(c *gin.Context) {
	out, err := ctl.runtime.CloseSession(c.Request.Context(), c.Param("id"), acpsdk.SessionId(c.Param("sessionId")))
	writeResult(c, 200, out, err)
}

type promptTextRequest struct {
	Text string `json:"text"`
}

func bindOptionalJSON(c *gin.Context, dst any) bool {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return true
	}
	if err := c.ShouldBindJSON(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		writeError(c, 400, "invalid_json", "request body must be valid JSON")
		return false
	}
	return true
}
