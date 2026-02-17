# Task: database-models

## Context
AgenTerm currently stores everything in memory. The SPEC requires persistent storage for projects, tasks, worktrees, sessions, and agent configs. SQLite is the chosen database (lightweight, single-file, no extra services). This task establishes the data foundation that all subsequent features depend on.

Tech stack: Go 1.22, nhooyr.io/websocket. Add `modernc.org/sqlite` (pure-Go SQLite driver, no CGO needed).

## Objective
Add SQLite database with full schema for Project, Task, Worktree, Session, and Agent models. Include a repository layer with CRUD operations for each model.

## Dependencies
- Depends on: none
- Branch: feature/database-models
- Base: main

## Scope

### Files to Create
- `internal/db/db.go` — Database initialization, connection management, migration runner
- `internal/db/migrations.go` — SQL migration definitions (embedded strings)
- `internal/db/models.go` — Go struct definitions for all data models
- `internal/db/project_repo.go` — Project CRUD repository
- `internal/db/task_repo.go` — Task CRUD repository
- `internal/db/worktree_repo.go` — Worktree CRUD repository
- `internal/db/session_repo.go` — Session CRUD repository
- `internal/db/agent_repo.go` — Agent config repository (reads YAML + stores in DB)

### Files to Modify
- `go.mod` — Add `modernc.org/sqlite` dependency
- `internal/config/config.go` — Add `DBPath` config field (default: `~/.config/agenterm/agenterm.db`)
- `cmd/agenterm/main.go` — Initialize DB on startup, pass to components that need it

### Files NOT to Touch
- `internal/tmux/` — No changes to tmux gateway
- `internal/hub/` — No changes to WebSocket hub
- `internal/parser/` — No changes to parser
- `web/` — No frontend changes

## Implementation Spec

### Step 1: Define Go models
```go
// internal/db/models.go
type Project struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    RepoPath  string    `json:"repo_path"`
    Status    string    `json:"status"` // active | paused | archived
    Playbook  string    `json:"playbook,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type Task struct {
    ID          string    `json:"id"`
    ProjectID   string    `json:"project_id"`
    Title       string    `json:"title"`
    Description string    `json:"description"`
    Status      string    `json:"status"` // pending | running | reviewing | done | failed | blocked
    DependsOn   []string  `json:"depends_on"`
    WorktreeID  string    `json:"worktree_id,omitempty"`
    SpecPath    string    `json:"spec_path,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type Worktree struct {
    ID         string `json:"id"`
    ProjectID  string `json:"project_id"`
    BranchName string `json:"branch_name"`
    Path       string `json:"path"`
    TaskID     string `json:"task_id,omitempty"`
    Status     string `json:"status"` // active | merged | abandoned
}

type Session struct {
    ID              string    `json:"id"`
    TaskID          string    `json:"task_id,omitempty"`
    TmuxSessionName string   `json:"tmux_session_name"`
    TmuxWindowID    string    `json:"tmux_window_id,omitempty"`
    AgentType       string    `json:"agent_type"`
    Role            string    `json:"role"` // coder | reviewer | coordinator
    Status          string    `json:"status"` // idle | running | waiting_review | human_takeover | completed
    HumanAttached   bool      `json:"human_attached"`
    CreatedAt       time.Time `json:"created_at"`
    LastActivityAt  time.Time `json:"last_activity_at"`
}

type AgentConfig struct {
    ID                    string   `json:"id"`
    Name                  string   `json:"name"`
    Command               string   `json:"command"`
    ResumeCommand         string   `json:"resume_command,omitempty"`
    HeadlessCommand       string   `json:"headless_command,omitempty"`
    Capabilities          []string `json:"capabilities"`
    Languages             []string `json:"languages"`
    CostTier              string   `json:"cost_tier"`
    SpeedTier             string   `json:"speed_tier"`
    SupportsSessionResume bool     `json:"supports_session_resume"`
    SupportsHeadless      bool     `json:"supports_headless"`
    AutoAcceptMode        string   `json:"auto_accept_mode,omitempty"`
}
```

### Step 2: Create migration system
- Use a simple version-based approach: track `schema_version` in a `_meta` table
- Migration 001: Create all tables (projects, tasks, worktrees, sessions, agent_configs)
- Tasks.depends_on stored as JSON array string
- AgentConfig.capabilities and .languages stored as JSON array strings
- All IDs are UUIDs generated with `crypto/rand`

### Step 3: Implement repository layer
Each repo should have:
- `Create(ctx, model) error`
- `Get(ctx, id) (*Model, error)`
- `List(ctx, filters) ([]*Model, error)`
- `Update(ctx, model) error`
- `Delete(ctx, id) error`

Plus model-specific queries:
- `ProjectRepo.ListByStatus(ctx, status)`
- `TaskRepo.ListByProject(ctx, projectID)`
- `TaskRepo.ListByStatus(ctx, projectID, status)`
- `SessionRepo.ListByTask(ctx, taskID)`
- `SessionRepo.ListActive(ctx)`
- `WorktreeRepo.ListByProject(ctx, projectID)`

### Step 4: Wire into main.go
- Open DB connection in main()
- Run migrations on startup
- Pass DB to server (for later API use)
- Close DB on shutdown

## Testing Requirements
- Test migration runs without error on empty DB
- Test CRUD operations for each model
- Test list with filters
- Test UUID generation uniqueness
- Test DB file is created at configured path

## Acceptance Criteria
- [ ] SQLite database created on first startup
- [ ] All 5 tables created with correct schema
- [ ] CRUD operations work for all models
- [ ] Migrations are idempotent (running twice is safe)
- [ ] DB path configurable via config file
- [ ] JSON array fields (depends_on, capabilities) serialize/deserialize correctly

## Notes
- Use `modernc.org/sqlite` (pure Go, no CGO) to keep single-binary deployment
- Generate UUIDs with `crypto/rand` + hex encoding, no external UUID library needed
- Store timestamps as RFC3339 strings in SQLite for readability
- The DependsOn field on Task is a JSON array stored as TEXT
