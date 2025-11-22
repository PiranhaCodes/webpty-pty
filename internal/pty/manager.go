package pty

import (
	"sync"
)

// Manager manages active PTY sessions in a thread-safe manner.
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// DefaultManager is the global session manager instance.
var DefaultManager = &Manager{sessions: make(map[string]*Session)}

// Add adds a session to the manager.
func (m *Manager) Add(id string, s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[id] = s
}

// Get retrieves a session by ID. Returns nil if not found.
func (m *Manager) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// Remove removes a session from the manager.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, id)
}

// List returns all active sessions.
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	return sessions
}

// Count returns the number of active sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
