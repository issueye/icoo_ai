package acp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/database"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/repositories"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services/admin"
)

func TestExtensionGatewayAgentCRUD(t *testing.T) {
	db, err := database.OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer database.Close(db)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	agentRepo := repositories.NewAgentRepository(db)
	gateway := NewExtensionGateway(Services{
		Agents: admin.NewAgentService(agentRepo),
	})

	raw, _ := json.Marshal(models.Agent{Name: "agent one", Enabled: true})
	createdAny, err := gateway.HandleExtensionMethod(context.Background(), "_icoo.gateway/agent.create", raw)
	if err != nil {
		t.Fatalf("agent.create error = %v", err)
	}
	created := createdAny.(models.Agent)
	if created.ID == "" {
		t.Fatal("created.ID is empty")
	}

	listAny, err := gateway.HandleExtensionMethod(context.Background(), "_icoo.gateway/agent.list", nil)
	if err != nil {
		t.Fatalf("agent.list error = %v", err)
	}
	page := listAny.(models.PageResult[models.Agent])
	if page.Total != 1 {
		t.Fatalf("page.Total = %d, want 1", page.Total)
	}
}

func TestExtensionGatewayChecksAgentRolePermissions(t *testing.T) {
	db, err := database.OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer database.Close(db)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	agentService := admin.NewAgentService(repositories.NewAgentRepository(db))
	roleService := admin.NewAgentRoleService(repositories.NewAgentRoleRepository(db))
	scheduleService := admin.NewScheduleTaskService(repositories.NewScheduleTaskRepository(db))
	role, err := roleService.Create(context.Background(), models.AgentRole{
		Name:            "limited",
		PermissionsJSON: `{"allow":["schedule.*"],"deny":["schedule.delete"]}`,
		Enabled:         true,
	})
	if err != nil {
		t.Fatalf("role create error = %v", err)
	}
	agent, err := agentService.Create(context.Background(), models.Agent{Name: "agent one", RoleID: role.ID})
	if err != nil {
		t.Fatalf("agent create error = %v", err)
	}
	gateway := NewExtensionGateway(Services{
		Agents:     agentService,
		AgentRoles: roleService,
		Schedules:  scheduleService,
	})
	ctx := ContextWithAgentID(context.Background(), agent.ID)

	raw, _ := json.Marshal(models.ScheduleTask{Name: "daily", Enabled: true})
	if _, err := gateway.HandleExtensionMethod(ctx, "_icoo.gateway/schedule.create", raw); err != nil {
		t.Fatalf("schedule.create error = %v", err)
	}
	if _, err := gateway.HandleExtensionMethod(ctx, "_icoo.gateway/schedule.delete", []byte(`{"id":"x"}`)); err == nil {
		t.Fatal("schedule.delete succeeded, want permission error")
	}
	if _, err := gateway.HandleExtensionMethod(ctx, "_icoo.gateway/mcp.list", nil); err == nil {
		t.Fatal("mcp.list succeeded, want permission error")
	}
}

func TestExtensionGatewayFiltersSkillsByEnabledAndRoleAllowlist(t *testing.T) {
	db, err := database.OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer database.Close(db)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	agentService := admin.NewAgentService(repositories.NewAgentRepository(db))
	roleService := admin.NewAgentRoleService(repositories.NewAgentRoleRepository(db))
	skillService := admin.NewSkillService(repositories.NewSkillRepository(db))
	allowed, err := skillService.Create(context.Background(), models.Skill{Name: "allowed", Enabled: true})
	if err != nil {
		t.Fatalf("allowed skill create error = %v", err)
	}
	if _, err := skillService.Create(context.Background(), models.Skill{Name: "blocked", Enabled: true}); err != nil {
		t.Fatalf("blocked skill create error = %v", err)
	}
	disabled, err := skillService.Create(context.Background(), models.Skill{Name: "disabled", Enabled: false})
	if err != nil {
		t.Fatalf("disabled skill create error = %v", err)
	}
	role, err := roleService.Create(context.Background(), models.AgentRole{
		Name:            "skill-limited",
		PermissionsJSON: `{"allow":["skill.*"],"skills":["allowed"]}`,
		Enabled:         true,
	})
	if err != nil {
		t.Fatalf("role create error = %v", err)
	}
	agent, err := agentService.Create(context.Background(), models.Agent{Name: "agent one", RoleID: role.ID})
	if err != nil {
		t.Fatalf("agent create error = %v", err)
	}
	gateway := NewExtensionGateway(Services{
		Agents:     agentService,
		AgentRoles: roleService,
		Skills:     skillService,
	})
	ctx := ContextWithAgentID(context.Background(), agent.ID)

	listAny, err := gateway.HandleExtensionMethod(ctx, "_icoo.gateway/skill.list", nil)
	if err != nil {
		t.Fatalf("skill.list error = %v", err)
	}
	page := listAny.(models.PageResult[models.Skill])
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != allowed.ID {
		t.Fatalf("page = %#v, want only allowed enabled skill", page)
	}
	disabledReq, _ := json.Marshal(idRequest{ID: disabled.ID})
	if _, err := gateway.HandleExtensionMethod(ctx, "_icoo.gateway/skill.get", disabledReq); err == nil {
		t.Fatal("skill.get disabled succeeded, want error")
	}
}

func TestExtensionGatewayPublishesAuditEvent(t *testing.T) {
	db, err := database.OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer database.Close(db)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	bus := events.NewBus(8)
	sub, _ := bus.Subscribe(context.Background(), "")
	defer sub.Close()
	gateway := NewExtensionGateway(Services{
		Agents: admin.NewAgentService(repositories.NewAgentRepository(db)),
		Events: bus,
	})
	if _, err := gateway.HandleExtensionMethod(context.Background(), "_icoo.gateway/agent.list", nil); err != nil {
		t.Fatalf("agent.list error = %v", err)
	}

	select {
	case event := <-sub.Events():
		if event.Type != "audit.acp_extension" {
			t.Fatalf("event.Type = %q, want audit.acp_extension", event.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("audit event was not published")
	}
}
