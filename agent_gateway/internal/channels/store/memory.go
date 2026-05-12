package store

import (
	"sort"
	"sync"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type StatusStore interface {
	Upsert(status models.ChannelRuntimeStatus)
	Delete(id string)
	List() []models.ChannelRuntimeStatus
	Get(id string) (models.ChannelRuntimeStatus, bool)
	Reset()
}

type MemoryStatusStore struct {
	mu       sync.RWMutex
	statuses map[string]models.ChannelRuntimeStatus
}

func NewMemoryStatusStore() *MemoryStatusStore {
	return &MemoryStatusStore{
		statuses: map[string]models.ChannelRuntimeStatus{},
	}
}

func (s *MemoryStatusStore) Upsert(status models.ChannelRuntimeStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses[status.ID] = status
}

func (s *MemoryStatusStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.statuses, id)
}

func (s *MemoryStatusStore) List() []models.ChannelRuntimeStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.ChannelRuntimeStatus, 0, len(s.statuses))
	for _, item := range s.statuses {
		out = append(out, cloneStatus(item))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *MemoryStatusStore) Get(id string) (models.ChannelRuntimeStatus, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status, ok := s.statuses[id]
	return cloneStatus(status), ok
}

func (s *MemoryStatusStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses = map[string]models.ChannelRuntimeStatus{}
}

func cloneStatus(in models.ChannelRuntimeStatus) models.ChannelRuntimeStatus {
	cp := in
	if in.StartedAt != nil {
		started := *in.StartedAt
		cp.StartedAt = &started
	}
	if in.StoppedAt != nil {
		stopped := *in.StoppedAt
		cp.StoppedAt = &stopped
	}
	return cp
}
