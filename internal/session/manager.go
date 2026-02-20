package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/hub"
	"github.com/user/agenterm/internal/registry"
	"github.com/user/agenterm/internal/tmux"
)

const (
	defaultIdleTimeout   = 30 * time.Second
	defaultPollInterval  = time.Second
	defaultRingBufferLen = 500
	defaultCaptureLines  = 800
)

type TmuxManager interface {
	CreateSession(name string, workDir string) (*tmux.Gateway, error)
	AttachSession(name string) (*tmux.Gateway, error)
	GetGateway(name string) (*tmux.Gateway, error)
	DestroySession(name string) error
	ListSessions() []string
}

type CreateSessionRequest struct {
	TaskID    string
	AgentType string
	Role      string
}

type OutputEntry struct {
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

type Manager struct {
	tmux         TmuxManager
	registry     *registry.Registry
	hub          *hub.Hub
	sessionRepo  *db.SessionRepo
	taskRepo     *db.TaskRepo
	projectRepo  *db.ProjectRepo
	worktreeRepo *db.WorktreeRepo

	idleTimeout   time.Duration
	pollInterval  time.Duration
	ringBufferLen int
	captureLines  int

	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	monitors map[string]monitorHandle
}

type monitorHandle struct {
	monitor *Monitor
	cancel  context.CancelFunc
}

func NewManager(conn *sql.DB, tmuxMgr TmuxManager, reg *registry.Registry, hubInst *hub.Hub) *Manager {
	if conn == nil || tmuxMgr == nil || reg == nil {
		return nil
	}

	return &Manager{
		tmux:          tmuxMgr,
		registry:      reg,
		hub:           hubInst,
		sessionRepo:   db.NewSessionRepo(conn),
		taskRepo:      db.NewTaskRepo(conn),
		projectRepo:   db.NewProjectRepo(conn),
		worktreeRepo:  db.NewWorktreeRepo(conn),
		idleTimeout:   defaultIdleTimeout,
		pollInterval:  defaultPollInterval,
		ringBufferLen: defaultRingBufferLen,
		captureLines:  defaultCaptureLines,
		monitors:      make(map[string]monitorHandle),
	}
}

func (sm *Manager) Start(ctx context.Context) error {
	if sm == nil {
		return fmt.Errorf("session manager unavailable")
	}

	sm.mu.Lock()
	if sm.cancel != nil {
		sm.mu.Unlock()
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sm.ctx, sm.cancel = context.WithCancel(ctx)
	sm.mu.Unlock()

	active, err := sm.sessionRepo.ListActive(context.Background())
	if err != nil {
		return err
	}
	for _, sess := range active {
		if err := sm.ensureMonitorForSession(context.Background(), sess); err != nil {
			slog.Warn("failed to start monitor for active session", "session_id", sess.ID, "error", err)
		}
	}
	return nil
}

func (sm *Manager) Close() {
	if sm == nil {
		return
	}

	sm.mu.Lock()
	handles := make([]monitorHandle, 0, len(sm.monitors))
	for _, handle := range sm.monitors {
		handles = append(handles, handle)
	}
	if sm.cancel != nil {
		sm.cancel()
		sm.cancel = nil
	}
	sm.monitors = make(map[string]monitorHandle)
	sm.mu.Unlock()
	for _, handle := range handles {
		if handle.cancel != nil {
			handle.cancel()
		}
	}
}

func (sm *Manager) CreateSession(ctx context.Context, req CreateSessionRequest) (*db.Session, error) {
	if err := sm.ensureStarted(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.TaskID) == "" {
		return nil, fmt.Errorf("task id is required")
	}
	if strings.TrimSpace(req.AgentType) == "" {
		return nil, fmt.Errorf("agent type is required")
	}
	if strings.TrimSpace(req.Role) == "" {
		return nil, fmt.Errorf("role is required")
	}

	agent := sm.registry.Get(req.AgentType)
	if agent == nil {
		return nil, fmt.Errorf("unknown agent type %q", req.AgentType)
	}

	task, err := sm.taskRepo.Get(ctx, req.TaskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errNotFound("task")
	}

	project, err := sm.projectRepo.Get(ctx, task.ProjectID)
	if err != nil {
		return nil, err
	}
	if project == nil {
		return nil, fmt.Errorf("project for task not found")
	}

	workDir, err := sm.resolveWorkDir(ctx, task, project)
	if err != nil {
		return nil, err
	}

	sessionName, gw, err := sm.createTmuxSession(project.Name, task.Title, req.Role, workDir)
	if err != nil {
		return nil, err
	}

	windows := gw.ListWindows()
	if len(windows) == 0 {
		_ = sm.tmux.DestroySession(sessionName)
		return nil, fmt.Errorf("created tmux session has no windows")
	}

	session := &db.Session{
		TaskID:          req.TaskID,
		TmuxSessionName: sessionName,
		TmuxWindowID:    windows[0].ID,
		AgentType:       req.AgentType,
		Role:            req.Role,
		Status:          "working",
		HumanAttached:   false,
	}
	if err := sm.sessionRepo.Create(ctx, session); err != nil {
		_ = sm.tmux.DestroySession(sessionName)
		return nil, err
	}

	if err := gw.SendRaw(session.TmuxWindowID, agent.Command+"\n"); err != nil {
		session.Status = "failed"
		_ = sm.sessionRepo.Update(ctx, session)
		return nil, err
	}

	if seq, ok := autoAcceptSequence(agent.AutoAcceptMode); ok {
		go func(windowID string, mode string) {
			time.Sleep(600 * time.Millisecond)
			if err := gw.SendRaw(windowID, mode); err != nil {
				slog.Debug("auto-accept send failed", "session", session.ID, "error", err)
			}
		}(session.TmuxWindowID, seq)
	}

	if err := sm.ensureMonitorForSession(ctx, session); err != nil {
		slog.Warn("failed to start session monitor", "session_id", session.ID, "error", err)
	}
	if sm.hub != nil {
		sm.hub.BroadcastSessionStatus(session.ID, session.Status)
	}

	return session, nil
}

func (sm *Manager) SendCommand(ctx context.Context, sessionID string, text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("text is required")
	}
	session, err := sm.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return errNotFound("session")
	}
	if session.TmuxWindowID == "" {
		return fmt.Errorf("session has no tmux window")
	}
	workDir := sm.resolveWorkDirForSession(ctx, session)
	if err := enforceCommandPolicy(text, workDir); err != nil {
		if policyErr, ok := err.(*CommandPolicyError); ok {
			auditCommandPolicyViolation(workDir, sessionID, text, policyErr)
		}
		return err
	}

	gw, err := sm.gatewayForSession(session.TmuxSessionName)
	if err != nil {
		return err
	}
	if err := gw.SendRaw(session.TmuxWindowID, text); err != nil {
		return err
	}

	session.Status = "working"
	if err := sm.sessionRepo.Update(ctx, session); err != nil {
		return err
	}
	if sm.hub != nil {
		sm.hub.BroadcastSessionStatus(sessionID, session.Status)
	}
	return nil
}

func (sm *Manager) GetOutput(ctx context.Context, sessionID string, since time.Time) ([]OutputEntry, error) {
	session, err := sm.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errNotFound("session")
	}
	if err := sm.ensureMonitorForSession(ctx, session); err != nil {
		return nil, err
	}

	sm.mu.RLock()
	handle := sm.monitors[sessionID]
	sm.mu.RUnlock()
	if handle.monitor == nil {
		return []OutputEntry{}, nil
	}
	return handle.monitor.OutputSince(since), nil
}

type SessionReadyState struct {
	Ready          bool      `json:"ready"`
	Reason         string    `json:"reason"`
	Status         string    `json:"status"`
	LastActivityAt time.Time `json:"last_activity_at"`
	PromptDetected bool      `json:"prompt_detected"`
	ObservedOutput bool      `json:"observed_output"`
	LastClass      string    `json:"last_class"`
	LastText       string    `json:"last_text"`
}

func (sm *Manager) GetSessionReadyState(ctx context.Context, sessionID string) (SessionReadyState, error) {
	session, err := sm.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		return SessionReadyState{}, err
	}
	if session == nil {
		return SessionReadyState{}, errNotFound("session")
	}
	if err := sm.ensureMonitorForSession(ctx, session); err != nil {
		return SessionReadyState{}, err
	}

	sm.mu.RLock()
	handle := sm.monitors[sessionID]
	sm.mu.RUnlock()
	if handle.monitor == nil {
		return SessionReadyState{
			Ready:          false,
			Reason:         "monitor_unavailable",
			Status:         session.Status,
			LastActivityAt: session.LastActivityAt,
		}, nil
	}

	monitorState := handle.monitor.ReadyState()
	state := SessionReadyState{
		Status:         session.Status,
		LastActivityAt: session.LastActivityAt,
		PromptDetected: monitorState.PromptDetected,
		ObservedOutput: monitorState.ObservedOutput,
		LastClass:      monitorState.LastClass,
		LastText:       monitorState.LastText,
	}
	if monitorState.PromptDetected {
		state.Ready = true
		state.Reason = "prompt_detected"
		return state, nil
	}
	if monitorState.ObservedOutput {
		state.Ready = true
		state.Reason = "output_observed"
		return state, nil
	}
	switch strings.ToLower(strings.TrimSpace(session.Status)) {
	case "idle", "waiting_review", "human_takeover", "completed", "failed":
		state.Ready = true
		state.Reason = "status_" + strings.ToLower(strings.TrimSpace(session.Status))
	default:
		state.Ready = false
		state.Reason = "booting"
	}
	return state, nil
}

func (sm *Manager) SetTakeover(ctx context.Context, sessionID string, takeover bool) error {
	session, err := sm.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return errNotFound("session")
	}

	session.HumanAttached = takeover
	if takeover {
		session.Status = "human_takeover"
	} else {
		session.Status = "idle"
	}
	if err := sm.sessionRepo.Update(ctx, session); err != nil {
		return err
	}
	if sm.hub != nil {
		sm.hub.BroadcastSessionStatus(sessionID, session.Status)
	}
	return nil
}

func (sm *Manager) DestroySession(ctx context.Context, sessionID string) error {
	session, err := sm.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return errNotFound("session")
	}

	sm.stopMonitor(sessionID)
	if err := sm.tmux.DestroySession(session.TmuxSessionName); err != nil {
		return err
	}

	session.Status = "completed"
	if err := sm.sessionRepo.Update(ctx, session); err != nil {
		return err
	}
	if sm.hub != nil {
		sm.hub.BroadcastSessionStatus(sessionID, session.Status)
	}
	return nil
}

func (sm *Manager) ObserveParsedOutput(tmuxSession string, windowID string, text string, class string, timestamp time.Time) {
	if sm == nil {
		return
	}
	tmuxSession = strings.TrimSpace(tmuxSession)
	windowID = strings.TrimSpace(windowID)
	if tmuxSession == "" || windowID == "" {
		return
	}
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	sm.mu.RLock()
	handles := make([]monitorHandle, 0, len(sm.monitors))
	for _, handle := range sm.monitors {
		handles = append(handles, handle)
	}
	sm.mu.RUnlock()

	for _, handle := range handles {
		if handle.monitor == nil {
			continue
		}
		if !handle.monitor.matches(tmuxSession, windowID) {
			continue
		}
		handle.monitor.IngestParsed(text, class, timestamp)
	}
}

func (sm *Manager) ensureStarted() error {
	if sm == nil {
		return fmt.Errorf("session manager unavailable")
	}
	sm.mu.RLock()
	started := sm.cancel != nil
	sm.mu.RUnlock()
	if started {
		return nil
	}
	return sm.Start(context.Background())
}

func (sm *Manager) createTmuxSession(projectName string, taskTitle string, role string, workDir string) (string, *tmux.Gateway, error) {
	base := buildTmuxSessionName(projectName, taskTitle, role)
	if base == "" {
		base = "session"
	}

	for attempt := 0; attempt < 8; attempt++ {
		name := base
		if attempt > 0 {
			suffix := fmt.Sprintf("-%02d", attempt)
			if len(name)+len(suffix) > 80 {
				name = name[:80-len(suffix)]
			}
			name += suffix
		}
		gw, err := sm.tmux.CreateSession(name, workDir)
		if err == nil {
			return name, gw, nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return "", nil, err
		}
	}
	const maxRandomAttempts = 8
	for i := 0; i < maxRandomAttempts; i++ {
		name := base
		randomID, err := db.NewID()
		if err != nil {
			return "", nil, err
		}
		suffix := "-" + randomID[:8]
		if len(name)+len(suffix) > 80 {
			name = name[:80-len(suffix)]
		}
		name += suffix
		gw, err := sm.tmux.CreateSession(name, workDir)
		if err == nil {
			return name, gw, nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return "", nil, err
		}
	}
	return "", nil, fmt.Errorf("failed to allocate unique tmux session name")
}

func (sm *Manager) resolveWorkDir(ctx context.Context, task *db.Task, project *db.Project) (string, error) {
	workDir := project.RepoPath
	if task.WorktreeID == "" {
		return workDir, nil
	}
	wt, err := sm.worktreeRepo.Get(ctx, task.WorktreeID)
	if err != nil {
		return "", err
	}
	if wt != nil && strings.TrimSpace(wt.Path) != "" {
		return wt.Path, nil
	}
	return workDir, nil
}

func (sm *Manager) resolveWorkDirForSession(ctx context.Context, session *db.Session) string {
	if session == nil || session.TaskID == "" {
		return ""
	}
	task, err := sm.taskRepo.Get(ctx, session.TaskID)
	if err != nil || task == nil {
		return ""
	}
	project, err := sm.projectRepo.Get(ctx, task.ProjectID)
	if err != nil || project == nil {
		return ""
	}
	workDir, err := sm.resolveWorkDir(ctx, task, project)
	if err != nil {
		return ""
	}
	return workDir
}

func (sm *Manager) ensureMonitorForSession(ctx context.Context, session *db.Session) error {
	if session == nil {
		return fmt.Errorf("session is required")
	}
	if err := sm.ensureStarted(); err != nil {
		return err
	}

	sm.mu.Lock()
	if existing := sm.monitors[session.ID]; existing.monitor != nil {
		sm.mu.Unlock()
		return nil
	}
	monitorCtx := sm.ctx
	if monitorCtx == nil {
		monitorCtx = context.Background()
	}
	childCtx, cancel := context.WithCancel(monitorCtx)
	workDir := sm.resolveWorkDirForSession(ctx, session)
	mon := NewMonitor(MonitorConfig{
		SessionID:      session.ID,
		TmuxSession:    session.TmuxSessionName,
		WindowID:       session.TmuxWindowID,
		WorkDir:        workDir,
		SessionRepo:    sm.sessionRepo,
		Hub:            sm.hub,
		IdleTimeout:    sm.idleTimeout,
		PollInterval:   sm.pollInterval,
		RingBufferSize: sm.ringBufferLen,
		CaptureLines:   sm.captureLines,
	})
	sm.monitors[session.ID] = monitorHandle{monitor: mon, cancel: cancel}
	sm.mu.Unlock()

	go func() {
		mon.Run(childCtx)
		sm.mu.Lock()
		handle := sm.monitors[session.ID]
		if handle.monitor == mon {
			delete(sm.monitors, session.ID)
		}
		sm.mu.Unlock()
	}()
	return nil
}

func (sm *Manager) stopMonitor(sessionID string) {
	sm.mu.Lock()
	handle := sm.monitors[sessionID]
	delete(sm.monitors, sessionID)
	sm.mu.Unlock()
	if handle.cancel != nil {
		handle.cancel()
	}
}

func (sm *Manager) gatewayForSession(sessionName string) (*tmux.Gateway, error) {
	gw, err := sm.tmux.GetGateway(sessionName)
	if err == nil {
		return gw, nil
	}
	return sm.tmux.AttachSession(sessionName)
}

func buildTmuxSessionName(projectName string, taskTitle string, role string) string {
	parts := []string{slugPart(projectName), slugPart(taskTitle), slugPart(role)}
	for i := range parts {
		if parts[i] == "" {
			parts[i] = "x"
		}
	}
	return strings.Join(parts, "-")
}

func slugPart(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	var b strings.Builder
	lastDash := false
	for _, r := range v {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 36 {
		s = s[:36]
	}
	return s
}

func errNotFound(kind string) error {
	return fmt.Errorf("%s not found", kind)
}

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(err.Error()), "not found") || errors.Is(err, sql.ErrNoRows)
}

func autoAcceptSequence(mode string) (string, bool) {
	raw := strings.TrimSpace(mode)
	if raw == "" {
		return "", false
	}

	switch strings.ToLower(raw) {
	case "none", "optional", "disabled":
		return "", false
	case "supported", "enter", "return":
		return "\n", true
	case "tab":
		return "\t", true
	case "shift+tab", "shift-tab", "backtab":
		return "\x1b[Z", true
	case "ctrl+c", "ctrl-c":
		return "\x03", true
	}

	if strings.Contains(raw, `\`) {
		if decoded, err := strconv.Unquote(`"` + strings.ReplaceAll(raw, `"`, `\"`) + `"`); err == nil {
			return decoded, true
		}
	}

	return raw, true
}
