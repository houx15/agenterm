---
status: complete
priority: p1
issue_id: "018"
tags: [code-review, orchestrator, scheduling, concurrency]
dependencies: []
---

# Scheduler does not enforce global concurrency limits

## Problem Statement

Scheduler checks are currently scoped to one project. This allows aggregate session creation across multiple projects to exceed total model/agent parallel budgets, violating stated orchestration constraints.

## Findings

- `internal/orchestrator/scheduler.go:44` uses `listSessionsByProject()` as the base dataset for all limits.
- `internal/orchestrator/scheduler.go:92` applies `agent.MaxParallelAgents` against project-scoped sessions only.
- No global session query exists for cross-project budget enforcement.

## Proposed Solutions

### Option 1: Add global session counting for scheduler decisions

Pros:
- Enforces intended system-wide caps.
- Minimal behavior change surface.

Cons:
- Requires additional query path in scheduler.

Effort: Medium
Risk: Medium

### Option 2: Add dedicated scheduler snapshot query API in repo layer

Pros:
- Cleaner separation and easier testing.
- Efficient single-query aggregation.

Cons:
- More repository/API code.

Effort: Medium
Risk: Low

## Recommended Action

## Technical Details

Affected files:
- `internal/orchestrator/scheduler.go`
- `internal/db/session_repo.go`

## Acceptance Criteria

- [x] Scheduler enforces global model/agent limits across all projects.
- [x] Tests include two-project scenario proving rejection when global cap is reached.
- [x] Decision reason clearly states global cap violation.

## Work Log

### 2026-02-17 - Reported during workflows-review

- Identified project-scoped counting where global counting is required.

### 2026-02-17 - Fixed

- Scheduler now counts active sessions globally via `sessionRepo.List`.
- Global cap and agent-type cap are enforced across all projects.
- Added regression test for two-project global-limit rejection.

## Resources

- `internal/orchestrator/scheduler.go:44`
- `internal/orchestrator/scheduler.go:92`
