package pty

import (
	"fmt"
	"sync"
)

// Manager tracks all active PTY sessions.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewManager creates a new, empty Manager.
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

// CreateSession spawns a new PTY session and registers it under the given id.
// It returns an error if a session with the same id already exists.
func (m *Manager) CreateSession(id, name string, argv []string, workDir string, env []string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[id]; exists {
		return nil, fmt.Errorf("pty: session %q already exists", id)
	}

	sess, err := newSession(id, name, argv, workDir, env)
	if err != nil {
		return nil, err
	}

	m.sessions[id] = sess
	return sess, nil
}

// GetSession returns the session with the given id, or an error if not found.
func (m *Manager) GetSession(id string) (*Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("pty: session %q not found", id)
	}
	return sess, nil
}

// DestroySession removes the session from the manager and closes it.
func (m *Manager) DestroySession(id string) error {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("pty: session %q not found", id)
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	return sess.Close()
}

// ListSessions returns metadata for every tracked session.
func (m *Manager) ListSessions() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]SessionInfo, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sess.mu.Lock()
		active := !sess.closed
		sess.mu.Unlock()

		infos = append(infos, SessionInfo{
			ID:        sess.id,
			Name:      sess.name,
			Active:    active,
			CreatedAt: sess.createdAt,
		})
	}
	return infos
}

// Close terminates and removes all sessions.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, sess := range m.sessions {
		_ = sess.Close()
		delete(m.sessions, id)
	}
}
