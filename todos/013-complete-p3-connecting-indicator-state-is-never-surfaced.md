---
status: complete
priority: p3
issue_id: "013"
tags: [code-review, frontend, ux]
dependencies: []
resolution: "Added state.connectionStatus updates on all connection state changes"
---

# Connecting Indicator State Is Never Surfaced

## Problem Statement

UI includes a connecting indicator style, but state updates never set `state.connectionStatus` to `connecting`, so the visual connecting state is effectively dead code.

## Findings

- Indicator logic checks `state.connectionStatus === 'connecting'` (`web/index.html:649`).
- Connection lifecycle updates only `this.status` and `state.connected` (`web/index.html:562`, `web/index.html:572`, `web/index.html:581`).
- `state.connectionStatus` is initialized but never updated (`web/index.html:543`).

## Proposed Solutions

### Option 1: Drive indicator from `connection.status`

**Approach:** Remove redundant `state.connectionStatus` and reference connection instance state directly.

**Pros:**
- Eliminates duplicated state
- Less drift risk

**Cons:**
- Requires small refactor in rendering helper

**Effort:** 15-45 minutes

**Risk:** Low

---

### Option 2: Keep state field but synchronize it

**Approach:** Update `state.connectionStatus` at connect/open/close/error transitions.

**Pros:**
- Minimal local changes

**Cons:**
- Maintains duplicate state source

**Effort:** 15-30 minutes

**Risk:** Low

---

### Option 3: Remove connecting style

**Approach:** Simplify to connected/disconnected only.

**Pros:**
- Less UI complexity

**Cons:**
- Loses useful user feedback during reconnect attempts

**Effort:** 10-20 minutes

**Risk:** Low

## Recommended Action


## Technical Details

**Affected files:**
- `web/index.html:543`
- `web/index.html:649`
- `web/index.html:562`

**Related components:**
- Connection status UX

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Spec target:** `TASK-05-frontend-ui.md`

## Acceptance Criteria

- [ ] Connecting state is visibly represented during initial connect/reconnect
- [ ] Indicator transitions are consistent with actual socket lifecycle

## Work Log

### 2026-02-16 - Task 05 Implementation Review

**By:** Codex

**Actions:**
- Traced connection state variables and indicator rendering logic
- Found unsynchronized `connectionStatus` field

**Learnings:**
- Duplicate UI state fields are a common source of stale indicators

## Notes

- Nice-to-have UX correctness improvement.
