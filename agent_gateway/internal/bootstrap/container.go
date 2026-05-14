package bootstrap

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/controllers"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/database"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/repositories"
	runtimemcp "github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/mcp"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/scheduler"
	runtimeskills "github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/skills"
	adminservice "github.com/icoo-ai/icoo-ai/agent_gateway/internal/services/admin"
	"gorm.io/gorm"
)

type Options struct {
	Config    config.Config
	Token     string
	StartedAt time.Time
}

type Container struct {
	Config config.Config
	Token  string
	DB     *gorm.DB
	Router *gin.Engine

	Controllers  Controllers
	Repositories Repositories
	Services     Services
	Managers     Managers
}

type Controllers struct {
	Health     *controllers.HealthController
	Agents     *controllers.AgentController
	AgentRoles *controllers.AgentRoleController
	MCPServers *controllers.MCPController
	Schedules  *controllers.ScheduleController
	Skills     *controllers.SkillController
}

type Repositories struct {
	Agents     *repositories.AgentRepository
	AgentRoles *repositories.AgentRoleRepository
	MCPServers *repositories.MCPServerRepository
	Schedules  *repositories.ScheduleTaskRepository
	Skills     *repositories.SkillRepository
}

type Services struct {
	Agents     *adminservice.AgentService
	AgentRoles *adminservice.AgentRoleService
	MCPServers *adminservice.MCPServerService
	Schedules  *adminservice.ScheduleTaskService
	Skills     *adminservice.SkillService
}

type Managers struct {
	MCP       *runtimemcp.Manager
	Scheduler *scheduler.Scheduler
	Skills    *runtimeskills.Scanner
}

func Build(ctx context.Context, opts Options) (*Container, error) {
	_ = ctx

	cfg := opts.Config
	if cfg.Version == "" {
		cfg.Version = config.Version
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	db, err := database.OpenSQLite(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	if err := database.AutoMigrate(db); err != nil {
		_ = database.Close(db)
		return nil, err
	}

	startedAt := opts.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	repos := Repositories{
		Agents:     repositories.NewAgentRepository(db),
		AgentRoles: repositories.NewAgentRoleRepository(db),
		MCPServers: repositories.NewMCPServerRepository(db),
		Schedules:  repositories.NewScheduleTaskRepository(db),
		Skills:     repositories.NewSkillRepository(db),
	}
	managers := Managers{
		MCP: runtimemcp.NewManager(),
	}
	services := Services{
		Agents:     adminservice.NewAgentService(repos.Agents),
		AgentRoles: adminservice.NewAgentRoleService(repos.AgentRoles),
		MCPServers: adminservice.NewMCPServerService(repos.MCPServers, managers.MCP),
		Schedules:  adminservice.NewScheduleTaskService(repos.Schedules),
		Skills:     adminservice.NewSkillService(repos.Skills),
	}
	managers.Scheduler = scheduler.New(services.Schedules, scheduler.NewRunner(nil), nil)
	managers.Skills = runtimeskills.NewScanner(
		runtimeskills.NewLoader(),
		runtimeskills.NewRegistry(),
		services.Skills,
		filepath.Join(cfg.DataDir, "skills"),
	)
	services.Skills.SetScanner(managers.Skills)
	ctrls := Controllers{
		Health:     controllers.NewHealthController(cfg.Version, startedAt),
		Agents:     controllers.NewAgentController(services.Agents),
		AgentRoles: controllers.NewAgentRoleController(services.AgentRoles),
		MCPServers: controllers.NewMCPController(services.MCPServers),
		Schedules:  controllers.NewScheduleController(services.Schedules),
		Skills:     controllers.NewSkillController(services.Skills),
	}
	container := &Container{
		Config:       cfg,
		Token:        opts.Token,
		DB:           db,
		Repositories: repos,
		Services:     services,
		Controllers:  ctrls,
		Managers:     managers,
	}
	container.Router = NewRouter(container)
	return container, nil
}

func (c *Container) Close() error {
	if c == nil {
		return nil
	}
	var err error
	if c.Managers.Scheduler != nil {
		c.Managers.Scheduler.Stop()
	}
	if c.Managers.MCP != nil {
		err = errors.Join(err, c.Managers.MCP.Close())
	}
	err = errors.Join(err, database.Close(c.DB))
	return err
}
