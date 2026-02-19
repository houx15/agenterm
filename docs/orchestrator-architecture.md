# Orchestrator Architecture

This document describes how the orchestrator is implemented in agenterm today.

## Goals

- Turn user intent into safe, auditable execution across projects/tasks/sessions/worktrees.
- Keep humans in control through explicit approval gates before mutating actions.
- Enforce parallelism and role/model constraints to avoid runaway execution.
- Persist enough context (history, project state, knowledge, command ledger) for continuity.

## High-Level Components

### 1. Runtime Core (`internal/orchestrator/orchestrator.go`)

- `Orchestrator` owns the main chat loop and stateful dependencies:
  - repositories (`projectRepo`, `taskRepo`, `worktreeRepo`, `sessionRepo`, history/governance repos)
  - registries (`registry` for agents, `playbookRegistry` for playbooks)
  - `toolset` for all callable actions
  - LLM config defaults and limits (`maxToolRounds`, `maxHistory`, `globalMaxParallel`)
- `Chat(ctx, projectID, userMessage)` is the main entrypoint.

### 2. Prompt Builder (`internal/orchestrator/prompt.go`)

- `BuildSystemPrompt` composes:
  - project summary (tasks/worktrees/sessions)
  - agent catalog (model, max parallel, bio, etc.)
  - playbook stage/role context (`plan`, `build`, `test`)
  - hard rules (approval-driven transitions, safe command behavior)
  - progressive skill summary block

### 3. Tool Layer (`internal/orchestrator/tools.go`)

- `Toolset` defines tool schemas exposed to the LLM and executes tool calls.
- `RESTToolClient` is the transport adapter from tool calls to local REST API.
- Tools include:
  - planning/execution primitives: create project/task/worktree/session, send command, close session
  - merge/review utilities: merge worktree, resolve conflict, read outputs, idle checks
  - reporting utilities: project status and progress summaries
  - skill protocol utilities: `list_skills`, `get_skill_details`, `install_online_skill`

### 4. Scheduler Guards (`internal/orchestrator/scheduler.go`)

- Intercepts `create_session` and checks:
  - global max parallel
  - project max parallel
  - workflow role limits
  - role binding/model constraints
  - agent max parallel
- Prevents tool execution and returns structured block reasons when limits are exceeded.

### 5. Skills Catalog (`internal/orchestrator/skill_catalog.go`)

- Discovers skill specs from filesystem roots (`skills/`, `.agents/skills/`, `.claude/skills/`).
- Uses progressive disclosure:
  - summaries in system prompt
  - full content only when model calls `get_skill_details`
- Supports online install of skills from GitHub/raw URLs.

### 6. Event Trigger (`internal/orchestrator/events.go`)

- Polls session status transitions and triggers orchestrator reactions:
  - idle -> dispatch/next-step evaluation
  - waiting_review -> review orchestration
  - periodic status checks and blocked notifications
- Emits project events for UI/monitoring integration.

## Request Lifecycle

1. API receives chat request (`POST /api/orchestrator/chat`).
2. Orchestrator loads:
  - project state
  - agent registry
  - project playbook (explicit project playbook preferred; fallback behavior if unset)
  - project history and knowledge highlights
  - project-level LLM profile (provider/model overrides)
3. Approval gate is evaluated from the user message.
4. System prompt + message history + tool schemas are sent to the selected LLM provider.
5. For each assistant response block:
  - `text` -> streamed as `token`
  - `tool_use` -> validated and executed (or blocked with explicit reason)
6. Tool results are fed back into the LLM loop as `tool_result`.
7. Loop exits when no tool is used, or fails on round limit/error.
8. Assistant/user messages are persisted in orchestrator history.

## Safety & Governance Model

### Approval Gate

- Mutating tools are blocked unless the user message includes explicit confirmation intent.
- Non-mutating analysis and planning remains allowed without confirmation.

### Mutating Tool Gate

Currently treated as mutating:

- `create_project`
- `create_task`
- `create_worktree`
- `merge_worktree`
- `resolve_merge_conflict`
- `create_session`
- `send_command`
- `close_session`
- `write_task_spec`

### Concurrency Controls

- Global cap: orchestrator-wide active sessions.
- Project cap: per-project max parallel.
- Workflow/role cap: governed by workflow phases and role bindings.
- Agent cap: `max_parallel_agents` in registry.

### Execution Auditability

- `StreamEvent` captures token/tool_call/tool_result/done/error events.
- Command ledger stores recent command execution metadata for diagnostics/reporting.
- REST report endpoint (`GET /api/orchestrator/report`) aggregates:
  - project/task/session/worktree status
  - review-loop verdict and readiness checks
  - recent command ledger entries

## Configuration Resolution

Orchestrator model/provider selection resolves in this order:

1. project orchestrator profile (`project_orchestrator` settings)
2. matching orchestrator-capable agent credentials/settings
3. process defaults (`Options.Model`, default provider URL/model)

Playbook context resolves as:

1. `project.playbook` if set
2. playbook registry fallback behavior when unset
3. workflow repo fallback (`loadWorkflowAsPlaybook`) if needed

## API/Transport Boundaries

- Chat: `POST /api/orchestrator/chat`
- History: `GET /api/orchestrator/history`
- Report: `GET /api/orchestrator/report`
- Governance profile: `/api/projects/{id}/orchestrator`
- Tool executions are mostly routed through local REST API endpoints by `RESTToolClient`.

## Extensibility Points

- Add tools: extend `defaultTools` in `tools.go`.
- Add skill domains: drop new `skills/<id>/SKILL.md` packages or install online.
- Add governance policy: extend scheduler checks in `scheduler.go`.
- Add prompt policy/rules: update `BuildSystemPrompt`.
- Add event-driven behaviors: extend `EventTrigger` transition handlers.

## Known Design Tradeoffs

- Approval gating currently depends on keyword intent detection in user text (simple but strict).
- Tool loop uses bounded rounds (`maxToolRounds`) to prevent runaway behavior.
- Some orchestration logic is split across playbook registry, workflow repo, and role bindings; this is flexible but adds configuration complexity.
