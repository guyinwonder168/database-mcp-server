package context

import (
	"sync"
	"time"
)

// Manager maintains per-session conversation history with TTL pruning.
type Manager struct {
	mu        sync.Mutex
	store     map[string]*Conversation
	expires   map[string]time.Time
	ttl       time.Duration
	maxRecent int
}

// NewManager creates a context manager with TTL and history cap.
func NewManager(ttl time.Duration, maxRecent int) *Manager {
	if ttl == 0 {
		ttl = 30 * time.Minute
	}
	if maxRecent <= 0 {
		maxRecent = 20
	}
	return &Manager{
		store:     map[string]*Conversation{},
		expires:   map[string]time.Time{},
		ttl:       ttl,
		maxRecent: maxRecent,
	}
}

// Append adds a message to a session conversation.
func (m *Manager) Append(sessionID, role, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	conv, ok := m.store[sessionID]
	if !ok {
		conv = &Conversation{}
		m.store[sessionID] = conv
	}
	conv.Add(role, content)
	if len(conv.Messages) > m.maxRecent {
		conv.Messages = conv.Messages[len(conv.Messages)-m.maxRecent:]
	}
	m.expires[sessionID] = time.Now().UTC().Add(m.ttl)
}

// History returns conversation messages for session (may be empty).
func (m *Manager) History(sessionID string) []Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneLocked()
	if conv, ok := m.store[sessionID]; ok {
		return conv.History()
	}
	return nil
}

func (m *Manager) pruneLocked() {
	now := time.Now().UTC()
	for id, exp := range m.expires {
		if now.After(exp) {
			delete(m.store, id)
			delete(m.expires, id)
		}
	}
}

// SetTTL updates the TTL used for new conversations.
func (m *Manager) SetTTL(ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ttl = ttl
}

// SetMaxRecent updates how many recent messages to keep.
func (m *Manager) SetMaxRecent(maxRecent int) {
	if maxRecent <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxRecent = maxRecent
}
