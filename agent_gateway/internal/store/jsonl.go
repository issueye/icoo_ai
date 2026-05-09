package store

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	conversationsFile = "conversations.jsonl"
	messagesFile      = "messages.jsonl"
	runsFile          = "runs.jsonl"
	approvalsFile     = "approvals.jsonl"
	auditFile         = "audit.jsonl"
)

type LoadIssue struct {
	File string
	Line int
	Err  string
}

type JSONLStore struct {
	mem *MemoryStore

	mu      sync.Mutex
	baseDir string
	issues  []LoadIssue
}

func NewJSONLStore(ctx context.Context, cfg JSONLConfig) (*JSONLStore, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.Dir) == "" {
		return nil, ErrInvalidConfig
	}
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: create jsonl dir: %w", err)
	}

	s := &JSONLStore{
		mem:     NewMemoryStore(),
		baseDir: cfg.Dir,
	}
	if err := s.load(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *JSONLStore) LoadIssues() []LoadIssue {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]LoadIssue, len(s.issues))
	copy(out, s.issues)
	return out
}

func (s *JSONLStore) load(ctx context.Context) error {
	if err := s.loadConversations(ctx); err != nil {
		return err
	}
	if err := s.loadMessages(ctx); err != nil {
		return err
	}
	if err := s.loadRuns(ctx); err != nil {
		return err
	}
	if err := s.loadApprovals(ctx); err != nil {
		return err
	}
	if err := s.loadAudit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *JSONLStore) loadConversations(ctx context.Context) error {
	path := filepath.Join(s.baseDir, conversationsFile)
	return decodeJSONLLines(path, func(raw []byte, line int) error {
		var item Conversation
		if err := json.Unmarshal(raw, &item); err != nil {
			s.appendIssue(path, line, err)
			return nil
		}
		if err := s.mem.UpsertConversation(ctx, item); err != nil {
			return err
		}
		return nil
	})
}

func (s *JSONLStore) loadMessages(ctx context.Context) error {
	path := filepath.Join(s.baseDir, messagesFile)
	return decodeJSONLLines(path, func(raw []byte, line int) error {
		var item MessageEvent
		if err := json.Unmarshal(raw, &item); err != nil {
			s.appendIssue(path, line, err)
			return nil
		}
		if err := s.mem.AppendMessage(ctx, item); err != nil {
			return err
		}
		return nil
	})
}

func (s *JSONLStore) loadRuns(ctx context.Context) error {
	path := filepath.Join(s.baseDir, runsFile)
	return decodeJSONLLines(path, func(raw []byte, line int) error {
		var item RunSummary
		if err := json.Unmarshal(raw, &item); err != nil {
			s.appendIssue(path, line, err)
			return nil
		}
		if err := s.mem.UpsertRun(ctx, item); err != nil {
			return err
		}
		return nil
	})
}

func (s *JSONLStore) loadApprovals(ctx context.Context) error {
	path := filepath.Join(s.baseDir, approvalsFile)
	return decodeJSONLLines(path, func(raw []byte, line int) error {
		var item ApprovalDecision
		if err := json.Unmarshal(raw, &item); err != nil {
			s.appendIssue(path, line, err)
			return nil
		}
		if err := s.mem.UpsertApproval(ctx, item); err != nil {
			return err
		}
		return nil
	})
}

func (s *JSONLStore) loadAudit(ctx context.Context) error {
	path := filepath.Join(s.baseDir, auditFile)
	return decodeJSONLLines(path, func(raw []byte, line int) error {
		var item AuditEvent
		if err := json.Unmarshal(raw, &item); err != nil {
			s.appendIssue(path, line, err)
			return nil
		}
		if err := s.mem.AppendAudit(ctx, item); err != nil {
			return err
		}
		return nil
	})
}

func (s *JSONLStore) appendIssue(path string, line int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.issues = append(s.issues, LoadIssue{
		File: filepath.Base(path),
		Line: line,
		Err:  err.Error(),
	})
}

func decodeJSONLLines(path string, handle func(raw []byte, line int) error) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("store: open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" {
			continue
		}
		if err := handle([]byte(trimmed), line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("store: scan %s: %w", path, err)
	}
	return nil
}

func appendJSONL(ctx context.Context, path string, item any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("store: open %s: %w", path, err)
	}
	defer file.Close()

	if err := ctx.Err(); err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(item); err != nil {
		return fmt.Errorf("store: append %s: %w", path, err)
	}
	return nil
}

func (s *JSONLStore) UpsertConversation(ctx context.Context, conversation Conversation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	item := cloneConversation(conversation)
	if err := s.mem.UpsertConversation(ctx, item); err != nil {
		return err
	}
	return appendJSONL(ctx, filepath.Join(s.baseDir, conversationsFile), item)
}

func (s *JSONLStore) ListConversations(ctx context.Context) ([]Conversation, error) {
	return s.mem.ListConversations(ctx)
}

func (s *JSONLStore) GetConversation(ctx context.Context, sessionID string) (Conversation, bool, error) {
	return s.mem.GetConversation(ctx, sessionID)
}

func (s *JSONLStore) AppendMessage(ctx context.Context, event MessageEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	item := cloneMessageEvent(event)
	if err := s.mem.AppendMessage(ctx, item); err != nil {
		return err
	}
	return appendJSONL(ctx, filepath.Join(s.baseDir, messagesFile), item)
}

func (s *JSONLStore) ListMessages(ctx context.Context, sessionID string) ([]MessageEvent, error) {
	return s.mem.ListMessages(ctx, sessionID)
}

func (s *JSONLStore) UpsertRun(ctx context.Context, run RunSummary) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	item := cloneRunSummary(run)
	if err := s.mem.UpsertRun(ctx, item); err != nil {
		return err
	}
	return appendJSONL(ctx, filepath.Join(s.baseDir, runsFile), item)
}

func (s *JSONLStore) ListRuns(ctx context.Context, sessionID string) ([]RunSummary, error) {
	return s.mem.ListRuns(ctx, sessionID)
}

func (s *JSONLStore) UpsertApproval(ctx context.Context, approval ApprovalDecision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	item := cloneApprovalDecision(approval)
	if err := s.mem.UpsertApproval(ctx, item); err != nil {
		return err
	}
	return appendJSONL(ctx, filepath.Join(s.baseDir, approvalsFile), item)
}

func (s *JSONLStore) ListApprovals(ctx context.Context) ([]ApprovalDecision, error) {
	return s.mem.ListApprovals(ctx)
}

func (s *JSONLStore) AppendAudit(ctx context.Context, event AuditEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	item := cloneAuditEvent(event)
	if err := s.mem.AppendAudit(ctx, item); err != nil {
		return err
	}
	return appendJSONL(ctx, filepath.Join(s.baseDir, auditFile), item)
}

func (s *JSONLStore) ListAuditEvents(ctx context.Context) ([]AuditEvent, error) {
	return s.mem.ListAuditEvents(ctx)
}
