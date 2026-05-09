package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/audit"
	"github.com/icoo-ai/icoo-ai/internal/hooks"
	"github.com/icoo-ai/icoo-ai/internal/llm"
)

type SessionStore interface {
	Create(ctx context.Context, session Session) (Session, error)
	Get(ctx context.Context, id string) (Session, error)
	Update(ctx context.Context, session Session) error
	List(ctx context.Context) ([]Session, error)
}

type RuntimeOptions struct {
	Loop     Loop
	Store    SessionStore
	CWD      string
	Model    string
	Approver Approver
	Hooks    hooks.Dispatcher
	Audit    audit.Logger
}

type DefaultRuntime struct {
	loop     Loop
	store    SessionStore
	cwd      string
	model    string
	approver Approver
	hooks    hooks.Dispatcher
	audit    audit.Logger

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

func NewRuntime(opts RuntimeOptions) (*DefaultRuntime, error) {
	if opts.Loop == nil {
		return nil, errors.New("agent runtime requires loop")
	}
	if opts.Store == nil {
		return nil, errors.New("agent runtime requires session store")
	}
	return &DefaultRuntime{
		loop:     opts.Loop,
		store:    opts.Store,
		cwd:      opts.CWD,
		model:    opts.Model,
		approver: opts.Approver,
		hooks:    opts.Hooks,
		audit:    opts.Audit,
		cancels:  make(map[string]context.CancelFunc),
	}, nil
}

func (r *DefaultRuntime) NewSession(ctx context.Context, req NewSessionRequest) (Session, error) {
	cwd := req.CWD
	if cwd == "" {
		cwd = r.cwd
	}
	now := time.Now().UTC()
	return r.store.Create(ctx, Session{
		CWD:       cwd,
		Model:     r.model,
		Messages:  nil,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (r *DefaultRuntime) Prompt(ctx context.Context, req PromptRequest) (<-chan Event, error) {
	sess, err := r.store.Get(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}
	if req.CWD != "" {
		sess.CWD = req.CWD
	}
	sess.Messages = append(sess.Messages, llm.Message{Role: "user", Content: req.Prompt})
	if err := r.store.Update(ctx, sess); err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithCancel(ctx)
	r.setCancel(sess.ID, cancel)

	events, err := r.loop.Run(runCtx, RunRequest{
		SessionID: sess.ID,
		CWD:       sess.CWD,
		Messages:  sess.Messages,
		Context:   WorkspaceContext{Root: sess.CWD},
		Options: RunOptions{
			Model:       r.model,
			Approver:    r.approver,
			Hooks:       r.hooks,
			AuditLogger: r.audit,
		},
	})
	if err != nil {
		r.clearCancel(sess.ID)
		cancel()
		return nil, err
	}

	out := make(chan Event)
	go func() {
		defer close(out)
		defer r.clearCancel(sess.ID)
		defer cancel()

		var assistant strings.Builder
		for event := range events {
			out <- event
			if event.Type == EventMessageDelta && event.Content != "" {
				assistant.WriteString(event.Content)
				continue
			}
			appendSessionEvent(&sess, event)
			_ = r.store.Update(context.Background(), sess)
		}
		if assistant.Len() > 0 {
			sess.Messages = append(sess.Messages, llm.Message{Role: "assistant", Content: assistant.String()})
			_ = r.store.Update(context.Background(), sess)
		}
	}()

	return out, nil
}

func (r *DefaultRuntime) Cancel(ctx context.Context, sessionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	cancel := r.cancels[sessionID]
	r.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	return nil
}

func (r *DefaultRuntime) LoadSession(ctx context.Context, sessionID string) (Session, error) {
	return r.store.Get(ctx, sessionID)
}

func (r *DefaultRuntime) setCancel(sessionID string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cancels[sessionID] = cancel
}

func (r *DefaultRuntime) clearCancel(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cancels, sessionID)
}

const maxSessionEvents = 100
const maxSessionContent = 256

func appendSessionEvent(sess *Session, event Event) {
	data := summarizeEventData(event.Type, event.Data)
	if data == nil && event.Content == "" && event.Error == "" && !shouldPersistEmptyEvent(event.Type) {
		return
	}
	summary := SessionEventSummary{
		Type:      event.Type,
		Content:   summarizeEventContent(event.Type, event.Content),
		Error:     summarizeEventError(event.Type, event.Error),
		Data:      data,
		CreatedAt: event.CreatedAt.UTC(),
	}
	sess.Events = append(sess.Events, summary)
	if len(sess.Events) > maxSessionEvents {
		sess.Events = append([]SessionEventSummary(nil), sess.Events[len(sess.Events)-maxSessionEvents:]...)
	}
}

func summarizeEventData(typ EventType, data map[string]any) map[string]any {
	if len(data) == 0 {
		return nil
	}
	switch typ {
	case EventToolCallStarted:
		return copyKeys(data, "id", "name")
	case EventToolCallCompleted:
		out := copyKeys(data, "id", "name", "ok")
		if result, ok := data["result"].(map[string]any); ok {
			if trimmed := copyKeys(result, "code", "path", "bytes", "status_code", "content_type", "retry_attempts"); len(trimmed) > 0 {
				out["result"] = trimmed
			}
		}
		return nilIfEmpty(out)
	case EventApprovalRequested:
		out := copyKeys(data, "id", "name", "reason")
		if raw, ok := data["data"].(map[string]any); ok {
			if trimmed := copyKeys(raw, "code", "risk", "action"); len(trimmed) > 0 {
				out["data"] = trimmed
			}
		}
		return nilIfEmpty(out)
	case EventApprovalDecided:
		return copyKeys(data, "id", "name", "decision")
	case EventRunStarted, EventRunCompleted, EventRunFailed, EventRunCancelled:
		return copyKeys(data, "id", "name", "ok", "reason")
	default:
		return nil
	}
}

func trimSummaryText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxSessionContent {
		return value
	}
	return value[:maxSessionContent] + "...(truncated)"
}

func summarizeEventContent(typ EventType, content string) string {
	switch typ {
	case EventApprovalRequested:
		return trimSummaryText(content)
	default:
		return ""
	}
}

func summarizeEventError(typ EventType, errText string) string {
	switch typ {
	case EventRunFailed, EventRunCancelled, EventApprovalDecided:
		return trimSummaryText(errText)
	default:
		return ""
	}
}

func shouldPersistEmptyEvent(typ EventType) bool {
	switch typ {
	case EventRunStarted, EventRunCompleted, EventRunFailed, EventRunCancelled:
		return true
	default:
		return false
	}
}

func copyKeys(data map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, key := range keys {
		if value, ok := data[key]; ok {
			out[key] = value
		}
	}
	return out
}

func nilIfEmpty(data map[string]any) map[string]any {
	if len(data) == 0 {
		return nil
	}
	return data
}
