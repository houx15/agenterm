---
status: pending
priority: p2
issue_id: "025"
tags: [code-review, architecture, orchestrator, notifications]
dependencies: []
---

# Complete Status Notification Contract

Two planned status-notification controls are not fully implemented: `notify_on_blocked` preference is unused, and timer blocked detection relies on a `blockers` field that progress reports do not provide.

## Problem Statement

Users expect PM-style push notifications and configurable blocked alerts. Current implementation may fail to emit blocked events and cannot honor per-project blocked-notification preferences.

## Findings

- `internal/orchestrator/events.go:63` checks `report["blockers"]`, but `generate_progress_report` output omits blockers.
- `internal/orchestrator/tools.go:479` progress summary includes counts only (`task_counts`, `session_counts`, totals).
- `internal/api/orchestrator_governance.go:17` and `internal/db/models.go:79` define `notify_on_blocked`, but no event path checks this preference before broadcasting.

## Proposed Solutions

### Option 1: Add blocker derivation + preference gating in event trigger

Pros: Fastest path to expected behavior.
Cons: Blocker logic may be heuristic if not centralized.
Effort: Medium
Risk: Medium

### Option 2: Extend report contract with explicit `phase`, `queue_depth`, `blockers` and consume centrally

Pros: Strong API contract aligned with plan; reusable by UI.
Cons: Requires coordinated API + orchestrator + tests changes.
Effort: Medium
Risk: Low

## Recommended Action


## Technical Details

Affected files:
- `internal/orchestrator/events.go`
- `internal/orchestrator/tools.go`
- `internal/api/orchestrator.go`
- `internal/api/orchestrator_governance.go`

Database changes:
- No

## Resources

- `docs/plans/2026-02-17-feat-project-orchestrator-workflow-governance-plan.md`
- `docs/TASK-15-orchestrator.md`

## Acceptance Criteria

- [ ] Blocked-event emission is based on explicit, tested report fields.
- [ ] `notify_on_blocked=false` suppresses blocked notifications for that project.
- [ ] Report payload exposes phase/blocker/review-loop status expected by PM UX.

## Work Log

### 2026-02-17 - Review Discovery

By: Codex

Actions:
- Audited timer-trigger event logic and progress report payload composition.
- Traced notification preference fields through API and event broadcasters.

Learnings:
- Status push exists, but contract and preference handling are incomplete.

## Notes

- Keep push and pull status surfaces consistent to avoid frontend drift.
