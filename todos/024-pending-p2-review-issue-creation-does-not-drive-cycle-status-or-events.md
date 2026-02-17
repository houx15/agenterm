---
status: pending
priority: p2
issue_id: "024"
tags: [code-review, quality, orchestrator, events]
dependencies: ["023"]
---

# Wire Issue Creation Into Review Loop Status

Creating a review issue does not update cycle status or emit milestone events, delaying or missing blocked-state signaling.

## Problem Statement

When reviewers create new issues, cycle state should reflect `review_changes_requested` immediately and emit relevant project events. Current behavior only recomputes status in issue-update flow.

## Findings

- `internal/api/orchestrator_governance.go:479` creates issues but does not recompute cycle status.
- `internal/api/orchestrator_governance.go:556` recomputes status only in update path.
- `internal/db/orchestrator_governance_repos.go:758` issue creation repo method does not integrate with cycle-state updates.

## Proposed Solutions

### Option 1: Recompute + emit events after successful issue creation

Pros: Minimal code change and correct user-visible state.
Cons: Additional DB reads after create.
Effort: Small
Risk: Low

### Option 2: Encapsulate issue-create + cycle-sync in transactional repo API

Pros: Cleaner invariant ownership.
Cons: Wider refactor across API layer.
Effort: Medium
Risk: Medium

## Recommended Action


## Technical Details

Affected files:
- `internal/api/orchestrator_governance.go`
- `internal/db/orchestrator_governance_repos.go`
- `internal/api/router_test.go`

Database changes:
- No

## Resources

- `docs/plans/2026-02-17-feat-project-orchestrator-workflow-governance-plan.md`

## Acceptance Criteria

- [ ] New open issue immediately drives latest cycle out of pass state.
- [ ] `project_blocked`/review iteration events emit on issue creation when applicable.
- [ ] API tests cover create-issue state/event behavior.

## Work Log

### 2026-02-17 - Review Discovery

By: Codex

Actions:
- Compared create vs update issue handlers.
- Verified only update path triggers status synchronization/events.

Learnings:
- State/event contract is incomplete on first issue creation.

## Notes

- Ensure event emission remains idempotent on repeated updates.
