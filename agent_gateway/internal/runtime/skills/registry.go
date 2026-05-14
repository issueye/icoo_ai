package skills

import "sync"

type RegistryEventType string

const (
	EventRegistered   RegistryEventType = "registered"
	EventUpdated      RegistryEventType = "updated"
	EventUnregistered RegistryEventType = "unregistered"
)

type RegistryEvent struct {
	Type    RegistryEventType
	Skill   Skill
	SkillID string
}

type Registry struct {
	mu        sync.RWMutex
	skills    map[string]Skill
	listeners map[uint64]func(RegistryEvent)
	nextID    uint64
}

func NewRegistry() *Registry {
	return &Registry{
		skills:    map[string]Skill{},
		listeners: map[uint64]func(RegistryEvent){},
	}
}

func (r *Registry) Register(skill Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()

	eventType := EventRegistered
	if _, ok := r.skills[skill.ID]; ok {
		eventType = EventUpdated
	}
	r.skills[skill.ID] = skill
	r.notifyLocked(RegistryEvent{Type: eventType, Skill: skill, SkillID: skill.ID})
}

func (r *Registry) Update(skill Skill) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.skills[skill.ID]; !ok {
		return false
	}
	r.skills[skill.ID] = skill
	r.notifyLocked(RegistryEvent{Type: EventUpdated, Skill: skill, SkillID: skill.ID})
	return true
}

func (r *Registry) Unregister(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.skills[id]; !ok {
		return false
	}
	delete(r.skills, id)
	r.notifyLocked(RegistryEvent{Type: EventUnregistered, SkillID: id})
	return true
}

func (r *Registry) Get(id string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, ok := r.skills[id]
	return skill, ok
}

func (r *Registry) Snapshot() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Skill, 0, len(r.skills))
	for _, skill := range r.skills {
		out = append(out, skill)
	}
	return out
}

func (r *Registry) OnChange(listener func(RegistryEvent)) func() {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := r.nextID
	r.nextID++
	r.listeners[id] = listener

	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.listeners, id)
	}
}

func (r *Registry) notifyLocked(event RegistryEvent) {
	listeners := make([]func(RegistryEvent), 0, len(r.listeners))
	for _, listener := range r.listeners {
		listeners = append(listeners, listener)
	}
	go func() {
		for _, listener := range listeners {
			listener(event)
		}
	}()
}
