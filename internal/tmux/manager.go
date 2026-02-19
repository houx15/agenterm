package tmux

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
)

type Manager struct {
	gateways   map[string]*Gateway
	mu         sync.RWMutex
	defaultDir string
}

var ErrSessionNotFound = errors.New("tmux session not found")

func NewManager(defaultDir string) *Manager {
	return &Manager{
		gateways:   make(map[string]*Gateway),
		defaultDir: defaultDir,
	}
}

func (m *Manager) CreateSession(name string, workDir string) (*Gateway, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("session name is required")
	}

	exists, err := tmuxSessionExists(name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("tmux session %q already exists", name)
	}

	dir := strings.TrimSpace(workDir)
	if dir == "" {
		dir = strings.TrimSpace(m.defaultDir)
	}

	args := []string{"new-session", "-d", "-s", name}
	if dir != "" {
		args = append(args, "-c", dir)
	}

	if out, err := exec.Command("tmux", args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to create tmux session %q: %s", name, strings.TrimSpace(string(out)))
	}

	return m.AttachSession(name)
}

func (m *Manager) AttachSession(name string) (*Gateway, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("session name is required")
	}

	m.mu.RLock()
	if gw, ok := m.gateways[name]; ok {
		m.mu.RUnlock()
		return gw, nil
	}
	m.mu.RUnlock()

	exists, err := tmuxSessionExists(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("%w: %q", ErrSessionNotFound, name)
	}

	gw := New(name)
	if err := gw.Start(context.Background()); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.gateways[name]; ok {
		_ = gw.Close()
		return existing, nil
	}
	m.gateways[name] = gw
	return gw, nil
}

func (m *Manager) DestroySession(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("session name is required")
	}

	m.mu.Lock()
	gw := m.gateways[name]
	delete(m.gateways, name)
	m.mu.Unlock()

	if gw != nil {
		_ = gw.Close()
	}

	cmd := exec.Command("tmux", "kill-session", "-t", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		if isTmuxNoSessionError(err) {
			return nil
		}
		return fmt.Errorf("failed to kill tmux session %q: %s", name, strings.TrimSpace(string(out)))
	}
	return nil
}

func (m *Manager) GetGateway(name string) (*Gateway, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("session name is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	gw, ok := m.gateways[name]
	if !ok {
		return nil, fmt.Errorf("gateway not found for session %q", name)
	}
	return gw, nil
}

func (m *Manager) ListSessions() []string {
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	sessions := make([]string, 0, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		sessions = append(sessions, name)
	}
	sort.Strings(sessions)
	return sessions
}

func (m *Manager) Close() {
	m.mu.Lock()
	gateways := make([]*Gateway, 0, len(m.gateways))
	for _, gw := range m.gateways {
		gateways = append(gateways, gw)
	}
	m.gateways = make(map[string]*Gateway)
	m.mu.Unlock()

	for _, gw := range gateways {
		_ = gw.Close()
	}
}

func tmuxSessionExists(name string) (bool, error) {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}

	if execErr, ok := err.(*exec.Error); ok && errors.Is(execErr.Err, exec.ErrNotFound) {
		return false, fmt.Errorf("tmux binary not found. Please install tmux")
	}
	if isTmuxNoSessionError(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check tmux session %q: %w", name, err)
}

func isTmuxNoSessionError(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}
