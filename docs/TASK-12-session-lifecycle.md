# Task: session-lifecycle

## Context
Sessions are the core execution unit: each session = one agent running in one tmux session, working on one task. The system needs to manage the full lifecycle: create a tmux session → start the agent command → monitor status → detect completion → clean up. This connects the database models, tmux manager, and agent registry.

## Objective
Implement the full session lifecycle: creation (spawn tmux session + start agent), monitoring (idle detection, status tracking), command sending, output capture, and human takeover.

## Dependencies
- Depends on: TASK-09 (agent-registry), TASK-10 (multi-session-tmux)
- Branch: feature/session-lifecycle
- Base: main (after TASK-09 and TASK-10 merge)

## Scope

### Files to Create
- `internal/session/manager.go` — Session lifecycle manager: create, monitor, destroy sessions
- `internal/session/monitor.go` — Session monitoring: idle detection, completion detection, status updates

### Files to Modify
- `internal/api/sessions.go` — Wire to session manager
- `internal/hub/hub.go` — Emit session status events
- `cmd/agenterm/main.go` — Initialize session manager

### Files NOT to Touch
- `internal/db/` — Use existing repos
- `internal/parser/` — Use existing parser for output classification
- `web/` — Minimal frontend changes

## Implementation Spec

### Step 1: Session Manager
```go
type SessionManager struct {
    db       *db.DB
    tmux     *tmux.Manager
    registry *registry.Registry
    hub      *hub.Hub
    monitors map[string]*Monitor  // session_id → monitor
}

// CreateSession: full lifecycle
// 1. Validate agent_type exists in registry
// 2. Generate tmux session name: {project}-{task}-{role}
// 3. Create tmux session via Manager (cd to worktree dir)
// 4. Save session record to DB
// 5. Send agent start command to tmux session
// 6. Start monitoring goroutine
func (sm *SessionManager) CreateSession(ctx context.Context, req CreateSessionRequest) (*db.Session, error)

// SendCommand: route command to session's tmux
func (sm *SessionManager) SendCommand(sessionID, text string) error

// GetOutput: get recent output from session's terminal
func (sm *SessionManager) GetOutput(sessionID string, since time.Time) ([]OutputEntry, error)

// SetTakeover: toggle human takeover mode
func (sm *SessionManager) SetTakeover(sessionID string, takeover bool) error

// DestroySession: kill tmux session, update DB
func (sm *SessionManager) DestroySession(sessionID string) error
```

### Step 2: Agent startup
When creating a session:
1. Look up agent config from registry
2. Create tmux session with `tmux.Manager.CreateSession()`
3. If worktree exists, cd to worktree path
4. Send agent's `command` to the tmux session
5. If agent has `auto_accept_mode`, send the key sequence after a short delay

### Step 3: Session monitoring
```go
type Monitor struct {
    sessionID    string
    lastOutput   time.Time
    lastStatus   string
    outputBuffer *RingBuffer  // Last N output lines
}

// Run as goroutine per session
func (m *Monitor) Run(ctx context.Context) {
    // Every 1s:
    // 1. Check tmux session still exists
    // 2. Update last_activity_at in DB
    // 3. Detect status changes:
    //    - No output for 30s → idle
    //    - Prompt detected → waiting_review
    //    - tmux session gone → completed
    // 4. Broadcast status change events
}
```

### Step 4: Idle/completion detection
Priority-based detection (from SPEC 5.3):
1. **Shell prompt detection**: Parser already does this — if last message is class "prompt" and has shell prompt pattern
2. **Idle timeout**: No output for 30s
3. **Marker file**: Check if `.orchestra/done` exists in worktree
4. **Git commit**: Check for commits with `[READY_FOR_REVIEW]` in message

### Step 5: Human takeover
When `SetTakeover(true)`:
- Update session.human_attached = true in DB
- Update session.status = "human_takeover"
- Broadcast status change to all connected clients
- The orchestrator (when implemented) will check this flag and skip the session

When `SetTakeover(false)`:
- Reset human_attached and status
- Broadcast status change

### Step 6: Output storage
- Maintain a ring buffer (last 500 lines) per active session
- Feed from Gateway events through Parser
- `GetOutput(since)` returns entries after the given timestamp

## Testing Requirements
- Test session creation with valid/invalid agent type
- Test session creates real tmux session with correct name
- Test command sending reaches the tmux session
- Test idle detection triggers after timeout
- Test session destruction kills tmux session
- Test human takeover flag propagation

## Acceptance Criteria
- [ ] Creating a session spawns a real tmux session with the agent command running
- [ ] Session status updates in real-time (working/idle/waiting)
- [ ] Commands sent via API appear in the tmux session
- [ ] Recent output retrievable via API
- [ ] Idle detection works (30s timeout)
- [ ] Human takeover flag prevents orchestrator interference
- [ ] Session destruction kills tmux session and updates DB

## Notes
- Session names must be unique within tmux (use UUID suffix if needed)
- Monitor goroutines should be cancellable (context-based)
- Ring buffer size should be configurable
- Consider: what if the agent exits immediately? Detect and report as failed.
- The output capture needs to handle both parsed (chat mode) and raw (terminal mode) data
