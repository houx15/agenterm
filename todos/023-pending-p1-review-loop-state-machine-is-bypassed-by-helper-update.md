---
status: pending
priority: p1
issue_id: "023"
tags: [code-review, quality, orchestrator, review-loop]
dependencies: []
---

# Prevent Review State Machine Bypass

A helper path updates review cycle status directly in SQL, bypassing transition validation and open-issue safeguards that were added to `UpdateCycleStatus`.

## Problem Statement

The review loop must be deterministic and state-machine driven. Direct status writes can silently violate transition rules, producing inconsistent cycle states and incorrect milestone events.

## Findings

- `internal/db/orchestrator_governance_repos.go:699` (`SetCycleStatusByTaskOpenIssues`) updates latest cycle status directly with SQL.
- `internal/db/orchestrator_governance_repos.go:622` (`UpdateCycleStatus`) contains transition + open-issue validation, but helper path bypasses it.
- `internal/api/orchestrator_governance.go:557` calls the bypass helper during issue updates.

## Proposed Solutions

### Option 1: Replace helper SQL write with validated transition path

Pros: Reuses existing invariants; minimal behavior drift.
Cons: Requires careful status derivation logic.
Effort: Small
Risk: Low

### Option 2: Move transition validation into DB-level constraints/triggers

Pros: Strong consistency at storage layer.
Cons: Higher complexity and reduced portability.
Effort: Large
Risk: Medium

## Recommended Action


## Technical Details

Affected files:
- `internal/db/orchestrator_governance_repos.go`
- `internal/api/orchestrator_governance.go`
- `internal/db/orchestrator_governance_repos_test.go`

Database changes:
- No (unless trigger-based approach selected)

## Resources

- `docs/plans/2026-02-17-feat-project-orchestrator-workflow-governance-plan.md`

## Acceptance Criteria

- [ ] No code path updates review cycle status without transition validation.
- [ ] `review_passed` is impossible when open issues exist.
- [ ] Tests cover helper-driven issue updates and verify legal transitions only.

## Work Log

### 2026-02-17 - Review Discovery

By: Codex

Actions:
- Traced review-cycle update flows from API handlers into repo methods.
- Confirmed direct SQL status write bypasses state-machine checks.

Learnings:
- Validation exists but is not uniformly applied across update paths.

## Notes

- Preserve current API behavior while hardening invariants.
