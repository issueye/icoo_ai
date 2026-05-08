package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger interface {
	Log(ctx context.Context, event Event) error
}

type JSONLLogger struct {
	path string
	mu   sync.Mutex
}

func NewJSONLLogger(path string) *JSONLLogger {
	return &JSONLLogger{path: path}
}

func DefaultDir(home string) string {
	return filepath.Join(home, ".icoo-ai", "audit")
}

func DefaultPath(home string) string {
	return filepath.Join(DefaultDir(home), "audit.jsonl")
}

func (l *JSONLLogger) Log(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	event.Data = RedactMap(event.Data)

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	data = append(data, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return fmt.Errorf("create audit directory: %w", err)
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}
