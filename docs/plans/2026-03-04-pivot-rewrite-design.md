# agenTerm v2 — Human Orchestrator Rewrite Design

**Date:** 2026-03-04
**Base commit:** `ecb98a7` on `main`
**Related docs:** `docs/2026-03-04-pivot-new-version.md`, `docs/2026-03-04-codebase-audit.md`, `docs/2026-03-04-onboarding-flow.md`

---

## 1. Migration Strategy

**Approach: Branch-and-Gut**

1. Create `v1-archive` branch from current `main` to preserve the old version.
2. On `main`, delete packages we're dropping, simplify the DB schema via migration, rewrite the frontend in-place with Tailwind.
3. Keep the Go module, build system, Tauri shell, and proven packages intact.

Git history is preserved for kept code. Build system, CI, Tauri config, go.mod all carry forward.

---

## 2. Backend Architecture

### Packages to Keep (as-is)

| Package | Purpose |
|---|---|
| `internal/pty` | PTY terminal backend |
| `internal/session` | Session lifecycle, command queue, safety policy |
| `internal/hub` | WebSocket hub, broadcast/subscription |
| `internal/db` | SQLite migrations, repositories |
| `internal/registry` | YAML-backed agent registry |
| `internal/parser` | Output classification + event detection |
| `internal/server` | HTTP server + embedded SPA |

### Packages to Delete

| Package | Reason |
|---|---|
| `internal/orchestrator` | LLM autonomous loop — replaced by human decisions |
| `internal/automation` | Autocommit/coordinator/merger loops — replaced by human-triggered actions |
| `internal/playbook` | YAML playbook schema — no longer needed |
| `internal/tmux` | Legacy, superseded by PTY |

### New Package

**`internal/scaffold/`** — Worktree setup automation (pure file I/O, no LLM):
- Create worktree + branch from blueprint
- Generate CLAUDE.md/AGENTS.md from project template + task spec + completion criteria
- Write agent-specific permission configs (`.claude/settings.json`, `.codex/rules/`, `opencode.json`, etc.)
- Cleanup worktree + branch on merge completion

### API Changes

**Remove handlers:** `orchestrator.go`, `orchestrator_governance.go`, `orchestrator_exceptions.go`, `playbooks.go`, `asr.go`, `asr_volc.go`

**Add handlers:**
- `requirements.go` — CRUD, reorder, status transitions
- `planning.go` — create planning session (links requirement to agent TUI session), save blueprint
- `execution.go` — one-click setup (create worktrees + CLAUDE.md + permissions + spawn sessions), stage transitions
- `onboarding.go` — agent team setup, permission template management

**Modify:**
- `sessions.go` — add agent capacity view endpoint
- `projects.go` — add context template and knowledge fields
- `run_state.go` — simplify to human-triggered transitions, add event-driven hooks (parser detects `[READY_FOR_REVIEW]`, `[BLOCKED]`)

### Event-Driven Automation (not AI orchestration)

The parser + a lightweight event handler detect signals from agent output/commits:
- `[READY_FOR_REVIEW]` in commit → mark task as review-ready, show alert
- `[BLOCKED]` in commit or BLOCKED.md created → mark task as blocked, show alert
- Agent process exit → update session status
- App restart → resume suspended sessions via agent resume commands

This is a deterministic state machine, not LLM orchestration.

---

## 3. Data Model

### Tables to Keep (adapt)

| Table | Changes |
|---|---|
| `projects` | Add `context_template` (CLAUDE.md template), `knowledge` (project summary) |
| `tasks` | Add `requirement_id` FK, keep completion criteria |
| `worktrees` | As-is |
| `sessions` | As-is (already has suspend/resume/terminate) |
| `session_commands` | As-is |
| `project_knowledge_entries` | Keep — stores `/init` output, feeds CLAUDE.md generation |
| `review_cycles` | Keep — tracks build→review transitions |
| `review_issues` | Keep — tracks issues found during review |
| `demand_pool_items` | Simplify to requirement seed queue |
| `project_runs` | Keep — tracks current requirement cycle |
| `stage_runs` | Keep — tracks build/review/merge/test stages |

### Tables to Drop

| Table | Reason |
|---|---|
| `orchestrator_messages` | LLM chat history |
| `workflows` / `workflow_phases` | AI workflow logic |
| `project_orchestrators` | AI orchestrator config |
| `role_bindings` | AI role assignment resolution |
| `role_agent_assignments` | AI-decided assignments |
| `role_loop_attempts` | AI retry tracking |

### New Tables

```sql
-- Prioritized queue of things to build
CREATE TABLE requirements (
    id              TEXT PRIMARY KEY,
    project_id      TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    description     TEXT,
    priority        INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'queued',
        -- 'queued' | 'planning' | 'ready' | 'building' | 'reviewing' | 'testing' | 'done'
    created_at      DATETIME NOT NULL,
    updated_at      DATETIME NOT NULL
);

-- Links a requirement to a planner agent session and stores the output blueprint
CREATE TABLE planning_sessions (
    id                TEXT PRIMARY KEY,
    requirement_id    TEXT NOT NULL REFERENCES requirements(id) ON DELETE CASCADE,
    agent_session_id  TEXT REFERENCES sessions(id) ON DELETE SET NULL,
    status            TEXT NOT NULL DEFAULT 'active',
        -- 'active' | 'completed'
    blueprint         TEXT,
        -- JSON: { tasks: [...], worktrees: [...], parallel_groups: [...] }
    created_at        DATETIME NOT NULL,
    updated_at        DATETIME NOT NULL
);

-- Per-agent permission configs (standard/strict/permissive)
CREATE TABLE permission_templates (
    id          TEXT PRIMARY KEY,
    agent_type  TEXT NOT NULL,
    name        TEXT NOT NULL,
        -- 'standard' | 'strict' | 'permissive'
    config      TEXT NOT NULL,
        -- JSON: { allow: [...], deny: [...] }
    created_at  DATETIME NOT NULL,
    updated_at  DATETIME NOT NULL
);
```

### Agent Capacity

No recommendation tables needed. The user sees capacity from existing data:
- `agent_configs.max_parallel` — how many slots
- `COUNT(sessions WHERE agent_type = X AND status IN ('working', ...))` — how many in use

User picks agents manually from this capacity view.

---

## 4. Frontend Architecture

### Tech Stack
- React 18 + TypeScript + Vite
- xterm.js (terminal rendering)
- Tailwind CSS (replaces 2200-line global stylesheet)
- Tauri webview (desktop shell)

### Layout Structure

```
┌─────────────────────────┬──────────────────────────────────────────┐
│ LEFT SIDEBAR (foldable) │  MAIN CONTENT AREA                      │
│                         │                                          │
│ ┌─────────────────────┐ │  Mode switch: [Workspace] [Demands] [⚙] │
│ │ PROJECTS            │ │                                          │
│ │ ▸ my-api  2🟢 1🔴   │ │  (content depends on selected mode)     │
│ │   my-cli  0🟢       │ │                                          │
│ │ [+ New Project]     │ │                                          │
│ ├─────────────────────┤ │                                          │
│ │ AGENTS              │ │                                          │
│ │ claude  1/2 working │ │                                          │
│ │ codex   2/2 working │ │                                          │
│ │ kimi    0/1 idle    │ │                                          │
│ ├─────────────────────┤ │                                          │
│ │ [⚙ Settings]        │ │                                          │
│ └─────────────────────┘ │                                          │
└─────────────────────────┴──────────────────────────────────────────┘
```

### Workspace Mode

Where the user spends most time. Three sub-areas:

**Agent sidebar (left):** Planner always on top. Builders and reviewers grouped by worktree. Status indicators (🟢 working, 🔴 needs response, ⚫ idle). Click to select → shows in main area.

**Status graph (top):** Shows the current demand's pipeline: `[Plan ✓] → [Build ◉] → [Review] → [Test]`. Under build stage, shows per-worktree status. Nodes are clickable → jumps to that agent's terminal.

**Main content (center):** Per-agent view with three modes toggled by `[TUI] [MD] [Split]`:
- **TUI:** Full xterm.js terminal for that agent's session
- **MD:** File tree (markdown files only in agent's worktree) + editor pane. User can edit CLAUDE.md, BLOCKED.md, plan.md, review docs directly.
- **Split:** TUI left, MD right

**Empty state (new project):** Centered input — "What do you want to build?" Creates first demand and starts planning flow.

### Demand Pool Mode

Simple and focused:
- Input frame at top to add new demands
- Table below: all demands, drag to reorder, status badges (`queued`, `building`, `done`)
- Active demands: click → jump to workspace
- Completed demands: expandable → shows status graph + linked sessions (history view)

### Project Settings Mode

- Project name
- Context template editor (the CLAUDE.md/AGENTS.md template for this project)
- Project knowledge viewer (codebase summary from `/init`)

### Onboarding Wizard (first-run)

Full-screen modal, step-focused per `docs/2026-03-04-onboarding-flow.md`:
1. **Set Up Your AI Team** — agent checklist with sensible defaults (command, parallel slots, roles)
2. **Permission Templates** — standard/strict/permissive per agent, preview + customize
3. **Done** — lands on empty workspace with "What do you want to build?" input

### Components

| Component | Purpose |
|---|---|
| `AppSidebar.tsx` | Project list, agent capacity, settings link, foldable |
| `Workspace.tsx` | Main workspace layout: agent sidebar + status graph + content area |
| `AgentSidebar.tsx` | Planner + builders/reviewers grouped by worktree, status indicators |
| `StatusGraph.tsx` | Pipeline visualization with clickable nodes |
| `AgentView.tsx` | TUI/MD/Split toggle, xterm.js terminal, markdown editor |
| `MarkdownPane.tsx` | Worktree .md file tree + editor |
| `DemandPool.tsx` | Input + table with reorder, status, expand to history |
| `ProjectSettings.tsx` | Name, context template editor, knowledge viewer |
| `OnboardingWizard.tsx` | 3-step first-run flow |
| `ExecutionSetup.tsx` | Blueprint review + agent assignment + "Launch All" |
| `NewProjectModal.tsx` | Folder picker + name input |
| `SettingsModal.tsx` | Agent registry, permission templates |

### Dropped Components

- `OrchestratorPanel.tsx`, `ChatPanel.tsx` — no LLM chat
- `MobileCompanion.tsx`, `ConnectModal.tsx` — cut for MVP

---

## 5. End-to-End User Journey

### First Launch
1. Onboarding wizard: pick agents → set permissions → done
2. Lands on empty workspace with project creation prompt

### Create Project
1. "+ New Project" → modal: select folder, set name
2. Empty workspace: "What do you want to build?"

### Add Demands
1. User types requirement → saved as `queued` in demand pool
2. Can add multiple, drag to reorder

### Planning
1. User clicks a demand → status becomes `planning`
2. System spawns planner agent session in project's repo
3. User interacts with planner via TUI
4. Planner outputs blueprint (task breakdown, worktree structure, completion criteria)
5. User reviews/edits blueprint in MD pane, confirms

### One-Click Execution
1. User clicks "Launch" on confirmed blueprint
2. `internal/scaffold/` creates worktrees, writes CLAUDE.md + permission configs, spawns agent sessions
3. Status graph appears: `[Plan ✓] → [Build ◉] → [Review] → [Test]`
4. Agents work autonomously

### Monitor & Intervene
1. Status graph shows per-worktree progress
2. 🔴 alerts when agent needs attention (blocked, review-ready, error)
3. User clicks agent → TUI or MD view → take over, answer questions, edit docs

### Stage Transitions
1. **Build → Review:** Parser detects `[READY_FOR_REVIEW]` or user clicks "Start Review". System spawns reviewer agents on completed worktrees.
2. **Review → Merge:** User clicks "Merge". System merges worktree branches.
3. **Merge → Test:** User clicks "I'll Test Now". Human does integration testing.
4. **Test → Done:** User clicks "Mark Done". Demand → `done`. Worktrees cleaned up.

### Next Cycle
1. Demand pool → pick next item → start planning again

---

## 6. Session Persistence

Sessions are persisted in SQLite with full state. On app restart:
- Session manager loads all non-terminal sessions from DB
- For agents that support resume (e.g. Claude Code `--continue`): re-spawn with resume command
- For agents that don't: mark as terminated, user can manually restart
- PTY session ID mapping maintained in DB across restarts

---

## 7. Best Practices Integration

Best practices are embedded in the system, not a separate UI:
- **CLAUDE.md/AGENTS.md auto-generation:** per worktree, from project template + task spec + rules
- **Permission configs:** written into worktrees during scaffold
- **Auto-compact rules:** included in generated CLAUDE.md
- **Commit conventions:** enforced via CLAUDE.md instructions (`[agenterm:task-XXX]` prefix)
- **Plan mode:** instructed in CLAUDE.md ("Start by creating a plan")
- **TDD mode:** optional, included in CLAUDE.md when selected
- **Project knowledge:** cached from initial `/init`, injected into all future CLAUDE.md files

The onboarding wizard is the only place the user configures these. After that, they're automatic.
