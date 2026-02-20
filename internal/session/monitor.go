package session

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/hub"
	"github.com/user/agenterm/internal/parser"
)

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

	mu         sync.RWMutex
	lastOutput time.Time
	lastStatus string
	lastClass  string
	lastText   string
	buffer     *ringBuffer

	lastCompletionCheck time.Time
	markerDoneCached    bool
	readyCommitCached   bool
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
				m.persistStatus(context.Background(), m.statusOnSessionExit())
				return
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

type ReadyState struct {
	PromptDetected bool   `json:"prompt_detected"`
	ObservedOutput bool   `json:"observed_output"`
	LastClass      string `json:"last_class"`
	LastText       string `json:"last_text"`
}

func (m *Monitor) ReadyState() ReadyState {
	if m == nil {
		return ReadyState{}
	}
	hasOutput := len(m.buffer.Last(1)) > 0
	m.mu.RLock()
	defer m.mu.RUnlock()
	return ReadyState{
		PromptDetected: m.lastClass == string(parser.ClassPrompt) && parser.PromptShellPattern.MatchString(strings.TrimSpace(m.lastText)),
		ObservedOutput: hasOutput,
		LastClass:      m.lastClass,
		LastText:       m.lastText,
	}
}

func (m *Monitor) IngestParsed(text string, class string, ts time.Time) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buffer.Add(OutputEntry{Text: text, Timestamp: ts})
	m.lastOutput = ts
	m.lastClass = strings.ToLower(strings.TrimSpace(class))
	m.lastText = text
}

func (m *Monitor) detectStatus() string {
	if m.hasPrompt() {
		return "waiting_review"
	}
	if m.isIdle() {
		return "idle"
	}
	m.refreshCompletionSignals(false)
	if m.isMarkerDoneCached() {
		return "completed"
	}
	if m.hasReadyForReviewCommitCached() {
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
	m.mu.RLock()
	lastClass := m.lastClass
	lastText := strings.TrimSpace(m.lastText)
	m.mu.RUnlock()
	// Task spec requires parser-based prompt detection for shell prompts.
	if lastClass == string(parser.ClassPrompt) && parser.PromptShellPattern.MatchString(lastText) {
		return true
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

func (m *Monitor) statusOnSessionExit() string {
	m.refreshCompletionSignals(true)
	if m.isMarkerDoneCached() {
		return "completed"
	}
	if m.hasReadyForReviewCommitCached() {
		return "waiting_review"
	}
	return "failed"
}

func (m *Monitor) refreshCompletionSignals(force bool) {
	if strings.TrimSpace(m.workDir) == "" {
		return
	}
	now := time.Now().UTC()
	m.mu.RLock()
	lastCheck := m.lastCompletionCheck
	m.mu.RUnlock()
	if !force && !lastCheck.IsZero() && now.Sub(lastCheck) < 5*time.Second {
		return
	}
	marker := m.isMarkerDone()
	ready := m.hasReadyForReviewCommit()
	m.mu.Lock()
	m.lastCompletionCheck = now
	m.markerDoneCached = marker
	m.readyCommitCached = ready
	m.mu.Unlock()
}

func (m *Monitor) isMarkerDoneCached() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.markerDoneCached
}

func (m *Monitor) hasReadyForReviewCommitCached() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.readyCommitCached
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

func tmuxSessionExists(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
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

func (m *Monitor) matches(tmuxSession string, windowID string) bool {
	if strings.TrimSpace(tmuxSession) == "" || strings.TrimSpace(windowID) == "" {
		return false
	}
	return m.tmuxSession == tmuxSession && m.windowID == windowID
}
