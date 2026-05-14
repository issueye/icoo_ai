package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services/admin"
)

type SkillController struct {
	*CRUDController[models.Skill]
	service *admin.SkillService
}

func NewSkillController(service *admin.SkillService) *SkillController {
	return &SkillController{CRUDController: NewCRUDController[models.Skill](service), service: service}
}

func (ctl *SkillController) Register(router gin.IRouter) {
	group := router.Group("/skills")
	ctl.RegisterRoutes(group)
	group.POST("/scan", ctl.Scan)
	group.POST("/:id/reload", ctl.Reload)
	group.GET("/:id/documentation", ctl.Documentation)
}

func (ctl *SkillController) Scan(c *gin.Context) {
	out, err := ctl.service.Scan(c.Request.Context())
	writeResult(c, 200, out, err)
}

func (ctl *SkillController) Reload(c *gin.Context) {
	out, err := ctl.service.Reload(c.Request.Context(), c.Param("id"))
	writeResult(c, 200, out, err)
}

func (ctl *SkillController) Documentation(c *gin.Context) {
	out, err := ctl.service.Documentation(c.Request.Context(), c.Param("id"))
	writeResult(c, 200, gin.H{"id": c.Param("id"), "documentation": out}, err)
}
