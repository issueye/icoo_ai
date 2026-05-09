package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_chat/internal/gatewayclient"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type AgentService struct {
	mu               sync.RWMutex
	now              time.Time
	conversations    []Conversation
	messages         []MessageEvent
	runs             []RunSummary
	approvals        []ApprovalDecision
	auditEvents      []AuditEvent
	gateway          *gatewayProxy
	devFallback      bool
	lastEventID      string
	currentSessionID string
	eventSink        func(MessageEvent)
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

func (s *AgentService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	if s.eventSink == nil {
		s.eventSink = func(event MessageEvent) {
			app := application.Get()
			if app != nil {
				app.Event.Emit("agent:event", event)
			}
		}
	}
	if s.gateway == nil {
		return nil
	}
	go s.streamGatewayEvents(ctx)
	return nil
}

func (s *AgentService) NewSession(ctx context.Context, req NewSessionRequest) (Conversation, error) {
	if s.gateway != nil {
		var out Conversation
		err := s.gatewayJSON(ctx, http.MethodPost, "/v1/sessions", req, &out)
		if err == nil {
			s.setCurrentStreamSessionID(out.ID)
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
	s.mu.Lock()
	s.conversations = append([]Conversation{conversation}, s.conversations...)
	s.mu.Unlock()
	s.setCurrentStreamSessionID(conversation.ID)
	return conversation, nil
}

func (s *AgentService) LoadSession(ctx context.Context, sessionID string) (Conversation, error) {
	if s.gateway != nil {
		var out Conversation
		err := s.gatewayJSON(ctx, http.MethodGet, "/v1/sessions/"+url.PathEscape(sessionID), nil, &out)
		if err == nil {
			s.setCurrentStreamSessionID(out.ID)
			return out, nil
		}
		if !s.shouldFallback(err) {
			return Conversation{}, err
		}
	}
	for _, conversation := range s.conversations {
		if conversation.ID == sessionID {
			s.setCurrentStreamSessionID(sessionID)
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Conversation(nil), s.conversations...), nil
}

func (s *AgentService) Prompt(ctx context.Context, req PromptRequest) ([]MessageEvent, error) {
	if s.gateway != nil {
		var out []MessageEvent
		err := s.gatewayJSON(ctx, http.MethodPost, "/v1/sessions/"+url.PathEscape(req.SessionID)+"/prompt", req, &out)
		if err == nil {
			s.setCurrentStreamSessionID(req.SessionID)
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
	s.mu.Lock()
	s.messages = append(s.messages, userMessage, assistantMessage)
	s.mu.Unlock()
	s.touchConversation(req.SessionID, "mock bridge 已生成响应", "idle")
	s.setCurrentStreamSessionID(req.SessionID)
	return []MessageEvent{userMessage, assistantMessage}, nil
}

func (s *AgentService) Cancel(ctx context.Context, sessionID string) (RunSummary, error) {
	if s.gateway != nil {
		var out RunSummary
		err := s.gatewayJSON(ctx, http.MethodPost, "/v1/sessions/"+url.PathEscape(sessionID)+"/cancel", nil, &out)
		if err == nil {
			s.setCurrentStreamSessionID(sessionID)
			return out, nil
		}
		if !s.shouldFallback(err) {
			return RunSummary{}, err
		}
	}
	s.touchConversation(sessionID, "运行已取消", "cancelled")
	s.setCurrentStreamSessionID(sessionID)
	run := RunSummary{ID: fmt.Sprintf("run_cancel_%d", len(s.runs)+1), SessionID: sessionID, Status: "cancelled", Label: "运行已取消", StartedAt: s.now, CompletedAt: &s.now}
	s.runs = append(s.runs, run)
	return run, nil
}

func (s *AgentService) ListMessages(ctx context.Context, sessionID string) ([]MessageEvent, error) {
	if s.gateway != nil {
		var out []MessageEvent
		err := s.gatewayJSON(ctx, http.MethodGet, "/v1/sessions/"+url.PathEscape(sessionID)+"/messages", nil, &out)
		if err == nil {
			s.setCurrentStreamSessionID(sessionID)
			return out, nil
		}
		if !s.shouldFallback(err) {
			return nil, err
		}
	}
	filtered := make([]MessageEvent, 0, len(s.messages))
	s.mu.RLock()
	for _, item := range s.messages {
		if item.SessionID == sessionID {
			filtered = append(filtered, item)
		}
	}
	s.mu.RUnlock()
	s.setCurrentStreamSessionID(sessionID)
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
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	s.mu.Lock()
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
	s.mu.Unlock()
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]AuditEvent(nil), s.auditEvents...), nil
}

func (s *AgentService) touchConversation(sessionID string, subtitle string, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.conversations {
		if s.conversations[i].ID == sessionID {
			s.conversations[i].Subtitle = subtitle
			s.conversations[i].Status = status
			s.conversations[i].UpdatedAt = s.now
			return
		}
	}
}

func (s *AgentService) streamGatewayEvents(ctx context.Context) {
	client := gatewayclient.New(s.gateway.baseURL, s.gateway.token)
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		lastEventID, sessionID := s.streamSubscriptionState()
		err := client.StreamEventsWithFilter(ctx, lastEventID, sessionID, "", func(event gatewayclient.StreamEnvelope) error {
			return s.forwardGatewayEvent(event)
		})
		if ctx.Err() != nil {
			return
		}
		bridgeErr := s.mapGatewayStreamError(err)
		if bridgeErr != nil && bridgeErr.Code == ErrorCodeGatewayAuthFailed {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 10*time.Second {
			backoff *= 2
			if backoff > 10*time.Second {
				backoff = 10 * time.Second
			}
		}
	}
}

func (s *AgentService) lastStreamEventID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastEventID
}

func (s *AgentService) setLastStreamEventID(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	s.lastEventID = id
	s.mu.Unlock()
}

func (s *AgentService) setCurrentStreamSessionID(sessionID string) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	s.mu.Lock()
	s.currentSessionID = sessionID
	s.mu.Unlock()
}

func (s *AgentService) streamSubscriptionState() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastEventID, s.currentSessionID
}

func (s *AgentService) forwardGatewayEvent(in gatewayclient.StreamEnvelope) error {
	envelope := GatewayEventEnvelope{
		ID:        in.ID,
		Type:      in.Type,
		AgentID:   in.AgentID,
		SessionID: in.SessionID,
		RunID:     in.RunID,
		Payload:   in.Payload,
	}
	if in.CreatedAt != "" {
		if ts, err := time.Parse(time.RFC3339, in.CreatedAt); err == nil {
			envelope.CreatedAt = ts
		}
	}
	out := s.mapEnvelopeToMessageEvent(envelope)
	s.mu.Lock()
	s.messages = append(s.messages, out)
	s.mu.Unlock()
	s.setLastStreamEventID(envelope.ID)
	if s.eventSink != nil {
		s.eventSink(out)
	}
	return nil
}

func (s *AgentService) mapEnvelopeToMessageEvent(envelope GatewayEventEnvelope) MessageEvent {
	var payload MessageEvent
	_ = json.Unmarshal(envelope.Payload, &payload)
	if payload.ID == "" {
		payload.ID = envelope.ID
	}
	if payload.SessionID == "" {
		payload.SessionID = envelope.SessionID
	}
	payload.Kind = mapGatewayEventTypeToBridgeKind(envelope.Type, payload.Kind)
	if payload.CreatedAt.IsZero() {
		if !envelope.CreatedAt.IsZero() {
			payload.CreatedAt = envelope.CreatedAt
		} else {
			payload.CreatedAt = time.Now()
		}
	}
	if !isKnownBridgeKind(payload.Kind) {
		originalKind := payload.Kind
		payload.Kind = BridgeEventKindGateway
		if payload.SafeMeta == nil {
			payload.SafeMeta = make(map[string]any, 2)
		}
		if strings.TrimSpace(envelope.Type) != "" {
			payload.SafeMeta["gatewayType"] = envelope.Type
		}
		if strings.TrimSpace(originalKind) != "" {
			payload.SafeMeta["gatewayKind"] = originalKind
		}
	}
	return payload
}

func mapGatewayEventTypeToBridgeKind(eventType string, payloadKind string) string {
	normalized := strings.ToLower(strings.TrimSpace(eventType))
	switch normalized {
	case "message", "msg", "agent.message", "session.message", "conversation.message":
		return BridgeEventKindMessage
	case "tool_call", "tool.call", "tool_started", "tool.start":
		return BridgeEventKindToolCall
	case "tool_result", "tool.result", "tool_completed", "tool.complete":
		return BridgeEventKindToolResult
	case "approval", "approval_required", "approval.requested", "approval_requested", "approval_decision", "approval.decided":
		return BridgeEventKindApproval
	case "subagent_run", "subagent.run", "subagent_started", "subagent.start", "subagent_completed", "subagent.complete":
		return BridgeEventKindSubagent
	case "run", "run_started", "run.start", "run_completed", "run.complete", "run_failed", "run.cancelled", "run_cancelled":
		return BridgeEventKindRun
	case "audit", "audit_event", "audit.event":
		return BridgeEventKindAudit
	}

	if strings.TrimSpace(payloadKind) != "" {
		return payloadKind
	}
	return eventType
}

func isKnownBridgeKind(kind string) bool {
	switch kind {
	case BridgeEventKindMessage, BridgeEventKindToolCall, BridgeEventKindToolResult, BridgeEventKindApproval, BridgeEventKindSubagent, BridgeEventKindRun, BridgeEventKindAudit:
		return true
	default:
		return false
	}
}

func (s *AgentService) mapGatewayStreamError(err error) *BridgeError {
	if err == nil {
		return nil
	}
	type statusCoder interface {
		StatusCode() int
	}
	var statusErr statusCoder
	if errors.As(err, &statusErr) {
		status := statusErr.StatusCode()
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			return &BridgeError{Code: ErrorCodeGatewayAuthFailed, Message: "gateway token is invalid or expired", StatusCode: status, Retryable: false}
		}
		return &BridgeError{Code: ErrorCodeGatewayStream, Message: err.Error(), StatusCode: status, Retryable: status >= 500}
	}
	return &BridgeError{Code: ErrorCodeGatewayUnavailable, Message: err.Error(), Retryable: true}
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
