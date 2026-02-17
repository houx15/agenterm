# Task: multi-session-tmux

## Context
AgenTerm currently connects to a single tmux session (`ai-coding` by default) and manages windows within it. The SPEC requires managing multiple tmux sessions — one per task/agent, with naming convention `{project}-{task}-{role}`. The Gateway needs to evolve from single-session to multi-session management.

## Objective
Refactor the tmux Gateway to support creating, attaching to, and managing multiple tmux sessions simultaneously. Each "agent session" in the system maps to its own tmux session (not just a window within one session).

## Dependencies
- Depends on: TASK-07 (database-models), TASK-08 (rest-api)
- Branch: feature/multi-session-tmux
- Base: main (after TASK-08 merge)

## Scope

### Files to Create
- `internal/tmux/manager.go` — Multi-session manager: create/destroy/list tmux sessions, manage multiple Gateway instances

### Files to Modify
- `internal/tmux/gateway.go` — Refactor to support being one-of-many (each Gateway instance manages one tmux session). Add `SessionName()` accessor. Make `Close()` cleaner for multi-instance use.
- `internal/hub/hub.go` — Route messages to correct Gateway based on session ID. Support multiple terminal streams.
- `internal/hub/client.go` — Include session identifier in terminal_input messages
- `internal/hub/protocol.go` — Add session_id field to terminal messages
- `cmd/agenterm/main.go` — Use Manager instead of single Gateway. Maintain backward compatibility (still auto-attach to default session).
- `web/index.html` — Update frontend to handle session-scoped messages (session_id in terminal_data and output messages)

### Files NOT to Touch
- `internal/parser/` — Parser already works per-window, just needs window→session mapping
- `internal/config/` — No config changes needed

## Implementation Spec

### Step 1: Create TmuxManager
```go
type Manager struct {
    gateways   map[string]*Gateway   // tmux_session_name → Gateway
    mu         sync.RWMutex
    defaultDir string
}

func NewManager(defaultDir string) *Manager
func (m *Manager) CreateSession(name string, workDir string) (*Gateway, error)
func (m *Manager) AttachSession(name string) (*Gateway, error)
func (m *Manager) DestroySession(name string) error
func (m *Manager) GetGateway(name string) (*Gateway, error)
func (m *Manager) ListSessions() []string                    // tmux list-sessions
func (m *Manager) Close()                                    // Close all gateways
```

### Step 2: Session creation flow
1. `tmux new-session -d -s <name> -c <workDir>` — create detached session
2. Create new Gateway instance attached to this session
3. Start event processing loop for this Gateway
4. Register Gateway in manager map

### Step 3: Refactor Hub for multi-session
- Messages now carry `session_id` (the tmux session name)
- `BroadcastTerminal(sessionID, windowID, data)` — route to correct clients
- Client can subscribe to specific sessions or all
- Maintain per-session output buffers

### Step 4: Backward compatibility
- On startup, auto-attach to the configured default session (current behavior)
- The default session Gateway works exactly as before
- New sessions created via API go through Manager
- Frontend detects whether it's in "legacy single-session" or "multi-session" mode

### Step 5: Frontend updates
- Add `session_id` to all inbound/outbound messages
- Sidebar groups windows by session
- Terminal input routes to the correct session

## Testing Requirements
- Test creating multiple tmux sessions programmatically
- Test destroying a session cleans up Gateway
- Test messages route to correct session
- Test backward compatibility (single-session mode still works)

## Acceptance Criteria
- [ ] Can create new tmux sessions via Manager
- [ ] Can attach to existing tmux sessions
- [ ] Multiple Gateways run concurrently without interference
- [ ] Hub routes messages to correct session
- [ ] Destroying a session kills the tmux session and closes the Gateway
- [ ] Default session auto-attach still works (backward compatible)

## Notes
- Each Gateway runs its own goroutine for reading tmux control mode output
- Use `tmux has-session -t <name>` to check if session exists before creating
- Session naming: `{project}-{task}-{role}` (e.g., `myapp-auth-coder`)
- Consider connection pooling — don't spawn too many tmux -C processes
- tmux has a limit on control mode clients; may need to share one control connection
