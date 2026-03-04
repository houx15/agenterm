# Feature Spec: AI Genesis

## Summary
AI Genesis is a project-scoped automation feature that can run an AI/TUI worker loop to modify code in a project, validate changes, and optionally restart the project service after successful gates.

## Product Direction
- Scope: **project-level**, not global singleton.
- Default mode: **manual run** (user-triggered).
- Optional mode: **watch mode** (continuous loop) behind explicit enable.

## Why
- Turn PM/orchestrator intent into direct code changes with fewer manual steps.
- Keep safety and auditability through staged execution and explicit approvals.
- Reuse existing orchestrator, worktree, session, and review infrastructure.

## Goals
1. Start a Genesis run for one project with a user goal.
2. Execute controlled loop: plan -> implement -> validate -> commit -> merge(optional) -> restart(optional).
3. Persist run history and action ledger for audit/debug.
4. Provide stop/pause controls and human intervention path.

## Non-Goals (MVP)
- Autonomous multi-project global daemon.
- Unbounded self-modifying loops.
- Bypassing review/test gates.

## UX

### Project Settings: AI Genesis
Fields:
- `enabled`: boolean
- `mode`: `manual | watch`
- `restart_command`: string (optional)
- `auto_merge`: boolean
- `max_iterations`: int
- `max_parallel`: int

### PM Chat / Project Actions
Buttons:
- `Run Genesis`
- `Stop Genesis`
- `View Run Log`

### Dashboard
- Per-project Genesis badge: `idle | running | blocked | failed | completed`
- Last run summary snippet

## Backend Design

### Data Model

#### `project_genesis_configs`
- `project_id` (PK/FK)
- `enabled`
- `mode`
- `restart_command`
- `auto_merge`
- `max_iterations`
- `max_parallel`
- `created_at`
- `updated_at`

#### `genesis_runs`
- `id` (PK)
- `project_id` (FK)
- `status` (`queued|running|blocked|failed|completed|stopped`)
- `goal`
- `iteration`
- `started_at`
- `ended_at`
- `summary`
- `error`

#### `genesis_actions`
- `id` (PK)
- `run_id` (FK)
- `kind` (`allocation|session|command|output|check|commit|merge|restart|approval`)
- `payload` (JSON)
- `created_at`

### API
- `GET /api/projects/{id}/genesis`
- `PATCH /api/projects/{id}/genesis`
- `POST /api/projects/{id}/genesis/runs`
- `POST /api/genesis-runs/{id}/stop`
- `GET /api/projects/{id}/genesis/runs`
- `GET /api/genesis-runs/{id}/actions`

### Runtime Flow (MVP)
1. User submits goal via `Run Genesis`.
2. Create run record (`queued -> running`).
3. Orchestrator applies stage-gated workflow:
   - allocate agents/models
   - create worktree(s)
   - create session(s)
   - dispatch prompts/commands
   - read outputs
   - run quality checks
   - commit changes
   - merge if enabled and gates pass
   - restart service if configured
4. Persist every step into `genesis_actions`.
5. Finalize run (`completed|blocked|failed|stopped`).

## Orchestrator Skills
Reuse (progressive disclosure):
- model-allocation
- session-bootstrap
- prompt-dispatch
- session-output-reading
- session-teardown
- worktree-planning
- git-integration
- stage-gates-and-approval
- capacity-scheduler
- quality-gates-and-review
- conflict-resolution-loop
- project-memory-management

Add:
- `genesis-runner` skill for loop contract and termination policy.

## Safety And Guardrails
1. Explicit approval required before mutating execution.
2. One active run per project.
3. Iteration/time budget limits.
4. Stop button must preempt loop quickly.
5. Restart command runs with timeout and captured output.
6. No auto-merge/restart when quality gates fail.
7. Human takeover path for blocked/conflict states.

## Restart Policy
- Project-configured command only (`restart_command`).
- Run after checks/merge success according to config.
- Capture stdout/stderr and exit code in action log.

## Observability
- Run status + counters exposed in API report.
- WS events for run lifecycle and blocking conditions.
- Action ledger visible in UI run log.

## Rollout Plan

### Slice 1
- Schema + repos for genesis config/run/action.
- Config + run start/stop/list APIs.
- UI settings scaffold + Run/Stop buttons.

### Slice 2
- Manual run engine with worktree/session/prompt/output loop.
- Action ledger persistence.

### Slice 3
- Commit/merge/restart integration with gates.
- Blocked-state handling and recovery prompts.

### Slice 4
- Watch mode.
- Dashboard analytics and richer run insights.

## Acceptance Criteria
1. User can run Genesis from a project and see live status transitions.
2. All mutating actions are approval-gated and logged.
3. Run can be stopped safely.
4. Commit/merge/restart only happen after required checks pass.
5. Run history and action logs are queryable and visible in UI.
