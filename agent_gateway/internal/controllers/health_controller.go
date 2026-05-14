package controllers

import (
	"time"

	"github.com/gin-gonic/gin"
)

type HealthController struct {
	version   string
	startedAt time.Time
}

type HealthResponse struct {
	Status       string    `json:"status"`
	Version      string    `json:"version"`
	Capabilities []string  `json:"capabilities"`
	StartedAt    time.Time `json:"startedAt"`
}

func NewHealthController(version string, startedAt time.Time) *HealthController {
	return &HealthController{version: version, startedAt: startedAt}
}

func (h *HealthController) Register(router gin.IRouter) {
	router.GET("/health", h.Get)
}

func (h *HealthController) Get(c *gin.Context) {
	JSON(c, 200, HealthResponse{
		Status:       "ok",
		Version:      h.version,
		Capabilities: []string{"health", "gin", "gorm", "sqlite-no-cgo"},
		StartedAt:    h.startedAt,
	})
}
