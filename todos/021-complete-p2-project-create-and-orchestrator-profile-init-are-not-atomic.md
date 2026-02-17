---
status: complete
priority: p2
issue_id: "021"
tags: [code-review, api, data-integrity]
dependencies: []
---

# Project creation and orchestrator profile initialization are not atomic

## Problem Statement

Project creation persists the project first, then creates default orchestrator profile. If profile initialization fails, API returns error but project row remains, yielding partial provisioning.

## Findings

- `internal/api/projects.go:51` creates project.
- `internal/api/projects.go:56` initializes default profile afterward with separate operation.
- No compensating delete/rollback on profile init failure.

## Proposed Solutions

### Option 1: Move project + profile creation into a single DB transaction

Pros:
- Strong consistency.
- Prevents partial state from API errors.

Cons:
- Requires repo-level transactional method.

Effort: Medium
Risk: Low

### Option 2: Best-effort compensation (delete project when profile init fails)

Pros:
- Lower refactor effort.

Cons:
- Less robust than transaction; compensation can also fail.

Effort: Small
Risk: Medium

## Recommended Action

## Technical Details

Affected files:
- `internal/api/projects.go`
- `internal/db/project_repo.go`
- `internal/db/orchestrator_governance_repos.go`

## Acceptance Criteria

- [x] Project creation endpoint is atomic for project+profile provisioning.
- [x] Failure path leaves no orphan project record.
- [x] Test verifies rollback behavior.

## Work Log

### 2026-02-17 - Reported during workflows-review

- Found multi-step create flow without transactional boundary.

### 2026-02-17 - Fixed

- `POST /api/projects` now uses transactional `CreateWithDefaultOrchestrator`.
- Added repo-level and API-level rollback tests that verify no orphan project row remains on init failure.

## Resources

- `internal/api/projects.go:51`
- `internal/api/projects.go:56`
