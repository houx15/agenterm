---
status: complete
priority: p2
issue_id: "012"
tags: [code-review, quality, frontend, realtime]
dependencies: []
---

# Dashboard State Not Realtime Over WebSocket

WebSocket events are currently used only to append activity feed items. Core dashboard data (`sessions`, `projects`, `tasksByProject`) stays stale until the 30-second polling refresh runs.

## Problem Statement

TASK-14 requires real-time updates via WebSocket. In current behavior, session grid/status and summary counters can lag by up to 30 seconds even while websocket messages are flowing.

## Findings

- `frontend/src/pages/Dashboard.tsx:166` consumes `app.lastMessage` only to append `activity` entries.
- `frontend/src/pages/Dashboard.tsx:175`, `frontend/src/pages/Dashboard.tsx:222` derive visual state from `sessions`/`tasksByProject`, which are only refreshed by `loadDashboard()` polling.
- `frontend/src/pages/Dashboard.tsx:159` polling interval is 30s fallback, but currently acts as primary state refresh for dashboard metrics.

## Proposed Solutions

### Option 1: Apply websocket deltas to dashboard state

**Approach:** update `sessions` state in response to `status`/`windows` messages and recompute derived summaries immediately.

**Pros:**
- Meets real-time intent
- Better UX consistency with live session transitions

**Cons:**
- Needs careful merge logic between polling snapshots and websocket deltas

**Effort:** Medium

**Risk:** Medium

---

### Option 2: Trigger immediate `loadDashboard()` on relevant websocket messages

**Approach:** when `status`/`windows` events arrive, debounce and refetch dashboard data.

**Pros:**
- Simpler than full delta reducer
- Keeps source of truth in API

**Cons:**
- More network traffic
- Still not strictly instantaneous under network latency

**Effort:** Small/Medium

**Risk:** Low/Medium

## Recommended Action

Implemented websocket-driven session state updates in dashboard with debounced API synchronization for eventual consistency.

## Technical Details

**Affected files:**
- `frontend/src/pages/Dashboard.tsx:159`
- `frontend/src/pages/Dashboard.tsx:166`
- `frontend/src/pages/Dashboard.tsx:175`
- `frontend/src/pages/Dashboard.tsx:222`

## Resources

- Spec: `docs/TASK-14-dashboard-ui.md`
- Commits reviewed: `d1c0404`, `2169644`

## Acceptance Criteria

- [ ] Session status changes are reflected in grid/summary without waiting for 30s poll
- [ ] WebSocket updates do not duplicate or corrupt dashboard state
- [ ] Polling remains fallback (not primary) refresh mechanism

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Traced websocket handling and state update paths in dashboard page
- Verified derived dashboard metrics are polling-driven

**Learnings:**
- Current websocket usage is activity-feed-only, not full dashboard state sync

## Notes

- Debounced refetch on `status` + `windows` is likely the fastest low-risk fix.
