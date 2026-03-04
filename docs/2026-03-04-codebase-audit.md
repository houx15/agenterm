# agenTerm — Codebase Audit (Pre-Pivot Snapshot)

**Date:** 2026-03-04
**Commit:** `22eb2eb` (chore(docs): archive pre-pivot documentation)
**Full SHA:** `22eb2eb5fb88cbf39a5be8d4e352fc7aba36025a`
**Branch:** `main`

---

## 1. Project Structure

| Path | Purpose |
|---|---|
| `cmd/agenterm` | App entrypoint and runtime wiring |
| `internal/api` | REST handlers, auth/CORS/JSON middleware, endpoint routing |
| `internal/db` | SQLite models, migrations, repositories |
| `internal/session` | Session lifecycle, command queue, output/idle/ready monitoring, command safety policy |
| `internal/pty` | PTY terminal backend (active runtime) |
| `internal/hub` | WebSocket hub, broadcast/subscription protocol |
| `internal/parser` | Terminal output parsing/classification + quick actions |
| `internal/orchestrator` | LLM orchestrator loop, prompting, tool execution, scheduler, event triggers |
| `internal/automation` | Auto-commit, reviewer coordinator, merge controller loops |
| `internal/registry` | YAML-backed agent registry |
| `internal/playbook` | YAML-backed playbook registry/schema validation |
| `internal/tmux` | Legacy tmux code (not the main runtime path) |
| `frontend` | React/TS UI (workspace, terminals, orchestrator panel, mobile companion) |
| `web` | Embedded frontend assets for Go binary |
| `configs/agents` | Agent YAML definitions |
| `configs/playbooks` | Playbook YAML definitions |
| `src-tauri` | Tauri desktop shell (starts/stops backend sidecar) |

## 2. Tech Stack

- **Backend:** Go 1.22, stdlib HTTP, SQLite (`modernc.org/sqlite`), WebSocket (`nhooyr.io/websocket`), YAML (`gopkg.in/yaml.v3`)
- **Frontend:** React 18 + TypeScript + Vite + xterm.js + lucide-react
- **Desktop:** Rust + Tauri 1
- **Build:** Go binary with embedded SPA via `web/embed.go`

## 3. Core Components and Interactions

- **Bootstrap:** `cmd/agenterm/main.go` — config → DB → registries → PTY backend → hub → session manager → orchestrators → automation loops → API router → HTTP server
- **HTTP/WebSocket:** `internal/server/server.go` — serves embedded SPA, `/api/*`, `/ws`, `/ws/orchestrator`
- **API routing:** `internal/api/router.go` — handler logic split by resource files
- **Session runtime:** lifecycle/queue/ready-state in `internal/session/manager.go`, backend abstraction in `internal/session/backend.go`, PTY execution in `internal/pty/backend.go`
- **Real-time stream:** PTY events → parser → hub broadcast → frontend WebSocket hooks
- **Orchestrator:** LLM loop in `internal/orchestrator/orchestrator.go`, tool catalog in `tools.go`, scheduling in `scheduler.go`, event triggers in `events.go`
- **Automation:** autocommit, coordinator, merger — all started at runtime in `main.go`

## 4. Current Features

- Project lifecycle CRUD (create/list/get/update/archive-delete)
- Task CRUD with dependency/status/spec and review-gate completion
- Git worktree create/status/log/merge/conflict resolution/delete
- Session create/list/get/delete, send keys/text, command queue, readiness/idle/output monitoring, takeover mode
- Real-time terminal streaming over WebSocket
- Output classification (prompt/error/code/normal) + quick actions
- Agent registry CRUD + runtime capacity/utilization view
- Playbook CRUD and validation
- Orchestrator chat/history/report (execution and demand lanes)
- Governance: project orchestrator profile, workflow CRUD, role bindings, assignment preview/confirm, knowledge entries
- Orchestrator exceptions collection and manual resolve tracking
- Review cycles/issues tracking with transition rules
- Run/stage state (`plan/build/test`) with transition API + event broadcast
- Demand pool CRUD/reprioritize/promote-to-task
- Directory browser API for project creation
- ASR upload/transcription (Volcengine backend)
- Mobile companion view for approvals/alerts/reporting
- Desktop shell sidecar management (Tauri)

## 5. Database Schema

Tables: `_meta`, `projects`, `tasks`, `worktrees`, `sessions`, `session_commands`, `agent_configs`, `orchestrator_messages`, `workflows`, `workflow_phases`, `project_orchestrators`, `role_bindings`, `role_agent_assignments`, `role_loop_attempts`, `project_knowledge_entries`, `review_cycles`, `review_issues`, `demand_pool_items`, `project_runs`, `stage_runs`

Key relationships:
- `tasks.project_id → projects.id` (CASCADE)
- `worktrees.project_id → projects.id` (CASCADE), `worktrees.task_id → tasks.id` (SET NULL)
- `sessions.task_id → tasks.id` (SET NULL)
- `session_commands.session_id → sessions.id` (CASCADE)
- `demand_pool_items.project_id → projects.id` (CASCADE), `demand_pool_items.selected_task_id → tasks.id` (SET NULL)
- `project_runs.project_id → projects.id` (CASCADE)
- `stage_runs.run_id → project_runs.id` (CASCADE)
- `review_cycles.task_id → tasks.id` (CASCADE), `review_issues.cycle_id → review_cycles.id` (CASCADE)

Note: `agent_configs` DB table exists but runtime CRUD uses YAML registry files instead.

## 6. Backend API Endpoints

**Projects:** POST/GET `/api/projects`, GET/PATCH/DELETE `/api/projects/{id}`
**Tasks:** POST/GET `/api/projects/{id}/tasks`, GET/PATCH `/api/tasks/{id}`
**Worktrees:** POST `/api/projects/{id}/worktrees`, GET `{id}/git-status`, GET `{id}/git-log`, POST `{id}/merge`, POST `{id}/resolve-conflict`, DELETE `{id}`
**Sessions:** POST `/api/tasks/{id}/sessions`, GET/GET/DELETE `/api/sessions/{id}`, POST `{id}/send`, POST `{id}/send-key`, POST/GET `{id}/commands`, GET `{id}/output`, GET `{id}/idle`, GET `{id}/ready`, GET `{id}/close-check`, PATCH `{id}/takeover`
**Agents:** GET `/api/agents`, GET `/api/agents/status`, GET/POST/PUT/DELETE `/api/agents/{id}`
**Playbooks:** GET/POST `/api/playbooks`, GET/PUT/DELETE `/api/playbooks/{id}`
**Run state:** GET `/api/projects/{id}/runs/current`, POST `/api/projects/{id}/runs/{run_id}/transition`
**Demand pool:** GET/POST `/api/projects/{id}/demand-pool`, POST `{id}/demand-pool/reprioritize`, GET/PATCH/DELETE `/api/demand-pool/{id}`, POST `{id}/promote`
**Orchestrator:** POST/GET `/api/orchestrator/chat|history|report`, POST/GET `/api/demand-orchestrator/chat|history|report`
**Governance:** GET/PATCH `/api/projects/{id}/orchestrator`, POST `assignments/preview|confirm`, GET `assignments|exceptions`
**Review:** GET/POST `/api/tasks/{id}/review-cycles`, PATCH `/api/review-cycles/{id}`, GET/POST `{id}/issues`
**Other:** GET `/api/fs/directories`, POST `/api/asr/transcribe`, GET/PUT `/api/settings`
**WebSocket:** `/ws`, `/ws/orchestrator`

## 7. Frontend Architecture

- Single-root app, path-based split: `/mobile` → mobile companion, otherwise → workspace
- Workspace: 3-pane shell (sidebar + terminal grid + orchestrator panel)
- Terminal: xterm.js rendering/delta replay/resize in `Terminal.tsx`, pane management in `TerminalGrid.tsx`
- Orchestrator UI: chat + stage pipeline + demand pool + exceptions in `OrchestratorPanel.tsx`
- Settings: project creation flow, agent registry, ASR, appearance in `SettingsModal.tsx`
- Styling: single global stylesheet (~2200 lines) in `workspace.css`

## 8. What Works Well (Keep for Pivot)

- **Session/PTY runtime** — clean backend abstraction, lifecycle management, command safety policy
- **WebSocket hub** — straightforward, resilient (reconnect, subscriptions, broadcast filtering)
- **Agent registry** — YAML-backed, runtime capacity view
- **Run/stage state model** — `project_runs` + `stage_runs` already supports explicit human-triggered transitions
- **Worktree management** — path safety, git operations
- **Database layer** — migrations, repos, well-structured
- **Test coverage** — 216 tests across critical packages (orchestrator 42, session 35, api 31)
- **Review gate integrity** — blocks completion with open issues

## 9. What Needs Rethinking (Pivot Targets)

| Area | Current State | Needed for Pivot |
|---|---|---|
| `internal/orchestrator/` | LLM autonomous plan/execute loop | Deterministic execution controller, human triggers every action |
| `internal/automation/` | Continuous autocommit/coordinator/merger loops | Manual, user-invoked operations (or explicit opt-in) |
| Frontend center | OrchestratorPanel chat UI | Human control board: requirement pool, planning Q&A, one-click setup, stage controls, monitor board |
| Data model | Demand pool (simple queue) | Full requirement → planning session → execution blueprint → cycle history |
| Governance model | AI-specific role bindings/workflow resolution | Simplified to visible human decisions only |
| Stage transitions | Partly supplemented by auto-derivation/AI flows | All transitions must be human-confirmed, auditable first-class UI actions |
| Legacy code | tmux package still exists alongside PTY runtime | Consolidate around PTY model |
| Security | Plaintext API keys in agent YAMLs, permissive CORS | Secret externalization, stricter auth |
| API/UI contracts | Mismatches (e.g. `playbook_id` vs `playbook`) | Align as part of pivot hardening |
