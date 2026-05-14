package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services/admin"
)

type ScheduleController struct {
	*CRUDController[models.ScheduleTask]
}

func NewScheduleController(service *admin.ScheduleTaskService) *ScheduleController {
	return &ScheduleController{CRUDController: NewCRUDController[models.ScheduleTask](service)}
}

func (ctl *ScheduleController) Register(router gin.IRouter) {
	ctl.RegisterRoutes(router.Group("/schedule-tasks"))
}
