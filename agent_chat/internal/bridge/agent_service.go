package bridge

import (
	"context"
	"time"
)

type AgentService struct {
	now time.Time
}

func NewAgentService() *AgentService {
	return &AgentService{now: time.Date(2026, 5, 9, 10, 30, 0, 0, time.FixedZone("CST", 8*60*60))}
}

func (s *AgentService) ListConversations(ctx context.Context) ([]Conversation, error) {
	return []Conversation{
		{ID: "sess_main_20260509_001", Type: "main", Title: "session 事件持久化", Subtitle: "记录必要元信息，避免敏感大输出落盘", Status: "waiting_approval", UnreadCount: 2, UpdatedAt: s.now},
		{ID: "subsess_review_20260509_001", Type: "subagent", Title: "Review Worker", Subtitle: "独立 subagent 会话", Status: "completed", UpdatedAt: s.now, ParentSessionID: "sess_main_20260509_001"},
	}, nil
}

func (s *AgentService) ListMessages(ctx context.Context, sessionID string) ([]MessageEvent, error) {
	items := []MessageEvent{
		{ID: "msg_1", SessionID: "sess_main_20260509_001", Kind: "message", Role: "user", Content: "为 session 增加最小可用的事件/运行摘要持久化能力。", CreatedAt: s.now},
		{ID: "tool_1", SessionID: "sess_main_20260509_001", Kind: "tool_call", ToolName: "shell", Status: "completed", DurationMs: 86, Summary: "仅读取文件摘要", SafeMeta: map[string]any{"outputBytes": 18432, "outputHash": "sha256:4b7c...91af", "persistedOutput": false}, CreatedAt: s.now},
		{ID: "sub_1", SessionID: "sess_main_20260509_001", Kind: "subagent_run", SubSessionID: "subsess_review_20260509_001", ParentSessionID: "sess_main_20260509_001", Task: "安全审查", Status: "completed", Summary: "未发现敏感大输出落盘。", EventCount: 12, CreatedAt: s.now},
		{ID: "msg_sub_1", SessionID: "subsess_review_20260509_001", Kind: "message", Role: "assistant", Content: "subagent 使用独立会话 ID。", CreatedAt: s.now},
	}
	filtered := make([]MessageEvent, 0, len(items))
	for _, item := range items {
		if item.SessionID == sessionID {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s *AgentService) ListRuns(ctx context.Context) ([]RunSummary, error) {
	completed := s.now
	return []RunSummary{{ID: "run_sub_1", SessionID: "subsess_review_20260509_001", ParentSessionID: "sess_main_20260509_001", Status: "completed", Label: "安全审查完成", StartedAt: s.now, CompletedAt: &completed}}, nil
}

func (s *AgentService) ListApprovals(ctx context.Context) ([]ApprovalDecision, error) {
	return []ApprovalDecision{{ID: "approval_1", SessionID: "sess_main_20260509_001", Decision: "approved", Actor: "user", Summary: "允许保存摘要元信息", CreatedAt: s.now}}, nil
}

func (s *AgentService) ListSkills(ctx context.Context) ([]SkillInfo, error) {
	return []SkillInfo{{ID: "security-auditor", Name: "security-auditor", Description: "审查敏感输出与权限边界。"}}, nil
}

func (s *AgentService) ListAuditEvents(ctx context.Context) ([]AuditEvent, error) {
	return []AuditEvent{{ID: "audit_1", SessionID: "sess_main_20260509_001", Type: "tool_result_summary", Level: "info", Summary: "只保存 outputBytes/outputHash/persistedOutput=false。", CreatedAt: s.now}}, nil
}
