package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/services"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
	"gorm.io/gorm"
)

func doRequest(t *testing.T, h http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func decodeResponse[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("Decode() error = %v body=%s", err, rr.Body.String())
	}
	return out
}

func newRouter(t *testing.T) http.Handler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Agent{}, &models.Channel{}, &models.MCPServer{}, &models.ScheduleTask{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	gateway := &testGateway{
		settings: models.ManagementSettings{},
		sessions: map[string]models.Session{},
		agent:    services.NewAgent(store.NewAgent(db)),
		channel:  services.NewChannel(store.NewChannel(db)),
		mcp:      services.NewMCPServer(store.NewMCPServer(db)),
		task:     services.NewScheduleTask(store.NewScheduleTask(db)),
	}
	return NewRouter(gateway)
}

type testGateway struct {
	settings models.ManagementSettings
	sessions map[string]models.Session
	agent    *services.Agent
	channel  *services.Channel
	mcp      *services.MCPServer
	task     *services.ScheduleTask
}

func (g *testGateway) ListAgents(ctx context.Context) ([]models.Agent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []models.Agent{}, nil
}

func (g *testGateway) ListSkills(ctx context.Context) ([]models.Skill, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []models.Skill{}, nil
}

func (g *testGateway) GetManagementSettings(ctx context.Context) (models.ManagementSettings, error) {
	return g.settings, ctx.Err()
}

func (g *testGateway) ReplaceManagementSettings(ctx context.Context, req models.ManagementSettings) (models.ManagementSettings, error) {
	if err := ctx.Err(); err != nil {
		return models.ManagementSettings{}, err
	}
	g.settings = req
	return g.settings, nil
}

func (g *testGateway) CreateSession(ctx context.Context, req models.CreateSessionRequest) (models.Session, error) {
	if err := ctx.Err(); err != nil {
		return models.Session{}, err
	}
	id := uuid.NewString()
	session := models.Session{
		BaseModel:             models.BaseModel{ID: id},
		Title:                 req.Title,
		WorkspaceID:           req.WorkspaceID,
		CWD:                   req.CWD,
		AdditionalDirectories: append([]string(nil), req.AdditionalDirectories...),
		StartupCommand:        req.StartupCommand,
		Mode:                  req.Mode,
		AgentID:               req.AgentID,
		Model:                 req.Model,
		Status:                "active",
	}
	g.sessions[id] = session
	return session, nil
}

func (g *testGateway) ListSessions(ctx context.Context) ([]models.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]models.Session, 0, len(g.sessions))
	for _, item := range g.sessions {
		out = append(out, item)
	}
	return out, nil
}

func (g *testGateway) GetSession(ctx context.Context, sessionID string) (models.Session, error) {
	if err := ctx.Err(); err != nil {
		return models.Session{}, err
	}
	session, ok := g.sessions[sessionID]
	if !ok {
		return models.Session{}, &services.GatewayError{Code: "session_not_found", Message: "session not found"}
	}
	return session, nil
}

func (g *testGateway) DeleteSession(ctx context.Context, sessionID string) (models.Session, error) {
	session, err := g.GetSession(ctx, sessionID)
	if err != nil {
		return models.Session{}, err
	}
	delete(g.sessions, sessionID)
	return session, nil
}

func (g *testGateway) ResumeSession(ctx context.Context, sessionID string, req models.ResumeSessionRequest) (models.Session, error) {
	session, err := g.GetSession(ctx, sessionID)
	if err != nil {
		return models.Session{}, err
	}
	session.CWD = req.CWD
	session.AdditionalDirectories = append([]string(nil), req.AdditionalDirectories...)
	g.sessions[sessionID] = session
	return session, nil
}

func (g *testGateway) UpdateSessionMode(ctx context.Context, sessionID string, req models.SetSessionModeRequest) (models.Session, error) {
	session, err := g.GetSession(ctx, sessionID)
	if err != nil {
		return models.Session{}, err
	}
	session.Mode = req.Mode
	g.sessions[sessionID] = session
	return session, nil
}

func (g *testGateway) UpdateSessionConfig(ctx context.Context, sessionID string, _ models.SetSessionConfigOptionRequest) (models.Session, error) {
	return g.GetSession(ctx, sessionID)
}

func (g *testGateway) ListSessionMessages(ctx context.Context, _ string) ([]models.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []models.Message{}, nil
}

func (g *testGateway) CreateSessionMessage(ctx context.Context, sessionID string, _ models.PromptRequest) (models.PromptResponse, error) {
	if _, err := g.GetSession(ctx, sessionID); err != nil {
		return models.PromptResponse{}, err
	}
	return models.PromptResponse{}, &services.GatewayError{Code: "connector_unavailable", Message: "connector unavailable"}
}

func (g *testGateway) CancelSessionRun(ctx context.Context, sessionID string) (models.Run, error) {
	if _, err := g.GetSession(ctx, sessionID); err != nil {
		return models.Run{}, err
	}
	return models.Run{BaseModel: models.BaseModel{ID: fmt.Sprintf("%s-run", sessionID)}, SessionID: sessionID, Status: "cancelled"}, nil
}

func (g *testGateway) ListRuns(ctx context.Context) ([]models.Run, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []models.Run{}, nil
}

func (g *testGateway) ListApprovals(ctx context.Context) ([]models.Approval, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []models.Approval{}, nil
}

func (g *testGateway) UpdateApprovalDecision(ctx context.Context, _ string, _ models.ApprovalDecisionRequest) (models.Approval, error) {
	if err := ctx.Err(); err != nil {
		return models.Approval{}, err
	}
	return models.Approval{}, &services.GatewayError{Code: "approval_not_found", Message: "approval not found"}
}

func (g *testGateway) Agent() *services.Agent               { return g.agent }
func (g *testGateway) Channel() *services.Channel           { return g.channel }
func (g *testGateway) MCPServer() *services.MCPServer       { return g.mcp }
func (g *testGateway) ScheduleTask() *services.ScheduleTask { return g.task }

func TestManagementSettingsPutThenGet(t *testing.T) {
	router := newRouter(t)
	payload := models.ManagementSettings{
		Agents: []models.Agent{
			{BaseModel: models.BaseModel{ID: "a1"}, Name: "Agent One", Protocol: models.AgentProtocolACP, ModelsJSON: `["gpt-5.4"]`, Enabled: true},
		},
	}

	putResp := doRequest(t, router, http.MethodPut, "/v1/management/settings", payload)
	if putResp.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 body=%s", putResp.Code, putResp.Body.String())
	}

	getResp := doRequest(t, router, http.MethodGet, "/v1/management/settings", nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 body=%s", getResp.Code, getResp.Body.String())
	}
	got := decodeResponse[models.ManagementSettings](t, getResp)
	if len(got.Agents) != 1 || got.Agents[0].ID != "a1" {
		t.Fatalf("unexpected settings agents: %#v", got.Agents)
	}
}

func TestAgentActionCRUD(t *testing.T) {
	router := newRouter(t)
	createResp := doRequest(t, router, http.MethodPost, "/v1/agents/create", models.Agent{
		BaseModel:  models.BaseModel{ID: "agent-crud"},
		Name:       "Agent CRUD",
		Protocol:   models.AgentProtocolACP,
		ModelsJSON: `["gpt-5.4"]`,
		Enabled:    true,
	})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 body=%s", createResp.Code, createResp.Body.String())
	}

	getResp := doRequest(t, router, http.MethodGet, "/v1/agents/getById?id=agent-crud", nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("getById status = %d, want 200 body=%s", getResp.Code, getResp.Body.String())
	}
	got := decodeResponse[models.Agent](t, getResp)
	if got.ID != "agent-crud" || got.Name != "Agent CRUD" {
		t.Fatalf("unexpected agent: %#v", got)
	}

	statusResp := doRequest(t, router, http.MethodGet, "/v1/agents/status?id=agent-crud", nil)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status status = %d, want 200 body=%s", statusResp.Code, statusResp.Body.String())
	}
	status := decodeResponse[models.ResourceStatus](t, statusResp)
	if !status.Exists || status.Enabled == nil || !*status.Enabled {
		t.Fatalf("unexpected status: %#v", status)
	}

	pageResp := doRequest(t, router, http.MethodGet, "/v1/agents/page?page=1&pageSize=10", nil)
	if pageResp.Code != http.StatusOK {
		t.Fatalf("page status = %d, want 200 body=%s", pageResp.Code, pageResp.Body.String())
	}
	page := decodeResponse[models.PageResult[models.Agent]](t, pageResp)
	if page.Total == 0 {
		t.Fatalf("page total = %d, want non-zero", page.Total)
	}
}

func TestSessionMessagesCreateWithoutConnectorReturnsServiceUnavailable(t *testing.T) {
	router := newRouter(t)

	createResp := doRequest(t, router, http.MethodPost, "/v1/sessions", models.CreateSessionRequest{Title: "demo"})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create session status = %d, want 201 body=%s", createResp.Code, createResp.Body.String())
	}
	session := decodeResponse[models.Session](t, createResp)

	promptResp := doRequest(t, router, http.MethodPost, "/v1/sessions/"+session.ID+"/messages", models.PromptRequest{Content: "hello"})
	if promptResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("prompt status = %d, want 503 body=%s", promptResp.Code, promptResp.Body.String())
	}
	errResp := decodeResponse[ErrorResponse](t, promptResp)
	if errResp.Code != "connector_unavailable" {
		t.Fatalf("error code = %q, want connector_unavailable", errResp.Code)
	}
}

func TestSessionModeUsesPutOnly(t *testing.T) {
	router := newRouter(t)
	createResp := doRequest(t, router, http.MethodPost, "/v1/sessions", models.CreateSessionRequest{Title: "demo"})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create session status = %d, want 201 body=%s", createResp.Code, createResp.Body.String())
	}
	session := decodeResponse[models.Session](t, createResp)

	postResp := doRequest(t, router, http.MethodPost, "/v1/sessions/"+session.ID+"/mode", models.SetSessionModeRequest{Mode: "x"})
	if postResp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 body=%s", postResp.Code, postResp.Body.String())
	}
}
