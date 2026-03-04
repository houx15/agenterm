# TASK-19 Orchestrator Modes And Phase-Driven Execution

## Goal
Define an orchestrator model that can move a project through phases (`plan -> work -> finalize`) with human checkpoints, safe tmux command routing, and clear capacity control for worker agents.

## Why
Current orchestration can run tools and sessions, but project execution needs stronger structure:
- Different phases need different numbers/types of worker TUIs.
- Commands must be sent to specific sessions to avoid chaos.
- Phase transitions should be proposed by orchestrator and approved by user.
- Completion should be based on structured evidence, not only free-form output.

## Core Model

### 1. Project Phases
- `plan`: one planner/analyzer TUI.
- `work`: multiple worker TUIs (coder/reviewer/test roles) across worktrees.
- `finalize`: one or two TUIs for integration/release checks.
- `done`: finished.
- `blocked`: waiting for user/external dependency.

### 2. Orchestrator State Machine
State fields (per project):
- `phase`: current phase.
- `phase_status`: `running | waiting_approval | blocked | done`.
- `proposed_next_phase`: nullable.
- `proposal_payload`: worker estimate, template/playbook suggestion, rationale.
- `last_transition_at`.

### 3. Human Checkpoints
Mandatory approval points:
- `plan -> work`
- `work -> finalize`
- optional: `finalize -> done`

Approval UI action should include:
- chosen playbook/template,
- max workers,
- optional constraints/notes.

## Command Safety And tmux Routing

### Rules
- Every orchestrator command targets explicit `session_id`.
- Never rely on "active terminal" state.
- Maintain per-session command queue (serialized execution).
- Include command ledger: issued_at, command, status, result snippet.

### Expected Tools
- `create_session(task_id, agent_type, role)`
- `send_command(session_id, text)`
- `read_session_output(session_id, lines)`
- `is_session_idle(session_id)`
- `close_session(session_id)`
- `create_worktree`, `merge_worktree`, `resolve_merge_conflict`

## Agent Pool Semantics
Agent capacity should come from Agent Registry:
- `max_parallel_agents` is pool size for that agent type.
- Busy sessions consume slots.
- Idle/completed/closed sessions free slots.
- Orchestrator role sessions can be tracked separately from worker sessions.

Scheduler policy:
- only assign new work from idle slots,
- block when agent pool capacity reached,
- unblock automatically when sessions finish/idle.

## Review And Completion Criteria
Do not decide completion from plain text alone.
Use structured checks:
1. reviewer verdict artifact/tool output (`pass | changes_requested`).
2. open issues count.
3. required tests/lint checks.

Only close work session/phase when gates pass.

## Memory Model
Project memory should be managed by deterministic tools:
- `write_project_memory(project_id, kind, title, content, source)`
- `read_project_memory(project_id, query|kind|limit)`

Workers can generate content, orchestrator decides what to persist.

## UX Requirements

### PM Chat
- Normal chat for requirements.
- Add action buttons above input (e.g. `Report Progress`).
- `Report Progress` calls orchestrator report endpoint on demand (no cron).

### Transition Alert Cards
When phase change is proposed:
- show rationale,
- expected worker usage,
- suggested playbook/template,
- approve/reject buttons.

## Minimal Implementation Plan

### Slice 1
- Add project phase state storage + transition proposal endpoint.
- Add PM alert card for phase transition approval.
- Add `Report Progress` button in PM chat.

### Slice 2
- Add per-session command queue/ledger for orchestrator-issued commands.
- Enforce `session_id` routing for all orchestrator commands.

### Slice 3
- Add structured review verdict + close-session gate.
- Wire finalize completion checks (`pass + zero open issues + required checks`).

## Open Questions
- Should `finalize -> done` always require manual approval?
- Should orchestrator be allowed to auto-retry failed worker commands?
- Do we need per-project overrides for which statuses count as busy vs idle?
