package bootstrap

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/controllers"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/security"
)

func NewRouter(container *Container) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	container.Controllers.Health.Register(router)

	v1 := router.Group("/v1")
	v1.Use(bearerAuth(container.Token))
	v1.GET("/status", func(c *gin.Context) {
		controllers.JSON(c, http.StatusOK, gin.H{"status": "ok"})
	})
	container.Controllers.Agents.Register(v1)
	container.Controllers.AgentRoles.Register(v1)
	container.Controllers.MCPServers.Register(v1)
	container.Controllers.Schedules.Register(v1)
	container.Controllers.Skills.Register(v1)

	return router
}

func bearerAuth(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := security.BearerToken(c.GetHeader("Authorization"))
		if err != nil || token != expected {
			controllers.Error(c, http.StatusUnauthorized, "unauthorized", "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}
