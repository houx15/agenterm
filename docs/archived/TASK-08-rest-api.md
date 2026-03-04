# Task: rest-api

## Context
AgenTerm currently only has a WebSocket endpoint (`/ws`). The SPEC requires a full RESTful API for Project, Task, Worktree, Session, and Agent management. This API serves two consumers: the Web UI frontend and the Orchestrator (LLM agent that calls these as tools/functions).

Tech stack: Go 1.22, standard library `net/http`. No external router needed — Go 1.22's enhanced ServeMux supports method+path routing natively (`GET /api/projects/{id}`).

## Objective
Implement the complete REST API surface defined in SPEC Section 8.1, using the repository layer from TASK-07.

## Dependencies
- Depends on: TASK-07 (database-models)
- Branch: feature/rest-api
- Base: feature/database-models (or main after merge)

## Scope

### Files to Create
- `internal/api/router.go` — API route registration, middleware (auth, JSON content-type, CORS)
- `internal/api/projects.go` — Project CRUD handlers
- `internal/api/tasks.go` — Task CRUD handlers
- `internal/api/worktrees.go` — Worktree handlers (create, git-status, git-log, delete)
- `internal/api/sessions.go` — Session handlers (create, list, send command, read output, idle check, takeover)
- `internal/api/agents.go` — Agent registry handlers (list, get)
- `internal/api/response.go` — JSON response helpers, error types

### Files to Modify
- `internal/server/server.go` — Mount API router under `/api/` prefix
- `cmd/agenterm/main.go` — Create API router, pass dependencies (DB, tmux gateway, hub)

### Files NOT to Touch
- `internal/db/` — Use as-is from TASK-07
- `internal/parser/` — No changes
- `web/index.html` — No frontend changes yet

## Implementation Spec

### Step 1: Response helpers
```go
// internal/api/response.go
func jsonResponse(w http.ResponseWriter, status int, data any)
func jsonError(w http.ResponseWriter, status int, message string)
```

### Step 2: Middleware
```go
// internal/api/router.go
func authMiddleware(token string) func(http.Handler) http.Handler
func jsonMiddleware(next http.Handler) http.Handler
func corsMiddleware(next http.Handler) http.Handler
```
- Auth: Check `Authorization: Bearer <token>` header OR `?token=<token>` query param
- JSON: Set Content-Type on responses
- CORS: Allow all origins (same as current WebSocket)

### Step 3: Project handlers
```
POST   /api/projects                — Create project (name, repo_path, playbook?)
GET    /api/projects                — List projects (optional ?status= filter)
GET    /api/projects/{id}           — Get project with nested tasks/worktrees/sessions
PATCH  /api/projects/{id}           — Update project (status, playbook, name)
DELETE /api/projects/{id}           — Archive project (set status=archived)
```

### Step 4: Task handlers
```
POST   /api/projects/{id}/tasks     — Create task (title, description, depends_on?)
GET    /api/projects/{id}/tasks     — List tasks for project
GET    /api/tasks/{id}              — Get task detail with sessions
PATCH  /api/tasks/{id}              — Update task (status, description)
```

### Step 5: Worktree handlers
```
POST   /api/projects/{id}/worktrees — Create worktree (calls git worktree add)
GET    /api/worktrees/{id}/git-status — Run git status in worktree, return parsed result
GET    /api/worktrees/{id}/git-log    — Run git log in worktree, return entries
DELETE /api/worktrees/{id}           — Remove worktree (git worktree remove)
```
- Worktree creation: `git worktree add <path> -b <branch>` using os/exec
- Git status/log: Execute git commands in the worktree directory, parse output

### Step 6: Session handlers
```
POST   /api/tasks/{id}/sessions     — Create session (agent_type, role) → creates tmux window
GET    /api/sessions                — List all sessions (?status=, ?task_id=, ?project_id=)
GET    /api/sessions/{id}           — Get session detail
POST   /api/sessions/{id}/send      — Send command text to session's tmux window
GET    /api/sessions/{id}/output    — Get recent output (?since= timestamp, ?lines= count)
GET    /api/sessions/{id}/idle      — Check if session is idle (returns boolean + last_activity)
PATCH  /api/sessions/{id}/takeover  — Toggle human_takeover status
```
- Session creation should: create DB record → create tmux window → send initial cd command
- Send command: use existing Gateway.SendKeys/SendRaw
- Output: store recent output in a ring buffer per session (or read from parser)

### Step 7: Agent registry handlers
```
GET    /api/agents                  — List all configured agents
GET    /api/agents/{id}             — Get agent config detail
```
- Load from YAML files in `~/.config/agenterm/agents/` directory
- Cache in memory, reload on request or file change

### Step 8: Route registration
```go
func NewRouter(db *db.DB, gw *tmux.Gateway, hub *hub.Hub, token string) http.Handler {
    mux := http.NewServeMux()

    // Apply middleware chain
    handler := authMiddleware(token)(jsonMiddleware(mux))

    // Register all routes
    mux.HandleFunc("POST /api/projects", h.createProject)
    mux.HandleFunc("GET /api/projects", h.listProjects)
    // ... etc

    return handler
}
```

## Testing Requirements
- Test each endpoint with valid/invalid input
- Test auth middleware (missing token, wrong token, valid token)
- Test project CRUD lifecycle
- Test task creation with dependencies
- Test session creation triggers tmux window
- Test error responses (404 for missing resources, 400 for bad input)

## Acceptance Criteria
- [x] All SPEC Section 8.1 endpoints implemented
- [x] Token auth on all API routes
- [x] Proper HTTP status codes (201 for create, 200 for get/update, 204 for delete)
- [x] JSON error responses with meaningful messages
- [x] Worktree operations execute real git commands
- [x] Session creation creates real tmux windows
- [x] Existing WebSocket endpoint still works alongside new API

## Notes
- Go 1.22 ServeMux supports `"GET /api/projects/{id}"` pattern matching natively
- Keep the existing `/ws` endpoint working — the API is additive
- For session output, consider storing last N messages per window in memory (the parser already has this data)
- Git operations (worktree, status, log) should use os/exec with proper working directory
- Sanitize all user inputs before passing to shell commands (prevent command injection)
