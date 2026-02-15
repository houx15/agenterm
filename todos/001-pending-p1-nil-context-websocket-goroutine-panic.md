---
status: pending
priority: p1
issue_id: "001"
tags: [code-review, websocket, reliability, concurrency]
dependencies: []
---

# Guard Hub Context Before Client Pumps

## Problem Statement

`HandleWebSocket` starts client goroutines with `h.ctx`, but `h.ctx` is only assigned inside `Run()`. If HTTP traffic arrives before `Run()` executes, client goroutines receive a nil context and can panic.

## Findings

- `internal/hub/hub.go:48` sets `h.ctx = ctx` only when `Run()` starts.
- `internal/hub/hub.go:120` and `internal/hub/hub.go:121` call `client.writePump(h.ctx)` and `client.readPump(h.ctx)` with no nil check.
- `internal/hub/client.go:76` calls `ctx.Done()` and `internal/hub/client.go:38` calls `c.conn.Read(ctx)`, both unsafe with nil context.
- Impact: first client connection can crash the process if startup order is incorrect.

## Proposed Solutions

### Option 1: Make Hub Context Non-Nil at Construction Time

**Approach:** Initialize `h.ctx = context.Background()` in `New()`, then replace with runtime context in `Run()`.

**Pros:**
- Smallest code change.
- Prevents nil panics immediately.

**Cons:**
- Pumps may outlive intended lifecycle if `Run()` never starts.

**Effort:** 30-60 minutes

**Risk:** Low

---

### Option 2: Pass Per-Connection Context from Request

**Approach:** In `HandleWebSocket`, derive `ctx := r.Context()` (or `context.WithCancel`) and pass that to pump goroutines.

**Pros:**
- Ties client lifecycle to HTTP/WebSocket request context.
- Removes hidden dependency on `Run()` startup ordering.

**Cons:**
- Requires careful coordination with hub shutdown behavior.

**Effort:** 1-2 hours

**Risk:** Medium

---

### Option 3: Reject Connections Until Hub Is Running

**Approach:** Track running state and return `503` from `HandleWebSocket` until `Run()` is active.

**Pros:**
- Explicit contract and predictable startup behavior.

**Cons:**
- Adds operational coupling and transient startup failures.

**Effort:** 1 hour

**Risk:** Low

## Recommended Action

To be filled during triage.

## Technical Details

**Affected files:**
- `internal/hub/hub.go:48`
- `internal/hub/hub.go:120`
- `internal/hub/hub.go:121`
- `internal/hub/client.go:38`
- `internal/hub/client.go:76`

**Related components:**
- Hub lifecycle management
- WebSocket connection handling

**Database changes (if any):**
- No

## Resources

- **Task spec:** `TASK-04-websocket-hub.md`
- **Related implementation:** `internal/hub/hub.go`, `internal/hub/client.go`

## Acceptance Criteria

- [ ] No nil context is passed to websocket read/write pump code paths.
- [ ] Connection accepted before `Run()` does not panic the process.
- [ ] New regression test covers pre-`Run()` connection behavior.
- [ ] Existing hub tests remain green.

## Work Log

### 2026-02-15 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed hub lifecycle wiring and websocket goroutine startup paths.
- Traced context initialization and usage in client pumps.
- Identified nil-context crash risk tied to startup ordering.

**Learnings:**
- Hub correctness currently depends on undocumented call ordering.
- A small context-handling change can remove a high-impact failure mode.

## Notes

- This is a merge-blocking reliability issue because it can terminate the process on first connection under valid deployment race conditions.
