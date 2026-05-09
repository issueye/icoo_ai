package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/agent"
)

var ErrNotFound = errors.New("session not found")

type FileStore struct {
	dir string
	mu  sync.Mutex
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

func DefaultDir(home string) string {
	return filepath.Join(home, ".icoo-ai", "sessions")
}

func (s *FileStore) Create(ctx context.Context, session agent.Session) (agent.Session, error) {
	if err := ctx.Err(); err != nil {
		return agent.Session{}, err
	}
	now := time.Now().UTC()
	if session.ID == "" {
		session.ID = newID(now)
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	session.UpdatedAt = now

	if err := validateID(session.ID); err != nil {
		return agent.Session{}, err
	}
	if err := s.write(session); err != nil {
		return agent.Session{}, err
	}
	return session, nil
}

func (s *FileStore) Get(ctx context.Context, id string) (agent.Session, error) {
	if err := ctx.Err(); err != nil {
		return agent.Session{}, err
	}
	if err := validateID(id); err != nil {
		return agent.Session{}, err
	}

	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return agent.Session{}, ErrNotFound
	}
	if err != nil {
		return agent.Session{}, fmt.Errorf("read session %q: %w", id, err)
	}

	var session agent.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return agent.Session{}, fmt.Errorf("decode session %q: %w", id, err)
	}
	return session, nil
}

func (s *FileStore) Update(ctx context.Context, session agent.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if session.ID == "" {
		return errors.New("session id is required")
	}
	session.UpdatedAt = time.Now().UTC()
	return s.write(session)
}

func (s *FileStore) List(ctx context.Context) ([]agent.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	sessions := make([]agent.Session, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		session, err := s.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (s *FileStore) write(session agent.Session) error {
	if err := validateID(session.ID); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session %q: %w", session.ID, err)
	}
	data = append(data, '\n')

	path := s.path(session.ID)
	tmp, err := os.CreateTemp(s.dir, session.ID+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp session file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp session file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp session file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace session file: %w", err)
	}
	return nil
}

func (s *FileStore) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func validateID(id string) error {
	if id == "" {
		return errors.New("session id is required")
	}
	if strings.ContainsAny(id, `/\`) || id == "." || id == ".." {
		return fmt.Errorf("invalid session id %q", id)
	}
	return nil
}

func newID(now time.Time) string {
	return fmt.Sprintf("sess_%d", now.UnixNano())
}
