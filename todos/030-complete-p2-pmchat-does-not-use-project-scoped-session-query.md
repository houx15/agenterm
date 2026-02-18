---
status: complete
priority: p2
issue_id: "030"
tags: [code-review, performance, frontend, api]
dependencies: []
---

# PM Chat Does Not Use Project-Scoped Session Query

PM Chat refreshes sessions without `project_id`, so it fetches all sessions every polling cycle and on orchestrator events.

## Problem Statement

This increases unnecessary payload and server work as data grows, and can cause context leakage risk if task/session identifiers ever collide across projects.

## Findings

- `frontend/src/pages/PMChat.tsx:61` calls `listSessions<Session[]>()` with no project filter.
- `internal/api/sessions.go:167` supports `project_id` filtering, but this path is unused here.
- PM Chat refreshes every 5s (`frontend/src/pages/PMChat.tsx:100`), amplifying over-fetching.

## Proposed Solutions

### Option 1: Add `project_id` Query In PM Chat

**Approach:** Extend `listSessions` client to accept optional filters and call with current `projectID`.

**Pros:**
- Directly reduces request/response size.
- Aligns query semantics with page context.

**Cons:**
- Minor API client signature change.

**Effort:** Small

**Risk:** Low

---

### Option 2: Add Dedicated PM Chat Aggregation Endpoint

**Approach:** Backend endpoint returning tasks + project sessions in one call.

**Pros:**
- Fewer round trips and tighter consistency.

**Cons:**
- More backend surface area.

**Effort:** Medium

**Risk:** Medium

## Recommended Action


## Technical Details

- `frontend/src/pages/PMChat.tsx`
- `frontend/src/api/client.ts`
- `internal/api/sessions.go`

## Resources

- `docs/TASK-17-pm-chat-ui.md`

## Acceptance Criteria

- [ ] PM Chat session fetch is scoped to selected project.
- [ ] Polling load decreases for multi-project environments.
- [ ] Behavior remains correct when switching projects.

## Work Log

### 2026-02-18 - Initial Discovery

**By:** Codex

**Actions:**
- Traced PM Chat refresh calls and backend listSessions API capabilities.
- Confirmed over-fetch path under polling.

**Learnings:**
- Existing API already supports filter; this is mainly a frontend integration gap.

## Notes

- This is not merge-blocking but should be addressed soon.
