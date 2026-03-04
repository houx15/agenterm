# AGENTS.md

This document is for AI assistants that need to explain, operate, or modify agenterm.

## Platform Summary

agenterm is a local control plane for coding agents where the human is the orchestrator:

1. Manages PTY-backed agent sessions across multiple worktrees.
2. Provides web UI + REST API + WebSocket streams.
3. Human drives all workflow decisions — plan, build, review, merge, test.
4. Agents execute autonomously within scoped sessions; no agent-to-agent coordination.

## Core Capabilities

1. Agent registry:
   - Store agent profiles (command, resume command, max parallel, capabilities)
   - Permission templates per agent type (standard/strict/permissive)
2. Requirement-driven workflow:
   - Prioritized requirement pool
   - Planning sessions with planner agents
   - Blueprint-based execution setup
   - Human-triggered stage transitions (build → review → merge → test → done)
3. Project/task/worktree/session lifecycle:
   - Create project linked to repo
   - Split worktrees per task from planning blueprint
   - Run agent sessions per worktree with auto-generated CLAUDE.md/AGENTS.md
4. Session management:
   - Suspend/resume across app restarts
   - Output monitoring with event detection ([READY_FOR_REVIEW], [BLOCKED])
   - Command queue and safety policy

## Architecture Pointers

1. `docs/plans/2026-03-04-pivot-rewrite-design.md` — full design document
2. `docs/2026-03-04-pivot-new-version.md` — pivot decision and rationale
3. `internal/session/*` — session lifecycle, command queue, safety policy
4. `internal/scaffold/*` — worktree setup, CLAUDE.md generation, permission config writing
5. `internal/api/*` — REST endpoints
6. `internal/pty/*` — PTY terminal backend

## Assistant Operating Rules

1. The human makes all orchestration decisions — never automate stage transitions.
2. Agents work within scoped TUI sessions — do not coordinate agents with each other.
3. Prefer asking clarifying questions before writing configs.
4. Use explicit approval before destructive or high-impact actions.
5. Preserve the requirement → plan → build → review → merge → test cycle.

## How To Help A User

When user asks "set this up for me":

1. Confirm they've run the onboarding wizard (agent team + permissions).
2. Help them create a project linked to their repo.
3. Guide them through adding requirements to the demand pool.
4. Explain the planning → execution → stage transition flow.
