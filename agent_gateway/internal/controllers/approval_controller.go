package controllers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	runtimeacp "github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/acp"
)

type ApprovalController struct {
	broker *runtimeacp.ApprovalBroker
}

func NewApprovalController(broker *runtimeacp.ApprovalBroker) *ApprovalController {
	return &ApprovalController{broker: broker}
}

func (ctl *ApprovalController) Register(router gin.IRouter) {
	group := router.Group("/approvals")
	group.GET("", ctl.List)
	group.GET("/:id", ctl.Get)
	group.POST("/:id/decision", ctl.Decide)
	group.PUT("/:id/decision", ctl.Decide)
}

func (ctl *ApprovalController) List(c *gin.Context) {
	writeResult(c, http.StatusOK, ctl.broker.List(), nil)
}

func (ctl *ApprovalController) Get(c *gin.Context) {
	record, ok := ctl.broker.Get(c.Param("id"))
	if !ok {
		writeError(c, http.StatusNotFound, "not_found", "approval not found")
		return
	}
	writeResult(c, http.StatusOK, record, nil)
}

func (ctl *ApprovalController) Decide(c *gin.Context) {
	var req runtimeacp.ApprovalDecision
	if !bindJSON(c, &req) {
		return
	}
	record, err := ctl.broker.Decide(c.Param("id"), req)
	if err != nil {
		switch {
		case errors.Is(err, runtimeacp.ErrApprovalNotFound):
			writeError(c, http.StatusNotFound, "not_found", "approval not found")
		case errors.Is(err, runtimeacp.ErrApprovalClosed):
			writeError(c, http.StatusConflict, "approval_closed", "approval is already decided")
		case errors.Is(err, runtimeacp.ErrInvalidOption):
			writeError(c, http.StatusBadRequest, "invalid_option", "approval optionId is invalid")
		default:
			writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeResult(c, http.StatusOK, record, nil)
}
