---
status: complete
priority: p2
issue_id: "026"
tags: [code-review, reliability, api, orchestrator]
dependencies: []
---

# Review events emitted without state transition

`syncLatestCycleStatusAndEmit` emits review milestone events even when the review cycle status does not actually change.

## Problem Statement

Review milestone events should represent state transitions, but the current helper emits them on every issue create/update call. This causes duplicate `review_iteration_completed` and `project_blocked` events, which makes PM status feeds noisy and semantically incorrect.

## Findings

- `internal/api/orchestrator_governance.go:663` calls `SetCycleStatusByTaskOpenIssues` and then always calls `emitReviewCycleProjectEvents`.
- `internal/db/orchestrator_governance_repos.go:727` short-circuits when status is already equal to computed `nextStatus`.
- When no state transition occurs (for example, adding a second open issue while status is already `review_changes_requested`), events are still emitted as if an iteration just completed.
- This contradicts milestone-style push events and can create notification churn.

## Proposed Solutions

### Option 1: Emit only on transition

**Approach:** Make status sync return transition metadata (`changed`, `from`, `to`) and emit events only when `changed=true`.

**Pros:**
- Accurate event semantics.
- Minimal behavior change elsewhere.

**Cons:**
- Requires repo and handler contract update.

**Effort:** Small

**Risk:** Low

---

### Option 2: Re-read previous status in handler

**Approach:** In `syncLatestCycleStatusAndEmit`, fetch latest cycle before and after sync and emit only if status changed.

**Pros:**
- No repo signature changes.

**Cons:**
- Additional DB queries in hot path.

**Effort:** Small

**Risk:** Low

## Recommended Action

To be filled during triage.

## Technical Details

**Affected files:**
- `internal/api/orchestrator_governance.go:663`
- `internal/db/orchestrator_governance_repos.go:701`

**Database changes (if any):**
- No

## Resources

- **Spec/plan:** `docs/plans/2026-02-17-feat-project-orchestrator-workflow-governance-plan.md`
- **Related endpoint tests:** `internal/api/router_test.go`

## Acceptance Criteria

- [x] Review milestone events are emitted only when cycle status transitions.
- [x] Repeated issue updates with unchanged cycle status do not emit duplicate milestone events.
- [x] Tests cover both transition and no-transition paths.

## Work Log

### 2026-02-17 - Discovery

**By:** Codex

**Actions:**
- Reviewed review-cycle issue creation/update flow.
- Traced event emission path and status sync logic.
- Verified helper emits events even when status remains unchanged.

**Learnings:**
- State sync and event emission are currently coupled without transition gating.

### 2026-02-17 - Resolution

**By:** Codex

**Actions:**
- Added `SyncLatestCycleStatusByTaskOpenIssues` to return transition flag and latest cycle.
- Updated API sync helper to emit review milestone events only when status actually changes.
- Added regression test for changed/no-change behavior in review status sync.

**Learnings:**
- Transition-aware repo contracts keep API event emission deterministic without extra queries.
