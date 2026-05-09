package bridge

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type AgentService struct {
	now           time.Time
	conversations []Conversation
	messages      []MessageEvent
	runs          []RunSummary
	approvals     []ApprovalDecision
	auditEvents   []AuditEvent
}

func NewAgentService() *AgentService {
	now := time.Date(2026, 5, 9, 10, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	completed := now
	return &AgentService{
		now: now,
		conversations: []Conversation{
			{ID: "sess_main_20260509_001", Type: "main", Title: "session 事件持久化", Subtitle: "记录 tool call / result / approval 摘要", Status: "waiting_approval", UnreadCount: 2, UpdatedAt: now, Model: "gpt-5.4"},
			{ID: "subsess_review_20260509_001", Type: "subagent", Title: "Review Worker", Subtitle: "检查持久化边界与敏感输出策略", Status: "completed", UpdatedAt: now, ParentSessionID: "sess_main_20260509_001", Skill: "security-auditor"},
			{ID: "sess_ui_20260509_002", Type: "main", Title: "桌面聊天 UI", Subtitle: "Wails + Vue + Pinia 初版骨架", Status: "running", UpdatedAt: now, Model: "gpt-5.4"},
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
	}
}

func (s *AgentService) NewSession(ctx context.Context, req NewSessionRequest) (Conversation, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "新的 Agent 会话"
	}
	sessionID := fmt.Sprintf("sess_mock_%d", len(s.conversations)+1)
	conversation := Conversation{
		ID:        sessionID,
		Type:      "main",
		Title:     title,
		Subtitle:  "已创建 mock 会话，等待输入",
		Status:    "idle",
		UpdatedAt: s.now,
		Model:     "gpt-5.4",
	}
	s.conversations = append([]Conversation{conversation}, s.conversations...)
	return conversation, nil
}

func (s *AgentService) LoadSession(ctx context.Context, sessionID string) (Conversation, error) {
	for _, conversation := range s.conversations {
		if conversation.ID == sessionID {
			return conversation, nil
		}
	}
	return Conversation{}, fmt.Errorf("session %q not found", sessionID)
}

func (s *AgentService) ListConversations(ctx context.Context) ([]Conversation, error) {
	return append([]Conversation(nil), s.conversations...), nil
}

func (s *AgentService) Prompt(ctx context.Context, req PromptRequest) ([]MessageEvent, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
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
	s.touchConversation(sessionID, "运行已取消", "cancelled")
	run := RunSummary{ID: fmt.Sprintf("run_cancel_%d", len(s.runs)+1), SessionID: sessionID, Status: "cancelled", Label: "运行已取消", StartedAt: s.now, CompletedAt: &s.now}
	s.runs = append(s.runs, run)
	return run, nil
}

func (s *AgentService) ListMessages(ctx context.Context, sessionID string) ([]MessageEvent, error) {
	filtered := make([]MessageEvent, 0, len(s.messages))
	for _, item := range s.messages {
		if item.SessionID == sessionID {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s *AgentService) ListRuns(ctx context.Context) ([]RunSummary, error) {
	return append([]RunSummary(nil), s.runs...), nil
}

func (s *AgentService) ListApprovals(ctx context.Context) ([]ApprovalDecision, error) {
	return append([]ApprovalDecision(nil), s.approvals...), nil
}

func (s *AgentService) DecideApproval(ctx context.Context, req ApprovalDecisionRequest) (ApprovalDecision, error) {
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
