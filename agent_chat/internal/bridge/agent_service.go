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

const (
	maxGatewayStreamFailures = 8
)

type AgentService struct {
	mu               sync.RWMutex
	messages         []MessageEvent
	auditEvents      []AuditEvent
	gateway          *gatewayProxy
	bootstrap        *gatewayBootstrapper
	lastEventID      string
	currentSessionID string
	activeSessions   map[string]struct{}
	eventSink        func(MessageEvent)
	gatewayStatus    string
	gatewaySummary   string
	gatewayUpdatedAt time.Time
	serviceCtx       context.Context
	streamMu         sync.Mutex
	streamCancel     context.CancelFunc
}

func NewAgentService() *AgentService {
	return &AgentService{
		messages:       make([]MessageEvent, 0, 32),
		auditEvents:    make([]AuditEvent, 0, 16),
		gateway:        loadGatewayProxy(),
		bootstrap:      newGatewayBootstrapper(),
		activeSessions: make(map[string]struct{}),
	}
}

func (s *AgentService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.serviceCtx = ctx
	if s.eventSink == nil {
		s.eventSink = func(event MessageEvent) {
			app := application.Get()
			if app != nil {
				app.Event.Emit("agent:event", event)
			}
		}
	}
	s.emitGatewayStatus(GatewayStatusConnecting, "正在连接网关服务", nil)
	if err := s.ensureGatewayRunning(ctx); err != nil {
		s.emitGatewayStatus(GatewayStatusFailed, "网关启动失败", map[string]any{
			"error": err.Error(),
		})
		return err
	}
	if s.gateway == nil {
		s.emitGatewayStatus(GatewayStatusFailed, "网关未配置", nil)
		return &BridgeError{
			Code:      ErrorCodeGatewayUnavailable,
			Message:   "gateway is not configured",
			Retryable: false,
		}
	}
	s.emitGatewayStatus(GatewayStatusReady, "网关连接已就绪", nil)
	s.startGatewayEventStream(ctx)
	return nil
}

func (s *AgentService) ServiceShutdown() error {
	s.stopGatewayEventStream()
	if s.bootstrap == nil {
		return nil
	}
	return s.bootstrap.StopManagedProcess()
}

func (s *AgentService) RestartGateway(ctx context.Context) (GatewayStatus, error) {
	s.emitGatewayStatus(GatewayStatusReconnecting, "正在重启网关服务", nil)
	s.stopGatewayEventStream()
	if s.bootstrap != nil {
		if err := s.bootstrap.StopManagedProcess(); err != nil {
			return GatewayStatus{}, &BridgeError{
				Code:      ErrorCodeGatewayBootstrap,
				Message:   fmt.Sprintf("stop managed gateway process failed: %v", err),
				Retryable: true,
			}
		}
	}
	s.mu.Lock()
	s.gateway = nil
	s.mu.Unlock()
	if err := s.ensureGatewayRunning(ctx); err != nil {
		s.emitGatewayStatus(GatewayStatusFailed, "网关重启失败", map[string]any{
			"error": err.Error(),
		})
		return GatewayStatus{}, err
	}
	s.emitGatewayStatus(GatewayStatusReady, "网关重启完成", nil)
	baseCtx := s.serviceCtx
	if baseCtx == nil {
		baseCtx = ctx
	}
	if baseCtx != nil {
		s.startGatewayEventStream(baseCtx)
	}
	return s.GetGatewayStatus(ctx)
}

func (s *AgentService) ensureGatewayRunning(ctx context.Context) error {
	if s.gateway != nil {
		if err := s.pingGateway(ctx, s.gateway); err == nil {
			return nil
		}
		s.gateway = nil
	}
	if s.bootstrap == nil {
		return &BridgeError{
			Code:      ErrorCodeGatewayBootstrap,
			Message:   "gateway bootstrap is not configured",
			Retryable: true,
		}
	}
	proxy, err := s.bootstrap.EnsureRunning(ctx)
	if err != nil {
		return &BridgeError{
			Code:      ErrorCodeGatewayBootstrap,
			Message:   err.Error(),
			Retryable: false,
		}
	}
	s.gateway = proxy
	return nil
}

func (s *AgentService) pingGateway(ctx context.Context, proxy *gatewayProxy) error {
	if proxy == nil {
		return fmt.Errorf("gateway proxy is nil")
	}
	healthCtx, cancel := context.WithTimeout(ctx, gatewayHealthTimeout)
	defer cancel()
	_, err := gatewayclient.New(proxy.baseURL, proxy.token).Health(healthCtx)
	return err
}

func (s *AgentService) NewSession(ctx context.Context, req NewSessionRequest) (Conversation, error) {
	var out gatewaySessionDTO
	err := s.gatewayJSON(ctx, http.MethodPost, "/v1/sessions", mapCreateSessionRequest(req), &out)
	if err != nil {
		return Conversation{}, err
	}
	conversation := mapGatewaySessionToConversation(out)
	s.setCurrentStreamSessionID(conversation.ID)
	return conversation, nil
}

func (s *AgentService) LoadSession(ctx context.Context, sessionID string) (Conversation, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Conversation{}, &BridgeError{Code: ErrorCodeInvalidArgument, Message: "sessionId is required", Retryable: false}
	}
	var out gatewaySessionDTO
	if err := s.gatewayJSON(ctx, http.MethodGet, "/v1/sessions/"+url.PathEscape(sessionID), nil, &out); err != nil {
		return Conversation{}, err
	}
	conversation := mapGatewaySessionToConversation(out)
	s.setCurrentStreamSessionID(sessionID)
	return conversation, nil
}

func (s *AgentService) ListConversations(ctx context.Context) ([]Conversation, error) {
	var out []gatewaySessionDTO
	if err := s.gatewayJSON(ctx, http.MethodGet, "/v1/sessions", nil, &out); err != nil {
		return nil, err
	}
	return mapGatewaySessionsToConversations(out), nil
}

func (s *AgentService) Prompt(ctx context.Context, req PromptRequest) ([]MessageEvent, error) {
	req.SessionID = strings.TrimSpace(req.SessionID)
	if req.SessionID == "" {
		return nil, &BridgeError{Code: ErrorCodeInvalidArgument, Message: "sessionId is required", Retryable: false}
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		return nil, &BridgeError{Code: ErrorCodeInvalidArgument, Message: "content is required", Retryable: false}
	}
	var raw json.RawMessage
	if err := s.gatewayJSON(ctx, http.MethodPost, "/v1/sessions/"+url.PathEscape(req.SessionID)+"/prompt", mapPromptRequest(req), &raw); err != nil {
		return nil, err
	}
	out, mapErr := parseGatewayPromptResponse(raw, req.SessionID)
	if mapErr != nil {
		return nil, &BridgeError{Code: ErrorCodeGatewayRequest, Message: "decode gateway prompt response failed", Retryable: false}
	}
	out = normalizeMessageEvents(out, req.SessionID)
	s.setCurrentStreamSessionID(req.SessionID)
	return out, nil
}

func (s *AgentService) Cancel(ctx context.Context, sessionID string) (RunSummary, error) {
	var out gatewayRunDTO
	if err := s.gatewayJSON(ctx, http.MethodPost, "/v1/sessions/"+url.PathEscape(sessionID)+"/cancel", nil, &out); err != nil {
		return RunSummary{}, err
	}
	s.setCurrentStreamSessionID(sessionID)
	return mapGatewayRunToSummary(out), nil
}

func (s *AgentService) ListMessages(ctx context.Context, sessionID string) ([]MessageEvent, error) {
	var out []gatewayMessageDTO
	if err := s.gatewayJSON(ctx, http.MethodGet, "/v1/sessions/"+url.PathEscape(sessionID)+"/messages", nil, &out); err != nil {
		return nil, err
	}
	filtered := make([]MessageEvent, 0, len(out))
	for _, item := range out {
		filtered = append(filtered, mapGatewayMessageToEvent(item, sessionID))
	}
	s.setCurrentStreamSessionID(sessionID)
	return filtered, nil
}

func (s *AgentService) ListRuns(ctx context.Context) ([]RunSummary, error) {
	var out []gatewayRunDTO
	if err := s.gatewayJSON(ctx, http.MethodGet, "/v1/runs", nil, &out); err != nil {
		return nil, err
	}
	return mapGatewayRunsToSummaries(out), nil
}

func (s *AgentService) ListApprovals(ctx context.Context) ([]ApprovalDecision, error) {
	var out []gatewayApprovalDTO
	if err := s.gatewayJSON(ctx, http.MethodGet, "/v1/approvals", nil, &out); err != nil {
		return nil, err
	}
	return mapGatewayApprovalsToDecisions(out), nil
}

func (s *AgentService) DecideApproval(ctx context.Context, req ApprovalDecisionRequest) (ApprovalDecision, error) {
	var out gatewayApprovalDTO
	if err := s.gatewayJSON(ctx, http.MethodPost, "/v1/approvals/"+url.PathEscape(req.ID)+"/decision", mapApprovalDecisionRequest(req), &out); err != nil {
		return ApprovalDecision{}, err
	}
	return mapGatewayApprovalToDecision(out), nil
}

func (s *AgentService) ListSkills(ctx context.Context) ([]SkillInfo, error) {
	var out []gatewaySkillDTO
	if err := s.gatewayJSON(ctx, http.MethodGet, "/v1/skills", nil, &out); err != nil {
		return nil, err
	}
	return mapGatewaySkillsToInfos(out), nil
}

func (s *AgentService) ListAuditEvents(ctx context.Context) ([]AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]AuditEvent(nil), s.auditEvents...), nil
}

func (s *AgentService) GetGatewayStatus(ctx context.Context) (GatewayStatus, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := strings.TrimSpace(s.gatewayStatus)
	if status == "" {
		status = GatewayStatusConnecting
	}
	return GatewayStatus{
		Status:    status,
		Summary:   s.gatewaySummary,
		UpdatedAt: s.gatewayUpdatedAt,
	}, nil
}

func (s *AgentService) streamGatewayEvents(ctx context.Context) {
	client := gatewayclient.New(s.gateway.baseURL, s.gateway.token)
	backoff := time.Second
	failures := 0
	for {
		if ctx.Err() != nil {
			return
		}
		if failures > 0 {
			s.emitGatewayStatus(GatewayStatusReconnecting, "网关事件流重连中", map[string]any{
				"attempt": failures + 1,
			})
		}
		lastEventID, sessionID := s.streamSubscriptionState()
		err := client.StreamEventsWithFilter(ctx, lastEventID, sessionID, "", func(event gatewayclient.StreamEnvelope) error {
			if failures > 0 {
				failures = 0
				backoff = time.Second
				s.emitGatewayStatus(GatewayStatusReady, "网关事件流已恢复", nil)
			}
			return s.forwardGatewayEvent(event)
		})
		if ctx.Err() != nil {
			return
		}
		bridgeErr := s.mapGatewayStreamError(err)
		if bridgeErr != nil && bridgeErr.Code == ErrorCodeGatewayAuthFailed {
			s.emitGatewayStatus(GatewayStatusFailed, "网关鉴权失败", map[string]any{
				"code":   bridgeErr.Code,
				"status": bridgeErr.StatusCode,
				"error":  bridgeErr.Message,
			})
			return
		}
		if bridgeErr != nil {
			failures++
			if failures >= maxGatewayStreamFailures {
				s.emitGatewayStatus(GatewayStatusFailed, "网关事件流重连失败", map[string]any{
					"code":     bridgeErr.Code,
					"status":   bridgeErr.StatusCode,
					"error":    bridgeErr.Message,
					"failures": failures,
				})
				return
			}
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

func (s *AgentService) startGatewayEventStream(baseCtx context.Context) {
	if baseCtx == nil {
		return
	}
	s.streamMu.Lock()
	if s.streamCancel != nil {
		s.streamCancel()
		s.streamCancel = nil
	}
	streamCtx, cancel := context.WithCancel(baseCtx)
	s.streamCancel = cancel
	s.streamMu.Unlock()
	go s.streamGatewayEvents(streamCtx)
}

func (s *AgentService) stopGatewayEventStream() {
	s.streamMu.Lock()
	cancel := s.streamCancel
	s.streamCancel = nil
	s.streamMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *AgentService) emitGatewayStatus(status, summary string, meta map[string]any) {
	status = strings.TrimSpace(status)
	if status == "" {
		return
	}
	s.mu.Lock()
	if s.gatewayStatus == status && s.gatewaySummary == summary {
		s.mu.Unlock()
		return
	}
	s.gatewayStatus = status
	s.gatewaySummary = summary
	s.gatewayUpdatedAt = time.Now()
	event := MessageEvent{
		ID:        fmt.Sprintf("gateway_status_%d", time.Now().UnixNano()),
		Kind:      BridgeEventKindGateway,
		Status:    status,
		Summary:   summary,
		CreatedAt: s.gatewayUpdatedAt,
		SafeMeta:  map[string]any{"gatewayStatus": status},
	}
	if meta != nil {
		for k, v := range meta {
			event.SafeMeta[k] = v
		}
	}
	s.messages = append(s.messages, event)
	s.mu.Unlock()
	if s.eventSink != nil {
		s.eventSink(event)
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
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	s.currentSessionID = sessionID
	if s.activeSessions == nil {
		s.activeSessions = make(map[string]struct{})
	}
	s.activeSessions[sessionID] = struct{}{}
	s.mu.Unlock()
}

func (s *AgentService) streamSubscriptionState() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if strings.TrimSpace(s.currentSessionID) != "" {
		return s.lastEventID, s.currentSessionID
	}
	if len(s.activeSessions) == 1 {
		for id := range s.activeSessions {
			return s.lastEventID, id
		}
	}
	return s.lastEventID, ""
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
	if out.Kind == BridgeEventKindAudit {
		audit := AuditEvent{
			ID:        out.ID,
			SessionID: out.SessionID,
			Type:      strings.TrimSpace(envelope.Type),
			Level:     "info",
			Summary:   out.Summary,
			CreatedAt: out.CreatedAt,
		}
		if audit.Type == "" {
			audit.Type = "audit"
		}
		s.auditEvents = append(s.auditEvents, audit)
	}
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

type gatewaySessionDTO struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	CWD            string    `json:"cwd,omitempty"`
	StartupCommand string    `json:"startupCommand,omitempty"`
	AgentID        string    `json:"agentId"`
	Model          string    `json:"model,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type gatewayCreateSessionRequest struct {
	Title          string `json:"title"`
	CWD            string `json:"cwd,omitempty"`
	StartupCommand string `json:"startupCommand,omitempty"`
	Model          string `json:"model,omitempty"`
}

type gatewayPromptRequest struct {
	Content string `json:"content"`
}

type gatewayMessageDTO struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	RunID     string    `json:"runId,omitempty"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type gatewayRunDTO struct {
	ID        string     `json:"id"`
	SessionID string     `json:"sessionId"`
	AgentID   string     `json:"agentId"`
	Status    string     `json:"status"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
}

type gatewayApprovalDTO struct {
	ID        string     `json:"id"`
	AgentID   string     `json:"agentId"`
	SessionID string     `json:"sessionId"`
	RunID     string     `json:"runId"`
	Status    string     `json:"status"`
	Action    string     `json:"action"`
	Message   string     `json:"message"`
	Decision  string     `json:"decision,omitempty"`
	DecidedAt *time.Time `json:"decidedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type gatewayApprovalDecisionRequest struct {
	Decision string `json:"decision"`
}

type gatewaySkillDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type gatewayPromptResponse struct {
	Run      gatewayRunDTO       `json:"run"`
	Messages []gatewayMessageDTO `json:"messages"`
	Approval *gatewayApprovalDTO `json:"approval,omitempty"`
}

func mapCreateSessionRequest(in NewSessionRequest) gatewayCreateSessionRequest {
	return gatewayCreateSessionRequest{
		Title:          strings.TrimSpace(in.Title),
		CWD:            strings.TrimSpace(in.Cwd),
		StartupCommand: strings.TrimSpace(in.StartupCommand),
		Model:          strings.TrimSpace(in.Model),
	}
}

func mapPromptRequest(in PromptRequest) gatewayPromptRequest {
	return gatewayPromptRequest{
		Content: strings.TrimSpace(in.Content),
	}
}

func mapApprovalDecisionRequest(in ApprovalDecisionRequest) gatewayApprovalDecisionRequest {
	return gatewayApprovalDecisionRequest{
		Decision: strings.TrimSpace(in.Decision),
	}
}

func mapGatewaySessionToConversation(in gatewaySessionDTO) Conversation {
	updatedAt := in.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = in.CreatedAt
	}
	conversationType := "main"
	if strings.HasPrefix(in.ID, "subsess_") {
		conversationType = "subagent"
	}
	subtitle := strings.TrimSpace(in.Status)
	if subtitle == "" {
		subtitle = "unknown"
	}
	return Conversation{
		ID:             in.ID,
		Type:           conversationType,
		Title:          in.Title,
		Subtitle:       subtitle,
		Status:         in.Status,
		UnreadCount:    0,
		UpdatedAt:      updatedAt,
		CWD:            in.CWD,
		StartupCommand: in.StartupCommand,
		Mode:           in.AgentID,
		Model:          in.Model,
	}
}

func mapGatewaySessionsToConversations(in []gatewaySessionDTO) []Conversation {
	out := make([]Conversation, 0, len(in))
	for _, item := range in {
		out = append(out, mapGatewaySessionToConversation(item))
	}
	return out
}

func runStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return "运行中"
	case "completed":
		return "已完成"
	case "failed":
		return "已失败"
	case "cancelled":
		return "已取消"
	case "waiting_approval":
		return "等待审批"
	case "queued":
		return "排队中"
	default:
		if strings.TrimSpace(status) == "" {
			return "未知状态"
		}
		return status
	}
}

func mapGatewayRunToSummary(in gatewayRunDTO) RunSummary {
	return RunSummary{
		ID:          in.ID,
		SessionID:   in.SessionID,
		Status:      in.Status,
		Label:       runStatusLabel(in.Status),
		StartedAt:   in.StartedAt,
		CompletedAt: in.EndedAt,
	}
}

func mapGatewayRunsToSummaries(in []gatewayRunDTO) []RunSummary {
	out := make([]RunSummary, 0, len(in))
	for _, item := range in {
		out = append(out, mapGatewayRunToSummary(item))
	}
	return out
}

func mapGatewayApprovalToDecision(in gatewayApprovalDTO) ApprovalDecision {
	decision := strings.TrimSpace(in.Decision)
	if decision == "" {
		decision = strings.TrimSpace(in.Status)
	}
	summary := strings.TrimSpace(in.Message)
	if summary == "" {
		summary = strings.TrimSpace(in.Action)
	}
	return ApprovalDecision{
		ID:        in.ID,
		SessionID: in.SessionID,
		Decision:  decision,
		Actor:     "gateway",
		Summary:   summary,
		CreatedAt: in.CreatedAt,
	}
}

func mapGatewayApprovalsToDecisions(in []gatewayApprovalDTO) []ApprovalDecision {
	out := make([]ApprovalDecision, 0, len(in))
	for _, item := range in {
		out = append(out, mapGatewayApprovalToDecision(item))
	}
	return out
}

func mapGatewaySkillsToInfos(in []gatewaySkillDTO) []SkillInfo {
	out := make([]SkillInfo, 0, len(in))
	for _, item := range in {
		out = append(out, SkillInfo{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
		})
	}
	return out
}

func mapGatewayMessageToEvent(in gatewayMessageDTO, fallbackSessionID string) MessageEvent {
	sessionID := strings.TrimSpace(in.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(fallbackSessionID)
	}
	return MessageEvent{
		ID:        in.ID,
		SessionID: sessionID,
		Kind:      BridgeEventKindMessage,
		Role:      in.Role,
		Content:   in.Content,
		CreatedAt: in.CreatedAt,
	}
}

func parseGatewayPromptResponse(raw json.RawMessage, fallbackSessionID string) ([]MessageEvent, error) {
	var structured gatewayPromptResponse
	if err := json.Unmarshal(raw, &structured); err != nil {
		return nil, err
	}
	out := make([]MessageEvent, 0, len(structured.Messages)+1)
	for _, message := range structured.Messages {
		out = append(out, mapGatewayMessageToEvent(message, fallbackSessionID))
	}
	if structured.Approval != nil {
		approval := mapGatewayApprovalToDecision(*structured.Approval)
		status := strings.TrimSpace(structured.Approval.Status)
		if status == "" {
			status = approval.Decision
		}
		out = append(out, MessageEvent{
			ID:        approval.ID,
			SessionID: approval.SessionID,
			Kind:      BridgeEventKindApproval,
			Status:    status,
			Decision:  approval.Decision,
			Summary:   approval.Summary,
			CreatedAt: approval.CreatedAt,
		})
	}
	return out, nil
}

func normalizeMessageEvents(items []MessageEvent, sessionID string) []MessageEvent {
	normalized := make([]MessageEvent, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.SessionID) == "" {
			item.SessionID = sessionID
		}
		if strings.TrimSpace(item.Kind) == "" {
			if strings.TrimSpace(item.Role) != "" {
				item.Kind = BridgeEventKindMessage
			} else {
				item.Kind = BridgeEventKindGateway
			}
		}
		normalized = append(normalized, item)
	}
	return normalized
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
