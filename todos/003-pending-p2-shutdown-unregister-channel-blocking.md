---
status: pending
priority: p2
issue_id: "003"
tags: [code-review, websocket, concurrency, reliability]
dependencies: []
---

# Prevent Unregister Blocking During Shutdown

## Problem Statement

Client teardown can block on `h.unregister <- c` after `Run()` exits. This violates the acceptance requirement for clean disconnect behavior and risks goroutine leaks under load.

## Findings

- `internal/hub/client.go:31` unconditionally sends to `h.unregister` in a defer.
- `internal/hub/hub.go:58` returns from `Run()` on context cancel, leaving no receiver on `h.unregister`.
- `internal/hub/hub.go:35` buffer size is 16; more than 16 post-shutdown disconnects can block forever.
- Impact: goroutine leaks and potential shutdown hangs with many active clients.

## Proposed Solutions

### Option 1: Non-Blocking Unregister Send

**Approach:** Replace defer send with a `select { case h.unregister <- c: default: }`.

**Pros:**
- Minimal fix for deadlock risk.
- Safe even when hub loop has exited.

**Cons:**
- Client map cleanup may be skipped after shutdown.

**Effort:** 30-60 minutes

**Risk:** Low

---

### Option 2: Dedicated Shutdown Drain Path

**Approach:** Add explicit shutdown state and a drain goroutine that handles pending unregister operations after `Run()` cancellation.

**Pros:**
- Preserves cleanup semantics.
- Stronger lifecycle guarantees.

**Cons:**
- More complexity and synchronization logic.

**Effort:** 2-4 hours

**Risk:** Medium

## Recommended Action

To be filled during triage.

## Technical Details

**Affected files:**
- `internal/hub/client.go:31`
- `internal/hub/hub.go:35`
- `internal/hub/hub.go:58`

**Related components:**
- Hub shutdown lifecycle
- Client goroutine teardown

**Database changes (if any):**
- No

## Resources

- **Task spec acceptance criteria:** `TASK-04-websocket-hub.md`
- **Implementation:** `internal/hub/client.go`, `internal/hub/hub.go`

## Acceptance Criteria

- [ ] Client teardown cannot block when hub `Run()` has returned.
- [ ] Shutdown with >16 clients does not leak goroutines.
- [ ] Regression test simulates high-client-count shutdown and completes within timeout.

## Work Log

### 2026-02-15 - Initial Discovery

**By:** Codex

**Actions:**
- Inspected client defer path and hub shutdown loop.
- Modeled post-cancel behavior for buffered unregister channel.

**Learnings:**
- Buffered channels reduce but do not eliminate shutdown deadlock risk.
- The current design needs explicit handling for post-loop teardown messages.
