# AgenTerm v2 Implementation Plan

## Current State (v1 - Complete)

AgenTerm v1 is a functional web-based terminal manager:
- **Go backend**: tmux Control Mode gateway, WebSocket hub, output parser
- **Frontend**: Single HTML file with chat UI + xterm.js raw terminal
- **Capabilities**: Single tmux session, window management, message classification, action buttons
- **Infra**: Token auth, config file, embedded frontend, Tailscale-ready

## Target State (v2 - SPEC.md)

Full cross-ecosystem AI agent orchestrator with:
- Multi-project/task/session management with SQLite persistence
- REST API for programmatic control
- LLM-based Orchestrator (AI Project Manager)
- Git worktree isolation for parallel agent work
- Agent Registry + Playbook system
- Auto-commit, idle detection, coder↔reviewer coordination
- React frontend with Dashboard, PM Chat, Session Terminal views

---

## Feature Breakdown (execution order)

### Phase 1: Data Foundation

| # | Task | Spec File | Depends On | Complexity |
|---|------|-----------|------------|------------|
| 7 | **SQLite Database + Models** | TASK-07-database-models.md | none | M |
| 8 | **REST API Layer** | TASK-08-rest-api.md | TASK-07 | L |

**Goal**: Persistent storage + HTTP API for all entities (Project, Task, Worktree, Session).

### Phase 2: Multi-Agent Infrastructure

| # | Task | Spec File | Depends On | Complexity |
|---|------|-----------|------------|------------|
| 9 | **Agent Registry** | TASK-09-agent-registry.md | TASK-08 | S |
| 10 | **Multi-Session Tmux** | TASK-10-multi-session-tmux.md | TASK-07, TASK-08 | L |
| 11 | **Worktree Management** | TASK-11-worktree-management.md | TASK-07, TASK-08 | M |
| 12 | **Session Lifecycle** | TASK-12-session-lifecycle.md | TASK-09, TASK-10 | L |

**Goal**: Support multiple agents running in parallel, each in its own tmux session + git worktree.

### Phase 3: Frontend Migration

| # | Task | Spec File | Depends On | Complexity |
|---|------|-----------|------------|------------|
| 13 | **React Frontend Scaffold** | TASK-13-frontend-react.md | TASK-08 | M |
| 14 | **Dashboard UI** | TASK-14-dashboard-ui.md | TASK-13 | M |

**Goal**: Modern React frontend with routing, terminal component, and dashboard.

### Phase 4: Orchestrator (AI PM)

| # | Task | Spec File | Depends On | Complexity |
|---|------|-----------|------------|------------|
| 15 | **Orchestrator Core** | TASK-15-orchestrator.md | TASK-08, TASK-09, TASK-11, TASK-12 | XL |
| 17 | **PM Chat UI** | TASK-17-pm-chat-ui.md | TASK-13, TASK-15 | L |

**Goal**: LLM-based project manager that decomposes tasks, dispatches agents, monitors progress.

### Phase 5: Automation & Polish

| # | Task | Spec File | Depends On | Complexity |
|---|------|-----------|------------|------------|
| 16 | **Automation** (auto-commit, coordinator, takeover) | TASK-16-automation.md | TASK-12, TASK-11, TASK-15 | L |
| 18 | **Playbook System** | TASK-18-playbook-system.md | TASK-09, TASK-15 | M |

**Goal**: Autonomous operation — agents commit, get reviewed, and iterate with minimal human intervention.

---

## Dependency Graph

```
TASK-07 (DB)
  ├──→ TASK-08 (REST API)
  │      ├──→ TASK-09 (Agent Registry)
  │      │      └──→ TASK-12 (Session Lifecycle) ←── TASK-10
  │      ├──→ TASK-10 (Multi-Session Tmux)
  │      ├──→ TASK-11 (Worktree Mgmt)
  │      ├──→ TASK-13 (React Frontend)
  │      │      ├──→ TASK-14 (Dashboard)
  │      │      └──→ TASK-17 (PM Chat UI) ←── TASK-15
  │      └──→ TASK-15 (Orchestrator) ←── TASK-09, TASK-11, TASK-12
  │             ├──→ TASK-16 (Automation) ←── TASK-11, TASK-12
  │             └──→ TASK-18 (Playbook)
```

## Parallelism Opportunities

After TASK-07 + TASK-08 are done, these can run in parallel:
- **Track A**: TASK-09 → TASK-10 → TASK-12 (backend agent infra)
- **Track B**: TASK-11 (git worktrees, independent)
- **Track C**: TASK-13 → TASK-14 (frontend, independent of backend tracks)

After tracks converge:
- TASK-15 (orchestrator) needs A + B
- TASK-16 (automation) needs A + B + orchestrator
- TASK-17 (PM Chat) needs C + orchestrator

## Estimated Effort

| Phase | Tasks | Est. Effort |
|-------|-------|-------------|
| Phase 1: Data Foundation | 7, 8 | 1 week |
| Phase 2: Multi-Agent Infra | 9, 10, 11, 12 | 2 weeks |
| Phase 3: Frontend Migration | 13, 14 | 1-2 weeks |
| Phase 4: Orchestrator | 15, 17 | 2 weeks |
| Phase 5: Automation | 16, 18 | 1-2 weeks |
| **Total** | **12 tasks** | **~7-9 weeks** |

## Risks & Mitigations

1. **tmux Control Mode limits**: Multiple `-C` connections may not be stable. Mitigation: Could use a single control connection with multiplexed commands.
2. **LLM tool calling reliability**: Orchestrator may make poor decisions. Mitigation: Always require human confirmation for destructive actions (merge, delete).
3. **Frontend migration scope**: React migration is a full rewrite. Mitigation: Keep single-file HTML as fallback during development.
4. **Agent diversity**: Each agent CLI has different behaviors. Mitigation: Start with 2-3 well-tested agents, add more incrementally.

## Not in Scope (Future)

- Voice input (STT integration) — SPEC Section 7.5
- Skill auto-detection and installation — SPEC Section 6.1
- Token budget management — SPEC Open Question 3
- Multi-user support
- Session recording/replay
