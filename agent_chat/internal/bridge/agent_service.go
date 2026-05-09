package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_chat/internal/gatewayclient"
)

type AgentService struct {
	now           time.Time
	conversations []Conversation
	messages      []MessageEvent
	runs          []RunSummary
	approvals     []ApprovalDecision
	auditEvents   []AuditEvent
	gateway       *gatewayProxy
	devFallback   bool
}

func NewAgentService() *AgentService {
	now := time.Date(2026, 5, 9, 10, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	completed := now
	return &AgentService{
		now: now,
		conversations: []Conversation{
			{ID: "sess_main_20260509_001", Type: "main", Title: "session 事件持久化", Subtitle: "记录 tool call / result / approval 摘要", Status: "waiting_approval", UnreadCount: 2, UpdatedAt: now, WorkspaceID: "workspace_current", CWD: "E:/code/issueye/icoo_ai", Mode: "agent", Model: "gpt-5.4"},
			{ID: "subsess_review_20260509_001", Type: "subagent", Title: "Review Worker", Subtitle: "检查持久化边界与敏感输出策略", Status: "completed", UpdatedAt: now, ParentSessionID: "sess_main_20260509_001", Skill: "security-auditor", WorkspaceID: "workspace_current", CWD: "E:/code/issueye/icoo_ai", Mode: "review", Model: "gpt-5.3-codex"},
			{ID: "sess_ui_20260509_002", Type: "main", Title: "桌面聊天 UI", Subtitle: "Wails + Vue + Pinia 初版骨架", Status: "running", UpdatedAt: now, WorkspaceID: "workspace_agent_chat", CWD: "E:/code/issueye/icoo_ai/agent_chat", Mode: "chat", Model: "gpt-5.4"},
		},
		messages: []MessageEvent{
			{ID: "msg_1", SessionID: "sess_main_20260509_001", Kind: "message", Role: "user", Content: "为 session 增加最小可用的事件/运行摘要持久化能力。", CreatedAt: now},
			{ID: "msg_2", SessionID: "sess_main_20260509_001", Kind: "message", Role: "assistant", Content: "我会先记录必要元信息，避免把敏感大输出落盘。", CreatedAt: now},
			{ID: "tool_1", SessionID: "sess_main_20260509_001", Kind: "tool_call", ToolName: "shell", Status: "completed", DurationMs: 86, Summary: "读取 session store 相关文件", SafeMeta: map[string]any{"command": "rg session", "outputBytes": 18432, "outputHash": "sha256:4b7c...91af", "persistedOutput": false}, CreatedAt: now},
			{ID: "approval_1", SessionID: "sess_main_20260509_001", Kind: "approval", Status: "pending", Decision: "pending", Summary: "允许写入摘要索引，不允许保存完整 tool result。", CreatedAt: now},
			{ID: "run_1", SessionID: "sess_main_20260509_001", Kind: "subagent_run", SubSessionID: "subsess_review_20260509_001", ParentSessionID: "sess_main_20260509_001", Task: "安全审查事件持久化方案", Status: "completed", Summary: "未发现敏感大输出落盘路径。", EventCount: 12, CreatedAt: now},
			{ID: "msg_sub_1", SessionID: "subsess_review_20260509_001", Kind: "message", Role: "assistant", Content: "subagent 使用独立会话 ID：subsess_review_20260509_001。", CreatedAt: now},
			{ID: "msg_ui_1", SessionID: "sess_ui_20260509_002", Kind: "message", Role: "assistant", Content: "正在生成三栏桌面聊天布局。", CreatedAt: now},
		},
		runs: []RunSummary{
			{ID: "run_main_1", SessionID: "sess_main_20260509_001", Status: "waiting_approval", Label: "等待审批", StartedAt: now},
			{ID: "run_sub_1", SessionID: "subsess_review_20260509_001", ParentSessionID: "sess_main_20260509_001", Status: "completed", Label: "安全审查完成", StartedAt: now, CompletedAt: &completed},
		},
		approvals: []ApprovalDecision{
			{ID: "approval_1", SessionID: "sess_main_20260509_001", Decision: "pending", Actor: "user", Summary: "允许保存摘要元信息", CreatedAt: now},
		},
		auditEvents: []AuditEvent{
			{ID: "audit_1", SessionID: "sess_main_20260509_001", Type: "tool_result_summary", Level: "info", Summary: "只保存 outputBytes/outputHash/persistedOutput=false。", CreatedAt: now},
			{ID: "audit_2", SessionID: "sess_main_20260509_001", Type: "approval_requested", Level: "notice", Summary: "等待用户审批写入运行摘要。", CreatedAt: now},
		},
		gateway:     loadGatewayProxy(),
		devFallback: shouldEnableDevFallback(),
	}
}

func (s *AgentService) NewSession(ctx context.Context, req NewSessionRequest) (Conversation, error) {
	if s.gateway != nil {
		var out Conversation
		err := s.gatewayJSON(ctx, http.MethodPost, "/v1/sessions", req, &out)
		if err == nil {
			return out, nil
		}
		if !s.shouldFallback(err) {
			return Conversation{}, err
		}
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "新的 Agent 会话"
	}
	sessionID := fmt.Sprintf("sess_mock_%d", len(s.conversations)+1)
	conversation := Conversation{
		ID:          sessionID,
		Type:        "main",
		Title:       title,
		Subtitle:    "已创建 mock 会话，等待输入",
		Status:      "idle",
		UpdatedAt:   s.now,
		WorkspaceID: fallbackString(req.WorkspaceID, "workspace_current"),
		CWD:         fallbackString(req.Cwd, "E:/code/issueye/icoo_ai"),
		Mode:        fallbackString(req.Mode, "agent"),
		Model:       fallbackString(req.Model, "gpt-5.4"),
	}
	s.conversations = append([]Conversation{conversation}, s.conversations...)
	return conversation, nil
}

func (s *AgentService) LoadSession(ctx context.Context, sessionID string) (Conversation, error) {
	if s.gateway != nil {
		var out Conversation
		err := s.gatewayJSON(ctx, http.MethodGet, "/v1/sessions/"+url.PathEscape(sessionID), nil, &out)
		if err == nil {
			return out, nil
		}
		if !s.shouldFallback(err) {
			return Conversation{}, err
		}
	}
	for _, conversation := range s.conversations {
		if conversation.ID == sessionID {
			return conversation, nil
		}
	}
	return Conversation{}, fmt.Errorf("session %q not found", sessionID)
}

func (s *AgentService) ListConversations(ctx context.Context) ([]Conversation, error) {
	if s.gateway != nil {
		var out []Conversation
		err := s.gatewayJSON(ctx, http.MethodGet, "/v1/sessions", nil, &out)
		if err == nil {
			return out, nil
		}
		if !s.shouldFallback(err) {
			return nil, err
		}
	}
	return append([]Conversation(nil), s.conversations...), nil
}

func (s *AgentService) Prompt(ctx context.Context, req PromptRequest) ([]MessageEvent, error) {
	if s.gateway != nil {
		var out []MessageEvent
		err := s.gatewayJSON(ctx, http.MethodPost, "/v1/sessions/"+url.PathEscape(req.SessionID)+"/prompt", req, &out)
		if err == nil {
			return out, nil
		}
		if !s.shouldFallback(err) {
			return nil, err
		}
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, &BridgeError{Code: ErrorCodeInvalidArgument, Message: "prompt is required", Retryable: false}
	}
	createdAt := s.now.Add(time.Duration(len(s.messages)) * time.Second)
	userMessage := MessageEvent{
		ID:        fmt.Sprintf("msg_user_%d", len(s.messages)+1),
		SessionID: req.SessionID,
		Kind:      "message",
		Role:      "user",
		Content:   prompt,
		CreatedAt: createdAt,
	}
	assistantMessage := MessageEvent{
		ID:        fmt.Sprintf("msg_assistant_%d", len(s.messages)+2),
		SessionID: req.SessionID,
		Kind:      "message",
		Role:      "assistant",
		Status:    "done",
		Content:   "mock bridge 已收到输入。真实 Runtime 接入后这里会由 agent:event 流式更新。",
		CreatedAt: createdAt.Add(time.Second),
	}
	s.messages = append(s.messages, userMessage, assistantMessage)
	s.touchConversation(req.SessionID, "mock bridge 已生成响应", "idle")
	return []MessageEvent{userMessage, assistantMessage}, nil
}

func (s *AgentService) Cancel(ctx context.Context, sessionID string) (RunSummary, error) {
	if s.gateway != nil {
		var out RunSummary
		err := s.gatewayJSON(ctx, http.MethodPost, "/v1/sessions/"+url.PathEscape(sessionID)+"/cancel", nil, &out)
		if err == nil {
			return out, nil
		}
		if !s.shouldFallback(err) {
			return RunSummary{}, err
		}
	}
	s.touchConversation(sessionID, "运行已取消", "cancelled")
	run := RunSummary{ID: fmt.Sprintf("run_cancel_%d", len(s.runs)+1), SessionID: sessionID, Status: "cancelled", Label: "运行已取消", StartedAt: s.now, CompletedAt: &s.now}
	s.runs = append(s.runs, run)
	return run, nil
}

func (s *AgentService) ListMessages(ctx context.Context, sessionID string) ([]MessageEvent, error) {
	if s.gateway != nil {
		var out []MessageEvent
		err := s.gatewayJSON(ctx, http.MethodGet, "/v1/sessions/"+url.PathEscape(sessionID)+"/messages", nil, &out)
		if err == nil {
			return out, nil
		}
		if !s.shouldFallback(err) {
			return nil, err
		}
	}
	filtered := make([]MessageEvent, 0, len(s.messages))
	for _, item := range s.messages {
		if item.SessionID == sessionID {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s *AgentService) ListRuns(ctx context.Context) ([]RunSummary, error) {
	if s.gateway != nil {
		var out []RunSummary
		err := s.gatewayJSON(ctx, http.MethodGet, "/v1/runs", nil, &out)
		if err == nil {
			return out, nil
		}
		if !s.shouldFallback(err) {
			return nil, err
		}
	}
	return append([]RunSummary(nil), s.runs...), nil
}

func (s *AgentService) ListApprovals(ctx context.Context) ([]ApprovalDecision, error) {
	if s.gateway != nil {
		var out []ApprovalDecision
		err := s.gatewayJSON(ctx, http.MethodGet, "/v1/approvals", nil, &out)
		if err == nil {
			return out, nil
		}
		if !s.shouldFallback(err) {
			return nil, err
		}
	}
	return append([]ApprovalDecision(nil), s.approvals...), nil
}

func (s *AgentService) DecideApproval(ctx context.Context, req ApprovalDecisionRequest) (ApprovalDecision, error) {
	if s.gateway != nil {
		var out ApprovalDecision
		err := s.gatewayJSON(ctx, http.MethodPost, "/v1/approvals/"+url.PathEscape(req.ID)+"/decision", req, &out)
		if err == nil {
			return out, nil
		}
		if !s.shouldFallback(err) {
			return ApprovalDecision{}, err
		}
	}
	decision := ApprovalDecision{ID: req.ID, SessionID: req.SessionID, Decision: req.Decision, Actor: "user", Summary: "用户已处理审批请求", CreatedAt: s.now}
	for i := range s.approvals {
		if s.approvals[i].ID == req.ID {
			s.approvals[i] = decision
		}
	}
	for i := range s.messages {
		if s.messages[i].ID == req.ID {
			s.messages[i].Decision = req.Decision
			s.messages[i].Status = "decided"
		}
	}
	s.auditEvents = append(s.auditEvents, AuditEvent{ID: fmt.Sprintf("audit_%d", len(s.auditEvents)+1), SessionID: req.SessionID, Type: "approval_decision", Level: "notice", Summary: "用户决策：" + req.Decision, CreatedAt: s.now})
	s.touchConversation(req.SessionID, "审批已处理："+req.Decision, "idle")
	return decision, nil
}

func (s *AgentService) ListSkills(ctx context.Context) ([]SkillInfo, error) {
	if s.gateway != nil {
		var out []SkillInfo
		err := s.gatewayJSON(ctx, http.MethodGet, "/v1/skills", nil, &out)
		if err == nil {
			return out, nil
		}
		if !s.shouldFallback(err) {
			return nil, err
		}
	}
	return []SkillInfo{{ID: "security-auditor", Name: "security-auditor", Description: "审查敏感输出与权限边界。"}}, nil
}

func (s *AgentService) ListAuditEvents(ctx context.Context) ([]AuditEvent, error) {
	return append([]AuditEvent(nil), s.auditEvents...), nil
}

func (s *AgentService) touchConversation(sessionID string, subtitle string, status string) {
	for i := range s.conversations {
		if s.conversations[i].ID == sessionID {
			s.conversations[i].Subtitle = subtitle
			s.conversations[i].Status = status
			s.conversations[i].UpdatedAt = s.now
			return
		}
	}
}

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

type gatewayProxy struct {
	client  *http.Client
	baseURL string
	token   string
}

func loadGatewayProxy() *gatewayProxy {
	discoveryPath := strings.TrimSpace(os.Getenv("ICOO_GATEWAY_DISCOVERY_PATH"))
	endpoint, token, err := gatewayclient.DiscoverFromPath(discoveryPath)
	if err != nil {
		return nil
	}
	if override := strings.TrimSpace(os.Getenv("ICOO_GATEWAY_TOKEN")); override != "" {
		token = override
	}
	return &gatewayProxy{
		client:  http.DefaultClient,
		baseURL: strings.TrimRight(endpoint.BaseURL, "/"),
		token:   strings.TrimSpace(token),
	}
}

func shouldEnableDevFallback() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("ICOO_BRIDGE_DEV_FALLBACK")), "false") {
		return false
	}
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ICOO_BRIDGE_ENV")))
	return env == "" || env == "dev" || env == "development"
}

func (s *AgentService) gatewayJSON(ctx context.Context, method, rawPath string, payload any, out any) error {
	if s.gateway == nil {
		return &BridgeError{Code: ErrorCodeGatewayUnavailable, Message: "gateway client is not configured", Retryable: true}
	}
	u, err := url.Parse(s.gateway.baseURL)
	if err != nil {
		return &BridgeError{Code: ErrorCodeGatewayUnavailable, Message: "gateway base URL is invalid", Retryable: false}
	}
	u.Path = path.Join(u.Path, rawPath)
	var body io.Reader
	if payload != nil {
		data, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return &BridgeError{Code: ErrorCodeGatewayRequest, Message: "encode gateway request failed", Retryable: false}
		}
		body = strings.NewReader(string(data))
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return &BridgeError{Code: ErrorCodeGatewayRequest, Message: "build gateway request failed", Retryable: false}
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.gateway.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.gateway.token)
	}
	resp, err := s.gateway.client.Do(req)
	if err != nil {
		return &BridgeError{Code: ErrorCodeGatewayUnavailable, Message: "gateway is unreachable", Retryable: true}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &BridgeError{Code: ErrorCodeGatewayAuthFailed, Message: "gateway token is invalid or expired", StatusCode: resp.StatusCode, Retryable: false}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		message := strings.TrimSpace(string(detail))
		if message == "" {
			message = fmt.Sprintf("gateway request failed with status %d", resp.StatusCode)
		}
		return &BridgeError{Code: ErrorCodeGatewayRequest, Message: message, StatusCode: resp.StatusCode, Retryable: resp.StatusCode >= 500}
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return &BridgeError{Code: ErrorCodeGatewayRequest, Message: "decode gateway response failed", Retryable: false}
	}
	return nil
}

func (s *AgentService) shouldFallback(err error) bool {
	if !s.devFallback {
		return false
	}
	bridgeErr, ok := err.(*BridgeError)
	if !ok {
		return false
	}
	return bridgeErr.Code == ErrorCodeGatewayUnavailable
}
