package audit

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	DefaultMaxSizeMB = 100
	DefaultBackups   = 5
)

type Logger interface {
	Log(ctx context.Context, event Event) error
}

type LoggerOptions struct {
	Path       string
	MaxSizeMB  int
	MaxBackups int
}

type SlogLogger struct {
	handler slog.Handler
}

func NewJSONLLogger(path string) *SlogLogger {
	return NewSlogLogger(LoggerOptions{Path: path})
}

func NewSlogLogger(opts LoggerOptions) *SlogLogger {
	writer := newRotatingWriter(opts.Path, opts.MaxSizeMB, opts.MaxBackups)
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			if attr.Key == slog.LevelKey {
				return slog.Attr{}
			}
			return attr
		},
	})
	return &SlogLogger{handler: handler}
}

func DefaultDir(home string) string {
	return filepath.Join(home, ".icoo-ai", "audit")
}

func DefaultPath(home string) string {
	return filepath.Join(DefaultDir(home), "audit.jsonl")
}

func (l *SlogLogger) Log(ctx context.Context, event Event) error {
	if l == nil || l.handler == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	event.Data = RedactMap(event.Data)

	record := slog.NewRecord(event.Timestamp.UTC(), slog.LevelInfo, event.Summary, 0)
	record.AddAttrs(
		slog.String("timestamp", event.Timestamp.UTC().Format(time.RFC3339Nano)),
		slog.String("type", string(event.Type)),
	)
	if event.ID != "" {
		record.AddAttrs(slog.String("id", event.ID))
	}
	if event.SessionID != "" {
		record.AddAttrs(slog.String("session_id", event.SessionID))
	}
	if event.UserID != "" {
		record.AddAttrs(slog.String("user_id", event.UserID))
	}
	if event.Summary != "" {
		record.AddAttrs(slog.String("summary", event.Summary))
	}
	if event.Data != nil {
		record.AddAttrs(slog.Any("data", event.Data))
	}
	if err := l.handler.Handle(ctx, record); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}

type rotatingWriter struct {
	path       string
	maxBytes   int64
	maxBackups int

	mu   sync.Mutex
	file *os.File
	size int64
}

func newRotatingWriter(path string, maxSizeMB int, maxBackups int) io.Writer {
	if maxSizeMB <= 0 {
		maxSizeMB = DefaultMaxSizeMB
	}
	if maxBackups < 0 {
		maxBackups = DefaultBackups
	}
	return &rotatingWriter{
		path:       path,
		maxBytes:   int64(maxSizeMB) * 1024 * 1024,
		maxBackups: maxBackups,
	}
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.ensureOpen(); err != nil {
		return 0, err
	}
	if w.shouldRotate(len(p)) {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	if closeErr := w.closeFile(); err == nil && closeErr != nil {
		err = closeErr
	}
	return n, err
}

func (w *rotatingWriter) ensureOpen() error {
	if w.path == "" {
		return fmt.Errorf("audit log path is required")
	}
	if w.file != nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(w.path), 0o700); err != nil {
		return fmt.Errorf("create audit directory: %w", err)
	}
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("stat audit log: %w", err)
	}
	w.file = file
	w.size = info.Size()
	return nil
}

func (w *rotatingWriter) shouldRotate(nextBytes int) bool {
	return w.maxBytes > 0 && w.size > 0 && w.size+int64(nextBytes) > w.maxBytes
}

func (w *rotatingWriter) rotate() error {
	if err := w.closeFile(); err != nil {
		return fmt.Errorf("close audit log before rotate: %w", err)
	}
	w.size = 0
	if w.maxBackups > 0 {
		oldest := rotatedPath(w.path, w.maxBackups)
		if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove old audit backup: %w", err)
		}
		for i := w.maxBackups - 1; i >= 1; i-- {
			src := rotatedPath(w.path, i)
			dst := rotatedPath(w.path, i+1)
			if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("rotate audit backup: %w", err)
			}
		}
		if err := os.Rename(w.path, rotatedPath(w.path, 1)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rotate audit log: %w", err)
		}
	} else if err := os.Remove(w.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove audit log: %w", err)
	}
	return w.ensureOpen()
}

func (w *rotatingWriter) closeFile() error {
	if w.file == nil {
		return nil
	}
	file := w.file
	w.file = nil
	return file.Close()
}

func rotatedPath(path string, index int) string {
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	return base + "." + strconv.Itoa(index) + ext
}
