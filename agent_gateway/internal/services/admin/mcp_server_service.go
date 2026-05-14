package admin

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/repositories"
	runtimemcp "github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime/mcp"
)

type MCPServerService struct {
	*Service[models.MCPServer]
	repo    *repositories.MCPServerRepository
	runtime *runtimemcp.Manager
}

func NewMCPServerService(repo *repositories.MCPServerRepository, runtime ...*runtimemcp.Manager) *MCPServerService {
	var manager *runtimemcp.Manager
	if len(runtime) > 0 {
		manager = runtime[0]
	}
	return &MCPServerService{Service: NewService[models.MCPServer](repo, normalizeMCPServer), repo: repo, runtime: manager}
}

func normalizeMCPServer(item *models.MCPServer) {
	if item.Transport == "" {
		item.Transport = "stdio"
	}
}

func (s *MCPServerService) Create(ctx context.Context, item models.MCPServer) (models.MCPServer, error) {
	normalizeMCPServer(&item)
	out, err := s.repo.Create(ctx, item)
	if err != nil {
		return out, err
	}
	s.syncRuntimeAsync(out)
	return out, nil
}

func (s *MCPServerService) Update(ctx context.Context, id string, item models.MCPServer) (models.MCPServer, error) {
	normalizeMCPServer(&item)
	out, err := s.Service.Update(ctx, id, item)
	if err != nil {
		return out, err
	}
	s.syncRuntimeAsync(out)
	return out, nil
}

func (s *MCPServerService) Delete(ctx context.Context, id string) error {
	if err := s.Service.Delete(ctx, id); err != nil {
		return err
	}
	if s.runtime != nil {
		_ = s.runtime.CloseServer(id)
	}
	return nil
}

func (s *MCPServerService) SetStatus(ctx context.Context, id string, enabled bool) (models.ResourceStatus, error) {
	out, err := s.Service.SetStatus(ctx, id, enabled)
	if err != nil {
		return out, err
	}
	if s.runtime == nil {
		return out, nil
	}
	if !enabled {
		_ = s.runtime.CloseServer(id)
		return out, nil
	}
	item, getErr := s.GetByID(ctx, id)
	if getErr == nil {
		s.syncRuntimeAsync(item)
	}
	return out, nil
}

func (s *MCPServerService) RefreshTools(ctx context.Context, id string) (models.MCPServer, error) {
	item, err := s.GetByID(ctx, id)
	if err != nil {
		return models.MCPServer{}, err
	}
	if s.runtime == nil {
		item.Status = string(runtimemcp.StateFailed)
		item.LastError = "MCP runtime is not configured"
		_ = s.repo.UpdateRuntimeState(ctx, item)
		return item, nil
	}

	tools, err := s.runtime.RefreshTools(ctx, mcpConfigFromModel(item))
	if err != nil {
		item.Status = string(runtimemcp.StateFailed)
		item.LastError = err.Error()
		_ = s.repo.UpdateRuntimeState(ctx, item)
		return item, err
	}
	data, marshalErr := json.Marshal(tools)
	if marshalErr != nil {
		item.Status = string(runtimemcp.StateFailed)
		item.LastError = marshalErr.Error()
		_ = s.repo.UpdateRuntimeState(ctx, item)
		return item, marshalErr
	}
	item.ToolsJSON = string(data)
	item.Status = string(runtimemcp.StateConnected)
	if !item.Enabled {
		item.Status = string(runtimemcp.StateDisabled)
	}
	item.LastError = ""
	return item, s.repo.UpdateRuntimeState(ctx, item)
}

func (s *MCPServerService) syncRuntimeAsync(item models.MCPServer) {
	if s.runtime == nil || !item.Enabled || strings.TrimSpace(item.ID) == "" {
		return
	}
	go func(id string) {
		_, _ = s.RefreshTools(context.Background(), id)
	}(item.ID)
}

func (s *MCPServerService) RuntimeStatus(id string) runtimemcp.ServerStatus {
	if s.runtime == nil {
		return runtimemcp.ServerStatus{ID: id, State: runtimemcp.StateDisconnected}
	}
	return s.runtime.Status(id)
}

func mcpConfigFromModel(item models.MCPServer) runtimemcp.ServerConfig {
	return runtimemcp.ServerConfig{
		ID:      item.ID,
		Name:    item.Name,
		Enabled: item.Enabled,
		Type:    runtimemcp.TransportType(item.Transport),
		Command: item.Command,
		Args:    decodeStringSlice(item.ArgsJSON),
		URL:     item.URL,
		CWD:     item.Cwd,
		Env:     decodeStringMap(item.EnvJSON),
		EnvFile: item.EnvFile,
		Headers: decodeStringMap(item.HeadersJSON),
	}
}

func decodeStringSlice(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func decodeStringMap(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}
