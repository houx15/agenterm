---
status: complete
priority: p2
issue_id: "027"
tags: [code-review, api, orchestrator, product-contract]
dependencies: []
---

# Orchestrator report misses review-loop state details

The report endpoint exposes totals, but not current review-loop state/iteration needed by the governance contract.

## Problem Statement

The governance plan requires PM status visibility to include review-loop state, and specifically calls out review iteration details. Current report payload only includes aggregate counts (`review_cycles_total`, `open_review_issues_total`), which is insufficient for user-facing PM status and notification UX.

## Findings

- Plan requirement: report should include phase/blockers/queue depth/review iteration details (`docs/plans/2026-02-17-feat-project-orchestrator-workflow-governance-plan.md:109`).
- Acceptance phrasing expects phase/blocker/review loop state visibility (`docs/plans/2026-02-17-feat-project-orchestrator-workflow-governance-plan.md:315`).
- `internal/api/orchestrator.go:93` only computes totals across tasks; no latest cycle status, no per-task review state, no latest iteration.
- Existing tests validate phase/blockers/queue depth but do not assert review-loop state contract (`internal/api/router_test.go:706`).

## Proposed Solutions

### Option 1: Add aggregate current review state fields

**Approach:** Extend report with `review_state`, `latest_review_iteration`, and `tasks_in_review` derived from latest cycle per task.

**Pros:**
- Minimal payload expansion.
- Easy frontend consumption.

**Cons:**
- Loses per-task detail when multiple tasks are in different review states.

**Effort:** Small

**Risk:** Low

---

### Option 2: Add per-task review summary block

**Approach:** Add `review_summary_by_task` containing `{task_id, iteration, status, open_issues}` for each task with cycles.

**Pros:**
- Full PM visibility.
- Better debugging and UI flexibility.

**Cons:**
- Larger payload; may need frontend shaping.

**Effort:** Medium

**Risk:** Low

## Recommended Action

To be filled during triage.

## Technical Details

**Affected files:**
- `internal/api/orchestrator.go:60`
- `internal/api/router_test.go:655`

**Database changes (if any):**
- No

## Resources

- **Spec/plan:** `docs/plans/2026-02-17-feat-project-orchestrator-workflow-governance-plan.md`
- **Task context:** `docs/TASK-15-orchestrator.md`

## Acceptance Criteria

- [x] `/api/orchestrator/report` includes review-loop state and iteration details, not only totals.
- [x] Report fields are documented and stable for UI consumption.
- [x] API tests assert the new review-loop state fields.

## Work Log

### 2026-02-17 - Discovery

**By:** Codex

**Actions:**
- Reviewed orchestrator report assembly and tests.
- Compared payload fields against governance plan status contract.
- Confirmed missing review-loop state/iteration information.

**Learnings:**
- Current payload is useful for high-level counters but not sufficient for PM-level workflow tracking.

### 2026-02-17 - Resolution

**By:** Codex

**Actions:**
- Extended `/api/orchestrator/report` with review-loop fields:
  - `review_state`
  - `review_latest_iteration`
  - `review_tasks_in_loop`
  - `review_task_summaries`
- Added API test coverage asserting new report fields.

**Learnings:**
- Per-task latest-cycle summaries provide enough detail for PM status UI while preserving aggregate counters.
