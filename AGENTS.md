# AGENTS.md

This document is for AI assistants that need to explain, operate, or modify agenterm.

## Platform Summary

agenterm is a local control plane for coding agents:

1. Runs tmux-backed agent sessions.
2. Provides web UI + REST API + WebSocket streams.
3. Uses an orchestrator to plan and coordinate work.
4. Uses playbooks (`plan`, `build`, `test`) to define role-based execution.

## Core Capabilities

1. Agent registry:
   - store agent profiles (command, model, limits, orchestrator support)
2. Playbooks:
   - role contracts (`mode`, `inputs_required`, `actions_allowed`, `handoff_to`, `retry_policy`, `gates`)
3. Project/task/worktree/session lifecycle:
   - create project
   - split worktrees
   - run per-role sessions
4. Orchestrator:
   - approval-gated mutating actions
   - role contract enforcement
   - loop/handoff/retry tracking
5. Review workflow:
   - review loops and issue tracking through API + orchestrator reporting

## Architecture Pointers

1. `docs/orchestrator-architecture.md`
2. `README.md` architecture + API sections
3. `internal/orchestrator/*` (runtime, tools, scheduler, events)
4. `internal/api/*` (REST endpoints)
5. `internal/playbook/*` (playbook schema + validation)

## Assistant Operating Rules

1. Prefer asking clarifying questions before writing configs.
2. Explain the reason behind each config decision.
3. Do not assume one fixed template; derive config from user goals.
4. Preserve required playbook stage keys: `plan`, `build`, `test`.
5. Use explicit approval before destructive or high-impact actions.

## How To Help A User

When user asks “set this up for me”, do this:

1. Collect requirements:
   - repository path
   - team models/tools available
   - desired workflow style (pairing / tdd / parallel compound / custom)
   - risk tolerance and max parallelism
2. Propose agent registry + playbook config with rationale.
3. Apply via local API (or `scripts/agentic-bootstrap.sh` if user prefers).
4. Create project and confirm orchestrator kickoff message.

## Genesis / Self-Modification Context

If user asks for “AI genesis” or system self-improvement:

1. Treat it as a governed project workflow.
2. Use role contracts and approval gates.
3. Ensure review/verification before merge/restart operations.
4. Keep outputs auditable through task/worktree/session records.

## Startup Checklist (for assistants)

1. Confirm agenterm server URL and token.
2. Verify API access (`GET /api/projects`).
3. Confirm user repo path exists.
4. Propose config plan and ask approval.
5. Apply config + create project + present next PM Chat prompt.
