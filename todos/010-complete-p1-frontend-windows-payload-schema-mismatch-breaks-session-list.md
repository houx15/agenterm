---
status: complete
priority: p1
issue_id: "010"
tags: [code-review, frontend, integration, quality]
dependencies: []
resolution: "Changed frontend to read data.list instead of data.windows for windows payload"
---

# Frontend Windows Payload Schema Mismatch Breaks Session List

## Problem Statement

The frontend reads the wrong field name for the `windows` message, so session list state never populates from backend payload.

## Findings

- Backend sends windows list as `{"type":"windows","list":[...]}` via `WindowsMessage.List` (`internal/hub/protocol.go:22`, `internal/hub/hub.go:141`).
- Frontend handler reads `data.windows` instead of `data.list` (`web/index.html:848`).
- Result: `state.windows` becomes empty on every windows update, preventing expected sidebar/session behavior.

## Proposed Solutions

### Option 1: Align frontend to backend contract

**Approach:** Update frontend windows handler to use `data.list` (with fallback support if needed).

**Pros:**
- Restores core session list functionality immediately
- Minimal code change

**Cons:**
- Requires careful regression check for reconnect/bootstrap flows

**Effort:** 15-45 minutes

**Risk:** Low

---

### Option 2: Change backend payload key to `windows`

**Approach:** Rename protocol field in hub to match frontend expectation.

**Pros:**
- Keeps frontend code unchanged

**Cons:**
- Breaks protocol compatibility with existing clients/tests expecting `list`

**Effort:** 30-90 minutes

**Risk:** Medium

---

### Option 3: Dual-key compatibility

**Approach:** Frontend reads `data.list ?? data.windows` during transition.

**Pros:**
- Backward/forward compatibility

**Cons:**
- Slight protocol ambiguity remains

**Effort:** 20-60 minutes

**Risk:** Low

## Recommended Action


## Technical Details

**Affected files:**
- `web/index.html:848`
- `internal/hub/protocol.go:24`
- `internal/hub/hub.go:141`

**Related components:**
- Sidebar rendering
- Active session selection
- Unread/status updates

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Spec target:** `TASK-05-frontend-ui.md`

## Acceptance Criteria

- [ ] Frontend correctly populates `state.windows` from backend windows payload
- [ ] Session list renders on initial connect and updates live
- [ ] Active window auto-select logic works when first windows message arrives

## Work Log

### 2026-02-16 - Task 05 Implementation Review

**By:** Codex

**Actions:**
- Traced backend windows message JSON schema and frontend handler
- Identified key mismatch causing empty window state

**Learnings:**
- UI protocol drift can silently break core rendering while connection appears healthy

## Notes

- This is a merge-blocking functional issue for Task 05.
