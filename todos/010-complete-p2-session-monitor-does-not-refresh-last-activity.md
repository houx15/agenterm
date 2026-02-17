---
status: complete
priority: p2
issue_id: "010"
tags: [code-review, quality, lifecycle]
dependencies: []
---

# Refresh last_activity_at on monitor polling

The session monitor does not update `sessions.last_activity_at` unless the status changes. The task spec requires the monitor loop to update activity timestamps continuously, which is needed for accurate observability and idle reasoning.

## Problem Statement

`docs/TASK-12-session-lifecycle.md` Step 3 explicitly requires monitor tick behavior to update `last_activity_at` in DB. Current code only calls `sessionRepo.Update()` inside `persistStatus()`, so if output changes but status remains `working`, `last_activity_at` becomes stale.

## Findings

- `internal/session/monitor.go:94` loop captures pane and records output, but does not persist activity timestamp by itself.
- `internal/session/monitor.go:211` `persistStatus()` is the only path that writes via `sessionRepo.Update()`.
- `internal/session/monitor.go:108` status persistence is gated on status transitions, so no transition means no DB activity update.

## Proposed Solutions

### Option 1: Persist activity each tick when session exists

**Approach:** Add a lightweight update path in the monitor loop to refresh `last_activity_at` every poll (or when new lines are detected), independent of status changes.

**Pros:**
- Matches spec directly.
- Keeps API timestamps accurate.

**Cons:**
- More DB write frequency.

**Effort:** 1-2 hours

**Risk:** Low

---

### Option 2: Persist activity only when new output lines arrive

**Approach:** Have `recordOutput()` signal whether new lines were observed; if yes, update `last_activity_at` in DB.

**Pros:**
- Lower write volume than every tick.
- Still tracks actual activity.

**Cons:**
- Slightly more plumbing in monitor state.

**Effort:** 2-3 hours

**Risk:** Low

## Recommended Action


## Technical Details

**Affected files:**
- `internal/session/monitor.go`

**Related components:**
- `internal/db/session_repo.go`
- `internal/api/sessions.go`

## Resources

- **Commit:** `b29a91a`
- **Spec:** `docs/TASK-12-session-lifecycle.md`

## Acceptance Criteria

- [ ] `last_activity_at` updates while monitor is running, even without status transitions.
- [ ] Idle endpoint reflects fresh activity timestamps.
- [ ] Tests cover the timestamp refresh behavior.

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed monitor polling flow.
- Verified DB writes only happen in status persistence path.
- Mapped behavior against TASK-12 Step 3 requirements.

**Learnings:**
- Current monitor detects status and output, but timestamp persistence is coupled to status transitions.

## Notes

- Keep write amplification in mind if polling interval remains at 1s.
