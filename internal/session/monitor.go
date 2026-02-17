package session

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/hub"
	"github.com/user/agenterm/internal/parser"
)

var capturePaneFn = capturePane
var tmuxSessionExistsFn = tmuxSessionExists

type MonitorConfig struct {
	SessionID      string
	TmuxSession    string
	WindowID       string
	WorkDir        string
	SessionRepo    *db.SessionRepo
	Hub            *hub.Hub
	IdleTimeout    time.Duration
	PollInterval   time.Duration
	RingBufferSize int
	CaptureLines   int
}

type Monitor struct {
	sessionID   string
	tmuxSession string
	windowID    string
	workDir     string
	sessionRepo *db.SessionRepo
	hub         *hub.Hub

	idleTimeout  time.Duration
	pollInterval time.Duration
	captureLines int

	mu           sync.RWMutex
	lastOutput   time.Time
	lastStatus   string
	lastSnapshot []string
	buffer       *ringBuffer
}

func NewMonitor(cfg MonitorConfig) *Monitor {
	idleTimeout := cfg.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = defaultIdleTimeout
	}
	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultPollInterval
	}
	capLines := cfg.CaptureLines
	if capLines <= 0 {
		capLines = defaultCaptureLines
	}
	size := cfg.RingBufferSize
	if size <= 0 {
		size = defaultRingBufferLen
	}

	return &Monitor{
		sessionID:    cfg.SessionID,
		tmuxSession:  cfg.TmuxSession,
		windowID:     cfg.WindowID,
		workDir:      cfg.WorkDir,
		sessionRepo:  cfg.SessionRepo,
		hub:          cfg.Hub,
		idleTimeout:  idleTimeout,
		pollInterval: poll,
		captureLines: capLines,
		lastOutput:   time.Now().UTC(),
		lastStatus:   "working",
		buffer:       newRingBuffer(size),
	}
}

func (m *Monitor) Run(ctx context.Context) {
	if m.sessionID == "" || m.tmuxSession == "" {
		return
	}

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !tmuxSessionExistsFn(m.tmuxSession) {
				m.persistStatus(context.Background(), "completed")
				return
			}
			captured, err := capturePaneFn(m.windowID, m.captureLines)
			if err == nil {
				m.recordOutput(captured)
			}
			m.touchActivity(context.Background())

			status := m.detectStatus()
			if status != m.currentStatus() {
				m.persistStatus(context.Background(), status)
				if status == "completed" {
					return
				}
			}
		}
	}
}

func (m *Monitor) OutputSince(since time.Time) []OutputEntry {
	return m.buffer.Since(since)
}

func (m *Monitor) recordOutput(captured []string) {
	filtered := make([]string, 0, len(captured))
	for _, line := range captured {
		if strings.TrimSpace(line) != "" {
			filtered = append(filtered, line)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	newLines := diffNewLines(m.lastSnapshot, filtered)
	if len(newLines) > 0 {
		now := time.Now().UTC()
		for i, line := range newLines {
			m.buffer.Add(OutputEntry{Text: line, Timestamp: now.Add(time.Duration(i) * time.Microsecond)})
		}
		m.lastOutput = now
	}
	m.lastSnapshot = filtered
}

func (m *Monitor) detectStatus() string {
	if m.hasPrompt() {
		return "waiting_review"
	}
	if m.isIdle() {
		return "idle"
	}
	if m.isMarkerDone() {
		return "completed"
	}
	if m.hasReadyForReviewCommit() {
		return "waiting_review"
	}
	return "working"
}

func (m *Monitor) currentStatus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastStatus
}

func (m *Monitor) isIdle() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Since(m.lastOutput) >= m.idleTimeout
}

func (m *Monitor) hasPrompt() bool {
	entries := m.buffer.Last(20)
	for i := len(entries) - 1; i >= 0; i-- {
		line := strings.TrimSpace(entries[i].Text)
		if line == "" {
			continue
		}
		if parser.PromptShellPattern.MatchString(line) {
			return true
		}
		if parser.PromptConfirmPattern.MatchString(line) || parser.PromptQuestionPattern.MatchString(line) || parser.PromptBracketedChoicePattern.MatchString(line) {
			return true
		}
		break
	}
	return false
}

func (m *Monitor) isMarkerDone() bool {
	if strings.TrimSpace(m.workDir) == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(m.workDir, ".orchestra", "done"))
	return err == nil
}

func (m *Monitor) hasReadyForReviewCommit() bool {
	if strings.TrimSpace(m.workDir) == "" {
		return false
	}
	cmd := exec.Command("git", "-C", m.workDir, "log", "-1", "--pretty=%B")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "[READY_FOR_REVIEW]")
}

func (m *Monitor) persistStatus(ctx context.Context, status string) {
	m.mu.Lock()
	m.lastStatus = status
	m.mu.Unlock()

	if m.sessionRepo == nil {
		return
	}
	sess, err := m.sessionRepo.Get(ctx, m.sessionID)
	if err != nil || sess == nil {
		return
	}
	if sess.HumanAttached {
		return
	}
	sess.Status = status
	if err := m.sessionRepo.Update(ctx, sess); err != nil {
		return
	}
	if m.hub != nil {
		m.hub.BroadcastSessionStatus(m.sessionID, status)
	}
}

func (m *Monitor) touchActivity(ctx context.Context) {
	if m.sessionRepo == nil {
		return
	}
	sess, err := m.sessionRepo.Get(ctx, m.sessionID)
	if err != nil || sess == nil {
		return
	}
	_ = m.sessionRepo.Update(ctx, sess)
}

func capturePane(windowID string, lines int) ([]string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-p", "-t", windowID, "-S", fmt.Sprintf("-%d", lines))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("tmux capture-pane failed: %s", strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("tmux capture-pane failed: %w", err)
	}
	return strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n"), nil
}

func tmuxSessionExists(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
}

func diffNewLines(previous []string, current []string) []string {
	if len(previous) == 0 {
		return current
	}
	maxOverlap := min(len(previous), len(current))
	for overlap := maxOverlap; overlap > 0; overlap-- {
		if slices.Equal(previous[len(previous)-overlap:], current[:overlap]) {
			return current[overlap:]
		}
	}
	return current
}

type ringBuffer struct {
	mu      sync.RWMutex
	entries []OutputEntry
	size    int
}

func newRingBuffer(size int) *ringBuffer {
	if size <= 0 {
		size = defaultRingBufferLen
	}
	return &ringBuffer{size: size}
}

func (r *ringBuffer) Add(entry OutputEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
	if len(r.entries) > r.size {
		r.entries = r.entries[len(r.entries)-r.size:]
	}
}

func (r *ringBuffer) Since(since time.Time) []OutputEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]OutputEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		if since.IsZero() || entry.Timestamp.After(since) || entry.Timestamp.Equal(since) {
			result = append(result, entry)
		}
	}
	return result
}

func (r *ringBuffer) Last(n int) []OutputEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if n <= 0 || len(r.entries) == 0 {
		return nil
	}
	if n >= len(r.entries) {
		out := make([]OutputEntry, len(r.entries))
		copy(out, r.entries)
		return out
	}
	start := len(r.entries) - n
	out := make([]OutputEntry, n)
	copy(out, r.entries[start:])
	return out
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
