package acp

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/google/uuid"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	runtimemcp "github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/mcp"
	runtimeskills "github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/skills"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services/admin"
)

const MethodPrefix = "_icoo.gateway/"

type contextKey string

const agentIDContextKey contextKey = "agent_id"

type Services struct {
	Agents     *admin.AgentService
	AgentRoles *admin.AgentRoleService
	MCPServers *admin.MCPServerService
	Schedules  *admin.ScheduleTaskService
	Skills     *admin.SkillService
	Events     *events.Bus
}

type ExtensionGateway struct {
	services Services
}

func NewExtensionGateway(services Services) *ExtensionGateway {
	return &ExtensionGateway{services: services}
}

func ContextWithAgentID(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, agentIDContextKey, agentID)
}

func AgentIDFromContext(ctx context.Context) string {
	agentID, _ := ctx.Value(agentIDContextKey).(string)
	return strings.TrimSpace(agentID)
}

func (g *ExtensionGateway) HandleExtensionMethod(ctx context.Context, method string, params json.RawMessage) (result any, err error) {
	action, ok := strings.CutPrefix(method, MethodPrefix)
	if !ok {
		return nil, acpsdk.NewMethodNotFound(method)
	}
	defer func() {
		g.audit(ctx, action, err)
	}()
	if err := g.authorize(ctx, action); err != nil {
		return nil, err
	}
	switch action {
	case "agent.create":
		return createResource[models.Agent](ctx, params, g.services.Agents)
	case "agent.update":
		return updateResource[models.Agent](ctx, params, g.services.Agents)
	case "agent.delete":
		return deleteResource(ctx, params, g.services.Agents)
	case "agent.get":
		return getResource[models.Agent](ctx, params, g.services.Agents)
	case "agent.list":
		return pageResource[models.Agent](ctx, params, g.services.Agents)
	case "agent-role.create":
		return createResource[models.AgentRole](ctx, params, g.services.AgentRoles)
	case "agent-role.update":
		return updateResource[models.AgentRole](ctx, params, g.services.AgentRoles)
	case "agent-role.delete":
		return deleteResource(ctx, params, g.services.AgentRoles)
	case "agent-role.get":
		return getResource[models.AgentRole](ctx, params, g.services.AgentRoles)
	case "agent-role.list":
		return pageResource[models.AgentRole](ctx, params, g.services.AgentRoles)
	case "mcp.create":
		return createResource[models.MCPServer](ctx, params, g.services.MCPServers)
	case "mcp.update":
		return updateResource[models.MCPServer](ctx, params, g.services.MCPServers)
	case "mcp.delete":
		return deleteResource(ctx, params, g.services.MCPServers)
	case "mcp.get":
		return getResource[models.MCPServer](ctx, params, g.services.MCPServers)
	case "mcp.list":
		return pageResource[models.MCPServer](ctx, params, g.services.MCPServers)
	case "mcp.refresh":
		var req idRequest
		if err := decodeParams(params, &req); err != nil {
			return nil, err
		}
		return g.services.MCPServers.RefreshTools(ctx, req.ID)
	case "mcp.status":
		var req idRequest
		if err := decodeParams(params, &req); err != nil {
			return nil, err
		}
		return g.services.MCPServers.RuntimeStatus(req.ID), nil
	case "mcp.call":
		var req mcpCallRequest
		if err := decodeParams(params, &req); err != nil {
			return nil, err
		}
		return g.services.MCPServers.CallTool(ctx, req.ID, runtimemcp.ToolCall{Name: req.Tool, Arguments: req.Arguments})
	case "schedule.create":
		return createResource[models.ScheduleTask](ctx, params, g.services.Schedules)
	case "schedule.update":
		return updateResource[models.ScheduleTask](ctx, params, g.services.Schedules)
	case "schedule.delete":
		return deleteResource(ctx, params, g.services.Schedules)
	case "schedule.get":
		return getResource[models.ScheduleTask](ctx, params, g.services.Schedules)
	case "schedule.list":
		return pageResource[models.ScheduleTask](ctx, params, g.services.Schedules)
	case "skill.create":
		return createResource[models.Skill](ctx, params, g.services.Skills)
	case "skill.update":
		return updateResource[models.Skill](ctx, params, g.services.Skills)
	case "skill.delete":
		return deleteResource(ctx, params, g.services.Skills)
	case "skill.get":
		var req idRequest
		if err := decodeParams(params, &req); err != nil {
			return nil, err
		}
		return g.getExposedSkill(ctx, req.ID)
	case "skill.list":
		page, err := pageResource[models.Skill](ctx, params, g.services.Skills)
		if err != nil {
			return page, err
		}
		return g.filterExposedSkills(ctx, page)
	case "skill.scan":
		return g.services.Skills.Scan(ctx)
	case "skill.reload":
		var req idRequest
		if err := decodeParams(params, &req); err != nil {
			return nil, err
		}
		if _, err := g.getExposedSkill(ctx, req.ID); err != nil {
			return nil, err
		}
		return g.services.Skills.Reload(ctx, req.ID)
	case "skill.documentation":
		var req idRequest
		if err := decodeParams(params, &req); err != nil {
			return nil, err
		}
		if _, err := g.getExposedSkill(ctx, req.ID); err != nil {
			return nil, err
		}
		doc, err := g.services.Skills.Documentation(ctx, req.ID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": req.ID, "documentation": doc}, nil
	default:
		return nil, acpsdk.NewMethodNotFound(method)
	}
}

func (g *ExtensionGateway) getExposedSkill(ctx context.Context, id string) (models.Skill, error) {
	skill, err := g.services.Skills.GetByID(ctx, id)
	if err != nil {
		return models.Skill{}, err
	}
	if !skill.Enabled {
		return models.Skill{}, acpsdk.NewAuthRequired(map[string]any{"reason": "skill_disabled", "id": id})
	}
	if !g.skillAllowed(ctx, skill) {
		return models.Skill{}, acpsdk.NewAuthRequired(map[string]any{"reason": "skill_not_allowed", "id": id})
	}
	return skill, nil
}

func (g *ExtensionGateway) filterExposedSkills(ctx context.Context, page models.PageResult[models.Skill]) (models.PageResult[models.Skill], error) {
	items := make([]models.Skill, 0, len(page.Items))
	for _, skill := range page.Items {
		if !skill.Enabled || !g.skillAllowed(ctx, skill) {
			continue
		}
		items = append(items, skill)
	}
	page.Items = items
	page.Total = len(items)
	return page, nil
}

func (g *ExtensionGateway) skillAllowed(ctx context.Context, skill models.Skill) bool {
	permissions, ok, err := g.permissionsForAgent(ctx)
	if err != nil {
		return false
	}
	if !ok {
		return true
	}
	allow := append([]string{}, permissions.Skills...)
	allow = append(allow, permissions.SkillAllow...)
	if len(allow) == 0 {
		return true
	}
	for _, entry := range allow {
		entry = strings.TrimSpace(entry)
		if entry == "*" || entry == skill.ID || entry == skill.Name {
			return true
		}
	}
	return false
}

func (g *ExtensionGateway) audit(ctx context.Context, action string, err error) {
	if g == nil || g.services.Events == nil {
		return
	}
	level := "info"
	status := "ok"
	summary := "ACP extension method called"
	if err != nil {
		level = "error"
		status = "error"
		summary = "ACP extension method failed"
	}
	agentID := AgentIDFromContext(ctx)
	audit := models.AuditEvent{
		BaseModel: models.BaseModel{ID: uuid.NewString()},
		Type:      "acp.extension_method",
		Level:     level,
		AgentID:   agentID,
		Summary:   summary,
		SafeMeta: models.SafeMeta{
			"method": action,
			"status": status,
		},
		CreatedAt: time.Now(),
	}
	if err != nil {
		audit.SafeMeta["error"] = err.Error()
	}
	g.services.Events.Publish(models.EventEnvelope{
		BaseModel: models.BaseModel{ID: uuid.NewString()},
		Type:      "audit.acp_extension",
		AgentID:   agentID,
		Payload:   audit,
		CreatedAt: audit.CreatedAt,
	})
}

type rolePermissions struct {
	Allow      []string `json:"allow"`
	Extensions []string `json:"extensions"`
	Deny       []string `json:"deny"`
	Skills     []string `json:"skills"`
	SkillAllow []string `json:"skillAllowlist"`
}

func (g *ExtensionGateway) authorize(ctx context.Context, action string) error {
	agentID := AgentIDFromContext(ctx)
	if agentID == "" {
		return nil
	}
	if g.services.Agents == nil {
		return acpsdk.NewAuthRequired(map[string]any{"reason": "agent_service_not_configured", "agentId": agentID})
	}
	agent, err := g.services.Agents.GetByID(ctx, agentID)
	if err != nil {
		return acpsdk.NewAuthRequired(map[string]any{"reason": "agent_not_found", "agentId": agentID})
	}
	if strings.TrimSpace(agent.RoleID) == "" {
		return nil
	}
	if g.services.AgentRoles == nil {
		return acpsdk.NewAuthRequired(map[string]any{"reason": "agent_role_service_not_configured", "agentId": agentID, "roleId": agent.RoleID})
	}
	role, err := g.services.AgentRoles.GetByID(ctx, agent.RoleID)
	if err != nil {
		return acpsdk.NewAuthRequired(map[string]any{"reason": "agent_role_not_found", "agentId": agentID, "roleId": agent.RoleID})
	}
	if !role.Enabled {
		return acpsdk.NewAuthRequired(map[string]any{"reason": "agent_role_disabled", "agentId": agentID, "roleId": role.ID, "method": action})
	}

	var permissions rolePermissions
	if raw := strings.TrimSpace(role.PermissionsJSON); raw != "" {
		if err := json.Unmarshal([]byte(raw), &permissions); err != nil {
			return acpsdk.NewInvalidParams(map[string]any{"reason": "invalid_agent_role_permissions", "roleId": role.ID, "error": err.Error()})
		}
	}
	if matchesAny(action, permissions.Deny) {
		return acpsdk.NewAuthRequired(map[string]any{"reason": "extension_denied", "agentId": agentID, "roleId": role.ID, "method": action})
	}
	allow := append([]string{}, permissions.Extensions...)
	allow = append(allow, permissions.Allow...)
	if len(allow) == 0 || matchesAny(action, allow) {
		return nil
	}
	return acpsdk.NewAuthRequired(map[string]any{"reason": "extension_not_allowed", "agentId": agentID, "roleId": role.ID, "method": action})
}

func (g *ExtensionGateway) permissionsForAgent(ctx context.Context) (rolePermissions, bool, error) {
	agentID := AgentIDFromContext(ctx)
	if agentID == "" || g.services.Agents == nil {
		return rolePermissions{}, false, nil
	}
	agent, err := g.services.Agents.GetByID(ctx, agentID)
	if err != nil || strings.TrimSpace(agent.RoleID) == "" || g.services.AgentRoles == nil {
		return rolePermissions{}, false, err
	}
	role, err := g.services.AgentRoles.GetByID(ctx, agent.RoleID)
	if err != nil {
		return rolePermissions{}, false, err
	}
	var permissions rolePermissions
	if raw := strings.TrimSpace(role.PermissionsJSON); raw != "" {
		if err := json.Unmarshal([]byte(raw), &permissions); err != nil {
			return rolePermissions{}, false, err
		}
	}
	return permissions, true, nil
}

func matchesAny(action string, patterns []string) bool {
	action = normalizeExtensionPattern(action)
	for _, pattern := range patterns {
		pattern = normalizeExtensionPattern(pattern)
		if pattern == "" {
			continue
		}
		if pattern == "*" || pattern == action {
			return true
		}
		if strings.HasSuffix(pattern, ".*") {
			prefix := strings.TrimSuffix(pattern, "*")
			if strings.HasPrefix(action, prefix) {
				return true
			}
		}
	}
	return false
}

func normalizeExtensionPattern(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, MethodPrefix)
	return value
}

type crudService[T any] interface {
	Create(ctx context.Context, item T) (T, error)
	Update(ctx context.Context, id string, item T) (T, error)
	Delete(ctx context.Context, id string) error
	GetByID(ctx context.Context, id string) (T, error)
	Page(ctx context.Context, query models.PageQuery) (models.PageResult[T], error)
}

type idRequest struct {
	ID string `json:"id"`
}

type mcpCallRequest struct {
	ID        string         `json:"id"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
}

type updateRequest[T any] struct {
	ID   string `json:"id"`
	Item T      `json:"item"`
}

func createResource[T any](ctx context.Context, params json.RawMessage, service crudService[T]) (T, error) {
	var item T
	if err := decodeParams(params, &item); err != nil {
		var zero T
		return zero, err
	}
	return service.Create(ctx, item)
}

func updateResource[T any](ctx context.Context, params json.RawMessage, service crudService[T]) (T, error) {
	var req updateRequest[T]
	if err := decodeParams(params, &req); err != nil {
		var zero T
		return zero, err
	}
	return service.Update(ctx, req.ID, req.Item)
}

func deleteResource[T any](ctx context.Context, params json.RawMessage, service crudService[T]) (map[string]string, error) {
	var req idRequest
	if err := decodeParams(params, &req); err != nil {
		return nil, err
	}
	if err := service.Delete(ctx, req.ID); err != nil {
		return nil, err
	}
	return map[string]string{"id": req.ID}, nil
}

func getResource[T any](ctx context.Context, params json.RawMessage, service crudService[T]) (T, error) {
	var req idRequest
	if err := decodeParams(params, &req); err != nil {
		var zero T
		return zero, err
	}
	return service.GetByID(ctx, req.ID)
}

func pageResource[T any](ctx context.Context, params json.RawMessage, service crudService[T]) (models.PageResult[T], error) {
	var query models.PageQuery
	if len(params) > 0 && string(params) != "null" {
		if err := decodeParams(params, &query); err != nil {
			return models.PageResult[T]{}, err
		}
	}
	return service.Page(ctx, query)
}

func decodeParams(params json.RawMessage, dst any) error {
	if len(params) == 0 || string(params) == "null" {
		params = []byte("{}")
	}
	if err := json.Unmarshal(params, dst); err != nil {
		return acpsdk.NewInvalidParams(map[string]any{"error": err.Error()})
	}
	return nil
}

var _ acpsdk.ExtensionMethodHandler = (*ExtensionGateway)(nil)

type MCPStatus = runtimemcp.ServerStatus
type SkillScanResult = runtimeskills.ScanResult
