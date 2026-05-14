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
	maxGatewayStreamFailures   = 8
	maxGatewayAuthFailures     = 3
	gatewayStreamProbeAttempts = 5
	gatewayStreamProbeTimeout  = 1200 * time.Millisecond
	gatewayStreamProbeBackoff  = 250 * time.Millisecond
	maxAuditEventCacheSize     = 5000
)

type AgentService struct {
	mu                sync.RWMutex
	messages          []MessageEvent
	conversations     []Conversation
	auditEvents       []AuditEvent
	gateway           *gatewayProxy
	bootstrap         *gatewayBootstrapper
	lastEventID       string
	currentSessionID  string
	activeSessions    map[string]struct{}
	sessionAgents     map[string]string
	eventSink         func(MessageEvent)
	gatewayStatus     string
	gatewaySummary    string
	gatewayUpdatedAt  time.Time
	serviceCtx        context.Context
	streamMu          sync.Mutex
	streamCancel      context.CancelFunc
	probeEventStream  func(context.Context, *gatewayProxy) error
	manualGatewayMode bool
}

func NewAgentService() *AgentService {
	logger.Debug("creating agent service")
	return &AgentService{
		messages:          make([]MessageEvent, 0, 32),
		conversations:     make([]Conversation, 0, 8),
		auditEvents:       make([]AuditEvent, 0, 16),
		gateway:           loadGatewayProxy(),
		bootstrap:         newGatewayBootstrapper(),
		activeSessions:    make(map[string]struct{}),
		sessionAgents:     make(map[string]string),
		manualGatewayMode: detectWailsDevMode(),
		probeEventStream: func(ctx context.Context, proxy *gatewayProxy) error {
			return probeGatewayEventStream(ctx, proxy)
		},
	}
}

func (s *AgentService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	logger.Info("service startup begin")
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
	if s.manualGatewayMode {
		if s.gateway != nil {
			if err := s.pingGateway(ctx, s.gateway); err != nil {
				logger.Warn("dev mode gateway probe failed, require manual start", "error", err)
				s.gateway = nil
			}
		}
		if s.gateway == nil {
			s.emitGatewayStatus(GatewayStatusFailed, "开发模式未自动启动网关，请手动启动网关", nil)
			logger.Info("service startup complete (manual gateway mode, gateway not started)")
			return nil
		}
	}
	if err := s.ensureGatewayRunning(ctx); err != nil {
		logger.Error("service startup failed to ensure gateway", "error", err)
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
	probeErr := s.waitGatewayStreamReady(ctx)
	if probeErr != nil {
		logger.Warn("gateway event stream is not ready at startup, continue in reconnecting state", "error", probeErr)
		s.emitGatewayStatus(GatewayStatusReconnecting, "网关已启动，事件流连接中", map[string]any{
			"error": probeErr.Error(),
		})
	} else {
		s.emitGatewayStatus(GatewayStatusReady, "网关连接已就绪", nil)
	}
	s.startGatewayEventStream(ctx)
	logger.Info("service startup complete")
	return nil
}

func detectWailsDevMode() bool {
	candidates := []string{
		strings.TrimSpace(os.Getenv("WAILS_DEV")),
		strings.TrimSpace(os.Getenv("WAILS_ENV")),
		strings.TrimSpace(os.Getenv("WAILS_MODE")),
	}
	for _, value := range candidates {
		lower := strings.ToLower(value)
		if lower == "1" || lower == "true" || lower == "dev" || lower == "development" {
			return true
		}
	}
	return false
}

func (s *AgentService) ServiceShutdown() error {
	logger.Info("service shutdown begin")
	s.stopGatewayEventStream()
	if s.bootstrap == nil {
		logger.Info("service shutdown complete")
		return nil
	}
	if err := s.bootstrap.StopManagedProcess(); err != nil {
		logger.Error("service shutdown failed to stop managed gateway process", "error", err)
		return err
	}
	logger.Info("service shutdown complete")
	return nil
}

func (s *AgentService) RestartGateway(ctx context.Context) (GatewayStatus, error) {
	logger.Info("gateway restart requested")
	s.emitGatewayStatus(GatewayStatusReconnecting, "正在重启网关服务", nil)
	s.stopGatewayEventStream()
	stopManagedErr := error(nil)
	if s.bootstrap != nil {
		if err := s.bootstrap.StopManagedProcess(); err != nil {
			stopManagedErr = err
			logger.Warn("gateway restart failed to stop managed process, continue to ensure running", "error", err)
		}
	}
	s.mu.Lock()
	s.gateway = nil
	s.mu.Unlock()
	if err := s.ensureGatewayRunning(ctx); err != nil {
		logger.Error("gateway restart failed to ensure running", "error", err)
		s.emitGatewayStatus(GatewayStatusFailed, "网关重启失败", map[string]any{
			"error": err.Error(),
		})
		return GatewayStatus{}, err
	}
	probeErr := s.waitGatewayStreamReady(ctx)
	if probeErr != nil {
		logger.Warn("gateway restart probe indicates event stream not ready yet", "error", probeErr)
		meta := map[string]any{
			"error": probeErr.Error(),
		}
		if stopManagedErr != nil {
			meta["warning"] = stopManagedErr.Error()
			s.emitGatewayStatus(GatewayStatusReconnecting, "网关已重启，事件流连接中（旧进程未强制结束）", meta)
		} else {
			s.emitGatewayStatus(GatewayStatusReconnecting, "网关已重启，事件流连接中", meta)
		}
	} else if stopManagedErr != nil {
		s.emitGatewayStatus(GatewayStatusReady, "网关重启完成（未强制结束旧进程）", map[string]any{
			"warning": stopManagedErr.Error(),
		})
	} else {
		s.emitGatewayStatus(GatewayStatusReady, "网关重启完成", nil)
	}
	baseCtx := s.serviceCtx
	if baseCtx == nil {
		baseCtx = ctx
	}
	if baseCtx != nil {
		s.startGatewayEventStream(baseCtx)
	}
	logger.Info("gateway restart complete")
	return s.GetGatewayStatus(ctx)
}

func (s *AgentService) StopGateway(ctx context.Context) (GatewayStatus, error) {
	logger.Info("gateway stop requested")
	s.emitGatewayStatus(GatewayStatusReconnecting, "正在关闭网关服务", nil)
	s.stopGatewayEventStream()
	if s.bootstrap != nil {
		if err := s.bootstrap.StopManagedProcess(); err != nil {
			logger.Error("gateway stop failed to stop managed process", "error", err)
			s.emitGatewayStatus(GatewayStatusFailed, "网关关闭失败", map[string]any{
				"error": err.Error(),
			})
			return GatewayStatus{}, &BridgeError{
				Code:      ErrorCodeGatewayBootstrap,
				Message:   fmt.Sprintf("stop managed gateway process failed: %v", err),
				Retryable: true,
			}
		}
	}
	s.mu.Lock()
	s.gateway = nil
	s.lastEventID = ""
	s.mu.Unlock()
	s.emitGatewayStatus(GatewayStatusFailed, "网关已关闭", nil)
	logger.Info("gateway stop complete")
	return s.GetGatewayStatus(ctx)
}

func (s *AgentService) ensureGatewayRunning(ctx context.Context) error {
	if s.gateway != nil {
		if err := s.pingGateway(ctx, s.gateway); err == nil {
			logger.Debug("gateway already healthy", "baseURL", s.gateway.baseURL)
			return nil
		}
		logger.Warn("gateway health check failed, rediscovering", "baseURL", s.gateway.baseURL)
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
	logger.Info("gateway ensured running", "baseURL", proxy.baseURL)
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

func (s *AgentService) waitGatewayStreamReady(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("agent service is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	proxy := s.currentGatewayProxy()
	if proxy == nil {
		return fmt.Errorf("gateway client is not configured")
	}
	probe := s.probeEventStream
	if probe == nil {
		probe = probeGatewayEventStream
	}
	var lastErr error
	for attempt := 1; attempt <= gatewayStreamProbeAttempts; attempt++ {
		probeCtx, cancel := context.WithTimeout(ctx, gatewayStreamProbeTimeout)
		err := probe(probeCtx, proxy)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		logger.Warn("gateway stream probe failed", "attempt", attempt, "error", err)
		if attempt == gatewayStreamProbeAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(gatewayStreamProbeBackoff):
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("gateway stream probe failed")
	}
	return lastErr
}

func (s *AgentService) currentGatewayProxy() *gatewayProxy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gateway
}

func probeGatewayEventStream(ctx context.Context, proxy *gatewayProxy) error {
	if proxy == nil {
		return fmt.Errorf("gateway proxy is nil")
	}
	return gatewayclient.New(proxy.baseURL, proxy.token).ProbeEvents(ctx)
}

func (s *AgentService) NewSession(ctx context.Context, req NewSessionRequest) (Conversation, error) {
	agentID, err := s.resolveGatewayAgentID(ctx, resolveAgentIDFromMode(req.Mode))
	if err != nil {
		return Conversation{}, err
	}
	if err := s.startGatewayAgent(ctx, agentID); err != nil {
		return Conversation{}, err
	}
	var out gatewayACPSessionDTO
	if err := s.gatewayJSON(ctx, http.MethodPost, "/v1/agents/"+url.PathEscape(agentID)+"/sessions", mapCreateSessionRequest(req), &out); err != nil {
		return Conversation{}, err
	}
	conversation := mapACPSessionToConversation(out, req, agentID)
	s.rememberConversation(conversation, agentID)
	s.setCurrentStreamSessionID(conversation.ID)
	return conversation, nil
}

func (s *AgentService) ConnectSession(ctx context.Context, req ConnectSessionRequest) (Conversation, error) {
	req.SessionID = strings.TrimSpace(req.SessionID)
	if req.SessionID == "" {
		return Conversation{}, &BridgeError{Code: ErrorCodeInvalidArgument, Message: "sessionId is required", Retryable: false}
	}
	conversation, ok := s.conversationByID(req.SessionID)
	if !ok {
		conversation = Conversation{ID: req.SessionID, Type: "main", Title: req.SessionID, Status: "connected", CWD: strings.TrimSpace(req.Cwd), UpdatedAt: time.Now()}
	}
	conversation.Status = "connected"
	if cwd := strings.TrimSpace(req.Cwd); cwd != "" {
		conversation.CWD = cwd
	}
	s.rememberConversation(conversation, s.sessionAgentID(req.SessionID))
	s.setCurrentStreamSessionID(req.SessionID)
	return conversation, nil
}

func (s *AgentService) DisconnectSession(ctx context.Context, sessionID string) (Conversation, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Conversation{}, &BridgeError{Code: ErrorCodeInvalidArgument, Message: "sessionId is required", Retryable: false}
	}
	agentID, err := s.resolveSessionAgentID(ctx, sessionID, "")
	if err != nil {
		return Conversation{}, err
	}
	var out gatewayCloseSessionDTO
	if err := s.gatewayJSON(ctx, http.MethodDelete, "/v1/agents/"+url.PathEscape(agentID)+"/sessions/"+url.PathEscape(sessionID), nil, &out); err != nil {
		return Conversation{}, err
	}
	conversation, ok := s.conversationByID(sessionID)
	if !ok {
		conversation = Conversation{ID: sessionID, Type: "main", Title: sessionID}
	}
	conversation.Status = "disconnected"
	conversation.UpdatedAt = time.Now()
	s.rememberConversation(conversation, agentID)
	s.setCurrentStreamSessionID(sessionID)
	return conversation, nil
}

func (s *AgentService) DeleteSession(ctx context.Context, sessionID string) (Conversation, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Conversation{}, &BridgeError{Code: ErrorCodeInvalidArgument, Message: "sessionId is required", Retryable: false}
	}
	conversation, _ := s.DisconnectSession(ctx, sessionID)
	s.forgetConversation(sessionID)
	return conversation, nil
}

func (s *AgentService) LoadSession(ctx context.Context, sessionID string) (Conversation, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Conversation{}, &BridgeError{Code: ErrorCodeInvalidArgument, Message: "sessionId is required", Retryable: false}
	}
	conversation, ok := s.conversationByID(sessionID)
	if !ok {
		return Conversation{}, &BridgeError{Code: ErrorCodeGatewayRequest, Message: "session not found", Retryable: false}
	}
	s.setCurrentStreamSessionID(sessionID)
	return conversation, nil
}

func (s *AgentService) ListConversations(ctx context.Context) ([]Conversation, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Conversation(nil), s.conversations...), nil
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
	agentID, err := s.resolveSessionAgentID(ctx, req.SessionID, resolveAgentIDFromMode(req.Mode))
	if err != nil {
		return nil, err
	}
	var out gatewayACPPromptResponse
	if err := s.gatewayJSON(ctx, http.MethodPost, "/v1/agents/"+url.PathEscape(agentID)+"/sessions/"+url.PathEscape(req.SessionID)+"/prompts", gatewayACPPromptRequest{Text: req.Content}, &out); err != nil {
		return nil, err
	}
	events := []MessageEvent{
		{
			ID:        newBridgeEventID("msg"),
			SessionID: req.SessionID,
			Kind:      BridgeEventKindMessage,
			Role:      "user",
			Content:   req.Content,
			CreatedAt: time.Now(),
		},
		{
			ID:        newBridgeEventID("run"),
			SessionID: req.SessionID,
			Kind:      BridgeEventKindRun,
			Status:    firstNonEmpty(out.StopReason, "completed"),
			Summary:   "Prompt submitted to ACP agent",
			CreatedAt: time.Now(),
		},
	}
	s.appendLocalMessages(events)
	s.setCurrentStreamSessionID(req.SessionID)
	return events, nil
}

func (s *AgentService) Cancel(ctx context.Context, sessionID string) (RunSummary, error) {
	agentID, err := s.resolveSessionAgentID(ctx, sessionID, "")
	if err != nil {
		return RunSummary{}, err
	}
	var out map[string]any
	if err := s.gatewayJSON(ctx, http.MethodPost, "/v1/agents/"+url.PathEscape(agentID)+"/sessions/"+url.PathEscape(sessionID)+"/cancel", nil, &out); err != nil {
		return RunSummary{}, err
	}
	s.setCurrentStreamSessionID(sessionID)
	now := time.Now()
	return RunSummary{ID: newBridgeEventID("run"), SessionID: sessionID, Status: "cancelled", Label: "Cancelled", StartedAt: now, CompletedAt: &now}, nil
}

func (s *AgentService) ListMessages(ctx context.Context, sessionID string) ([]MessageEvent, error) {
	_ = ctx
	s.mu.RLock()
	filtered := make([]MessageEvent, 0)
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
	_ = ctx
	return []RunSummary{}, nil
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
		var bridgeErr *BridgeError
		if errors.As(err, &bridgeErr) && bridgeErr.StatusCode == http.StatusNotFound {
			// Compatible with older gateway versions that do not expose /v1/skills yet.
			return []SkillInfo{}, nil
		}
		return nil, err
	}
	return mapGatewaySkillsToInfos(out), nil
}

func (s *AgentService) ListAgents(ctx context.Context) ([]AgentProfile, error) {
	var out []gatewayAgentDTO
	if err := s.gatewayJSON(ctx, http.MethodGet, "/v1/agents", nil, &out); err != nil {
		return nil, err
	}
	return mapGatewayAgentsToProfiles(out), nil
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
	logger.Info("gateway event stream started")
	backoff := time.Second
	failures := 0
	authFailures := 0
	for {
		if ctx.Err() != nil {
			return
		}
		proxy := s.currentGatewayProxy()
		if proxy == nil {
			recoverCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			recoverErr := s.ensureGatewayRunning(recoverCtx)
			cancel()
			if recoverErr != nil {
				s.emitGatewayStatus(GatewayStatusFailed, "网关连接丢失", map[string]any{
					"error": recoverErr.Error(),
				})
				return
			}
			proxy = s.currentGatewayProxy()
		}
		if proxy == nil {
			s.emitGatewayStatus(GatewayStatusFailed, "网关代理不可用", nil)
			return
		}
		client := gatewayclient.New(proxy.baseURL, proxy.token)
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
			if authFailures > 0 {
				authFailures = 0
			}
			return s.forwardGatewayEvent(event)
		})
		if ctx.Err() != nil {
			return
		}
		bridgeErr := s.mapGatewayStreamError(err)
		if bridgeErr != nil && bridgeErr.Code == ErrorCodeGatewayAuthFailed {
			authFailures++
			logger.Warn("gateway event stream auth failed, try refresh gateway proxy", "error", bridgeErr.Message, "status", bridgeErr.StatusCode, "authFailures", authFailures)
			recoverCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			recoverErr := s.refreshGatewayProxy(recoverCtx)
			cancel()
			if recoverErr != nil {
				logger.Error("gateway event stream auth failed after refresh", "error", bridgeErr.Message, "status", bridgeErr.StatusCode, "refreshError", recoverErr)
				s.emitGatewayStatus(GatewayStatusFailed, "网关鉴权失败", map[string]any{
					"code":   bridgeErr.Code,
					"status": bridgeErr.StatusCode,
					"error":  bridgeErr.Message,
				})
				return
			}

			if authFailures >= maxGatewayAuthFailures {
				logger.Warn("gateway auth failed repeatedly, try hard restart recovery", "authFailures", authFailures)
				restartCtx, restartCancel := context.WithTimeout(context.Background(), 10*time.Second)
				restartErr := s.restartGatewayForAuthRecovery(restartCtx)
				restartCancel()
				if restartErr != nil {
					logger.Error("gateway hard restart recovery failed after auth errors", "error", restartErr)
					s.emitGatewayStatus(GatewayStatusFailed, "网关鉴权失败", map[string]any{
						"code":         bridgeErr.Code,
						"status":       bridgeErr.StatusCode,
						"error":        bridgeErr.Message,
						"recoverError": restartErr.Error(),
					})
					return
				}
				authFailures = 0
			}

			failures++
			if failures > 0 {
				s.emitGatewayStatus(GatewayStatusReconnecting, "网关事件流重连中", map[string]any{
					"attempt":      failures + 1,
					"authFailures": authFailures,
				})
			}
			if !waitGatewayReconnectBackoff(ctx, &backoff) {
				return
			}
			continue
		}
		if bridgeErr != nil {
			failures++
			logger.Warn("gateway event stream failed", "attempt", failures, "error", bridgeErr.Message, "status", bridgeErr.StatusCode)
			if failures >= maxGatewayStreamFailures {
				logger.Error("gateway event stream giving up after retries", "failures", failures, "error", bridgeErr.Message)
				s.emitGatewayStatus(GatewayStatusFailed, "网关事件流重连失败", map[string]any{
					"code":     bridgeErr.Code,
					"status":   bridgeErr.StatusCode,
					"error":    bridgeErr.Message,
					"failures": failures,
				})
				return
			}
		}
		if !waitGatewayReconnectBackoff(ctx, &backoff) {
			return
		}
	}
}

func waitGatewayReconnectBackoff(ctx context.Context, backoff *time.Duration) bool {
	if backoff == nil {
		return true
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(*backoff):
	}
	*backoff = nextGatewayReconnectBackoff(*backoff)
	return true
}

func nextGatewayReconnectBackoff(current time.Duration) time.Duration {
	if current < 10*time.Second {
		current *= 2
		if current > 10*time.Second {
			current = 10 * time.Second
		}
	}
	return current
}

func (s *AgentService) refreshGatewayProxy(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("agent service is nil")
	}
	if s.bootstrap == nil {
		return fmt.Errorf("gateway bootstrap is not configured")
	}
	proxy, _, err := s.bootstrap.discoverHealthy(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.gateway = proxy
	s.mu.Unlock()
	return nil
}

func (s *AgentService) restartGatewayForAuthRecovery(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("agent service is nil")
	}
	if s.bootstrap == nil {
		return fmt.Errorf("gateway bootstrap is not configured")
	}
	if err := s.bootstrap.StopManagedProcess(); err != nil {
		logger.Warn("stop managed gateway process during auth recovery failed", "error", err)
	}
	s.mu.Lock()
	s.gateway = nil
	s.mu.Unlock()
	if err := s.ensureGatewayRunning(ctx); err != nil {
		return err
	}
	return s.waitGatewayStreamReady(ctx)
}

func (s *AgentService) startGatewayEventStream(baseCtx context.Context) {
	if baseCtx == nil {
		logger.Warn("skip starting gateway event stream: base context is nil")
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
	logger.Debug("starting gateway event stream goroutine")
	go s.streamGatewayEvents(streamCtx)
}

func (s *AgentService) stopGatewayEventStream() {
	s.streamMu.Lock()
	cancel := s.streamCancel
	s.streamCancel = nil
	s.streamMu.Unlock()
	if cancel != nil {
		logger.Debug("stopping gateway event stream")
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

func (s *AgentService) rememberConversation(conversation Conversation, agentID string) {
	if strings.TrimSpace(conversation.ID) == "" {
		return
	}
	if conversation.UpdatedAt.IsZero() {
		conversation.UpdatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, item := range s.conversations {
		if item.ID == conversation.ID {
			idx = i
			break
		}
	}
	if idx >= 0 {
		s.conversations[idx] = conversation
	} else {
		s.conversations = append([]Conversation{conversation}, s.conversations...)
	}
	if s.activeSessions == nil {
		s.activeSessions = make(map[string]struct{})
	}
	s.activeSessions[conversation.ID] = struct{}{}
	if s.sessionAgents == nil {
		s.sessionAgents = make(map[string]string)
	}
	if agentID = strings.TrimSpace(agentID); agentID != "" {
		s.sessionAgents[conversation.ID] = agentID
	}
}

func (s *AgentService) forgetConversation(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.conversations[:0]
	for _, item := range s.conversations {
		if item.ID != sessionID {
			out = append(out, item)
		}
	}
	s.conversations = out
	delete(s.activeSessions, sessionID)
	delete(s.sessionAgents, sessionID)
	if s.currentSessionID == sessionID {
		s.currentSessionID = ""
	}
}

func (s *AgentService) conversationByID(sessionID string) (Conversation, bool) {
	sessionID = strings.TrimSpace(sessionID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.conversations {
		if item.ID == sessionID {
			return item, true
		}
	}
	return Conversation{}, false
}

func (s *AgentService) sessionAgentID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return strings.TrimSpace(s.sessionAgents[sessionID])
}

func (s *AgentService) appendLocalMessages(items []MessageEvent) {
	if len(items) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, items...)
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

func (s *AgentService) resolveSessionAgentID(ctx context.Context, sessionID string, preferred string) (string, error) {
	if agentID := strings.TrimSpace(preferred); agentID != "" {
		if sessionID = strings.TrimSpace(sessionID); sessionID != "" {
			s.mu.Lock()
			if s.sessionAgents == nil {
				s.sessionAgents = make(map[string]string)
			}
			s.sessionAgents[sessionID] = agentID
			s.mu.Unlock()
		}
		return agentID, nil
	}
	if agentID := s.sessionAgentID(sessionID); agentID != "" {
		return agentID, nil
	}
	return s.resolveGatewayAgentID(ctx, "")
}

func (s *AgentService) resolveGatewayAgentID(ctx context.Context, preferred string) (string, error) {
	if agentID := strings.TrimSpace(preferred); agentID != "" {
		return agentID, nil
	}
	var agents []gatewayAgentDTO
	if err := s.gatewayJSON(ctx, http.MethodGet, "/v1/agents", nil, &agents); err != nil {
		return "", err
	}
	for _, agent := range agents {
		if agent.Enabled {
			return strings.TrimSpace(agent.ID), nil
		}
	}
	if len(agents) == 0 {
		return "", &BridgeError{Code: ErrorCodeGatewayRequest, Message: "gateway has no enabled ACP agent", Retryable: false}
	}
	return "", &BridgeError{Code: ErrorCodeGatewayRequest, Message: "gateway has no enabled ACP agent", Retryable: false}
}

func (s *AgentService) startGatewayAgent(ctx context.Context, agentID string) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return &BridgeError{Code: ErrorCodeInvalidArgument, Message: "agentId is required", Retryable: false}
	}
	var out map[string]any
	return s.gatewayJSON(ctx, http.MethodPost, "/v1/agents/"+url.PathEscape(agentID)+"/start", nil, &out)
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
		audit := mapMessageToAuditEvent(envelope, out)
		s.auditEvents = append(s.auditEvents, audit)
		if overflow := len(s.auditEvents) - maxAuditEventCacheSize; overflow > 0 {
			s.auditEvents = append([]AuditEvent(nil), s.auditEvents[overflow:]...)
		}
	}
	s.mu.Unlock()
	s.setLastStreamEventID(envelope.ID)
	if s.eventSink != nil {
		s.eventSink(out)
	}
	return nil
}

func mapMessageToAuditEvent(envelope GatewayEventEnvelope, message MessageEvent) AuditEvent {
	auditType := strings.TrimSpace(envelope.Type)
	if auditType == "" {
		auditType = "audit"
	}
	level := extractAuditLevel(message)
	summary := strings.TrimSpace(message.Summary)
	if summary == "" {
		summary = strings.TrimSpace(message.Content)
	}
	if summary == "" {
		summary = "审计事件"
	}
	return AuditEvent{
		ID:        strings.TrimSpace(message.ID),
		SessionID: strings.TrimSpace(message.SessionID),
		Type:      auditType,
		Level:     level,
		Summary:   summary,
		CreatedAt: message.CreatedAt,
	}
}

func extractAuditLevel(message MessageEvent) string {
	candidates := []string{
		strings.TrimSpace(message.Status),
		anyString(message.SafeMeta["level"]),
		anyString(message.SafeMeta["severity"]),
	}
	for _, raw := range candidates {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "debug", "info", "warn", "warning", "error":
			if strings.EqualFold(raw, "warning") {
				return "warn"
			}
			return strings.ToLower(strings.TrimSpace(raw))
		}
	}
	return "info"
}

func anyString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
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

type gatewayACPSessionDTO struct {
	SessionID string `json:"sessionId"`
}

type gatewayCloseSessionDTO struct {
	SessionID string `json:"sessionId,omitempty"`
}

type gatewayACPNewSessionRequest struct {
	CWD                   string   `json:"cwd"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
	MCPServers            []any    `json:"mcpServers"`
}

type gatewayACPPromptRequest struct {
	Text string `json:"text"`
}

type gatewayACPPromptResponse struct {
	StopReason string `json:"stopReason"`
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

type gatewayAgentDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Protocol    string   `json:"protocol"`
	Models      []string `json:"models,omitempty"`
	ModelsJSON  string   `json:"modelsJson,omitempty"`
	Description string   `json:"description,omitempty"`
	Enabled     bool     `json:"enabled"`
}

func mapCreateSessionRequest(in NewSessionRequest) gatewayACPNewSessionRequest {
	return gatewayACPNewSessionRequest{
		CWD:                   strings.TrimSpace(in.Cwd),
		AdditionalDirectories: []string{},
		MCPServers:            []any{},
	}
}

func resolveAgentIDFromMode(mode string) string {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return ""
	}
	switch strings.ToLower(mode) {
	case "agent", "default", "main":
		return ""
	default:
		return mode
	}
}

func mapApprovalDecisionRequest(in ApprovalDecisionRequest) gatewayApprovalDecisionRequest {
	return gatewayApprovalDecisionRequest{
		Decision: strings.TrimSpace(in.Decision),
	}
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

func mapGatewayAgentsToProfiles(in []gatewayAgentDTO) []AgentProfile {
	out := make([]AgentProfile, 0, len(in))
	for _, item := range in {
		models := append([]string(nil), item.Models...)
		if len(models) == 0 && strings.TrimSpace(item.ModelsJSON) != "" {
			_ = json.Unmarshal([]byte(item.ModelsJSON), &models)
		}
		out = append(out, AgentProfile{
			ID:          strings.TrimSpace(item.ID),
			Name:        strings.TrimSpace(item.Name),
			Protocol:    strings.TrimSpace(item.Protocol),
			Models:      models,
			Description: strings.TrimSpace(item.Description),
		})
	}
	return out
}

func mapACPSessionToConversation(in gatewayACPSessionDTO, req NewSessionRequest, agentID string) Conversation {
	now := time.Now()
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = stringOrFallback(in.SessionID, "ACP Session")
	}
	return Conversation{
		ID:          strings.TrimSpace(in.SessionID),
		Type:        "main",
		Title:       title,
		Subtitle:    strings.TrimSpace(agentID),
		Status:      "connected",
		UpdatedAt:   now,
		WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		CWD:         strings.TrimSpace(req.Cwd),
		Mode:        strings.TrimSpace(req.Mode),
		Model:       strings.TrimSpace(req.Model),
	}
}

func stringOrFallback(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}

func newBridgeEventID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

type gatewayProxy struct {
	client  *http.Client
	baseURL string
	token   string
}

func loadGatewayProxy() *gatewayProxy {
	endpoint, token, err := gatewayclient.DiscoverFromPath("")
	if err != nil {
		return nil
	}
	if settings, settingsErr := loadAppSettings(); settingsErr == nil {
		if configured := strings.TrimSpace(settings.GatewayToken); configured != "" && strings.TrimSpace(token) == "" {
			token = configured
		}
	}
	return &gatewayProxy{
		client:  http.DefaultClient,
		baseURL: strings.TrimRight(endpoint.BaseURL, "/"),
		token:   strings.TrimSpace(token),
	}
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
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	if err != nil {
		return &BridgeError{Code: ErrorCodeGatewayRequest, Message: "read gateway response failed", Retryable: false}
	}
	if err := decodeGatewayResponse(raw, out); err != nil {
		return &BridgeError{Code: ErrorCodeGatewayRequest, Message: "decode gateway response failed", Retryable: false}
	}
	return nil
}

func decodeGatewayResponse(raw []byte, out any) error {
	var wrapped struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && strings.TrimSpace(wrapped.Code) != "" {
		if wrapped.Code != "ok" {
			return fmt.Errorf("%s", firstNonEmpty(wrapped.Message, wrapped.Code))
		}
		if len(wrapped.Data) == 0 || string(wrapped.Data) == "null" {
			return nil
		}
		return decodeGatewayPayload(wrapped.Data, out)
	}
	return decodeGatewayPayload(raw, out)
}

func decodeGatewayPayload(raw []byte, out any) error {
	if err := json.Unmarshal(raw, out); err == nil {
		return nil
	}
	var page struct {
		Items json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &page); err != nil || len(page.Items) == 0 {
		return json.Unmarshal(raw, out)
	}
	return json.Unmarshal(page.Items, out)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
