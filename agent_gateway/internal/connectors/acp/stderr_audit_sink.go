package acp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

const (
	defaultStderrMaxSummaryBytes = 1024
	defaultStderrSampleEvery     = 10
)

type StderrAuditSinkOptions struct {
	Store           store.Store
	AgentID         string
	SessionID       string
	RunID           string
	MaxSummaryBytes int
	SampleEvery     int
	Now             func() time.Time
}

type stderrAuditSink struct {
	mu sync.Mutex

	store   store.Store
	agentID string
	session string
	runID   string

	maxSummaryBytes int
	sampleEvery     int
	now             func() time.Time

	seq       uint64
	lineIndex uint64
	buffer    string
}

func NewStderrAuditSink(opts StderrAuditSinkOptions) *stderrAuditSink {
	maxSummaryBytes := opts.MaxSummaryBytes
	if maxSummaryBytes <= 0 {
		maxSummaryBytes = defaultStderrMaxSummaryBytes
	}
	sampleEvery := opts.SampleEvery
	if sampleEvery <= 0 {
		sampleEvery = defaultStderrSampleEvery
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &stderrAuditSink{
		store:           opts.Store,
		agentID:         opts.AgentID,
		session:         opts.SessionID,
		runID:           opts.RunID,
		maxSummaryBytes: maxSummaryBytes,
		sampleEvery:     sampleEvery,
		now:             now,
	}
}

func (w *stderrAuditSink) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buffer += string(p)
	for {
		i := strings.IndexByte(w.buffer, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(w.buffer[:i], "\r")
		w.buffer = w.buffer[i+1:]
		w.consumeLineLocked(line)
	}
	return len(p), nil
}

func (w *stderrAuditSink) consumeLineLocked(line string) {
	if w.store == nil {
		return
	}
	w.lineIndex++
	if !w.shouldKeepLineLocked(w.lineIndex) {
		return
	}
	summary, truncated := truncateUTF8(line, w.maxSummaryBytes)
	w.seq++
	meta := store.SafeMeta{
		"source":     "acp.stderr",
		"line_index": w.lineIndex,
	}
	if truncated > 0 {
		meta["truncated_bytes"] = truncated
	}
	_ = w.store.AppendAudit(context.Background(), store.AuditEvent{
		ID:        fmt.Sprintf("audit_acp_stderr_%d", w.seq),
		Type:      "acp.stderr",
		Level:     "warn",
		AgentID:   w.agentID,
		SessionID: w.session,
		RunID:     w.runID,
		Summary:   summary,
		SafeMeta:  meta,
		CreatedAt: w.now(),
	})
}

func (w *stderrAuditSink) shouldKeepLineLocked(lineIndex uint64) bool {
	return lineIndex == 1 || (lineIndex-1)%uint64(w.sampleEvery) == 0
}

func truncateUTF8(in string, maxBytes int) (string, int) {
	if len(in) <= maxBytes {
		return in, 0
	}
	cut := maxBytes
	for cut > 0 && (in[cut]&0xC0) == 0x80 {
		cut--
	}
	if cut == 0 {
		cut = maxBytes
	}
	return in[:cut], len(in) - cut
}
