---
status: complete
priority: p2
issue_id: "008"
tags: [code-review, reliability, frontend]
dependencies: []
---

# Prevent WebSocket Reconnect Scheduling After React Unmount

## Problem Statement

The WebSocket hook schedules reconnect attempts in `onclose` without guarding for component unmount/intentional close. This can trigger reconnect timers during teardown and cause unnecessary reconnect attempts or state updates after unmount.

## Findings

- `useWebSocket` sets reconnect timer in `ws.onclose` unconditionally (`frontend/src/hooks/useWebSocket.ts:42`).
- Cleanup closes the socket (`frontend/src/hooks/useWebSocket.ts:74`), which triggers `onclose`, but there is no `isUnmounted`/`shouldReconnect` guard.
- Timer callback re-enters `connect()` (`frontend/src/hooks/useWebSocket.ts:51`), which can execute after teardown boundaries.

## Proposed Solutions

### Option 1: Add explicit `shouldReconnectRef` lifecycle guard

**Approach:** Set `shouldReconnectRef.current = true` on mount and false during cleanup; in `onclose`, only schedule reconnect when true.

**Pros:**
- Robust and explicit lifecycle intent
- Prevents reconnect churn after unmount

**Cons:**
- Slightly more hook state/complexity

**Effort:** Small

**Risk:** Low

---

### Option 2: Null event handlers before close in cleanup

**Approach:** In cleanup, remove `onclose/onerror/onmessage` handlers before `close()`.

**Pros:**
- Minimal code changes

**Cons:**
- More brittle than explicit lifecycle guard
- Easy to regress when refactoring

**Effort:** Small

**Risk:** Medium

---

### Option 3: Migrate to reconnect controller abstraction

**Approach:** Encapsulate socket lifecycle/retry behavior in a controller with start/stop semantics.

**Pros:**
- Reusable and testable behavior

**Cons:**
- Larger refactor than needed

**Effort:** Medium

**Risk:** Medium

## Recommended Action
Implemented Option 1: added `shouldReconnectRef` lifecycle guard and disabled reconnect scheduling during teardown.

## Technical Details

**Affected files:**
- `frontend/src/hooks/useWebSocket.ts:42`
- `frontend/src/hooks/useWebSocket.ts:74`

## Resources

- **Commit under review:** `358083d`

## Acceptance Criteria

- [x] WebSocket cleanup does not schedule reconnect timers
- [x] No reconnect attempts happen after component unmount
- [x] Existing reconnect behavior still works for real network disconnects
- [x] Hook behavior is covered by at least one lifecycle-focused test or validated manual scenario

## Work Log

### 2026-02-17 - Review finding creation

**By:** Codex

**Actions:**
- Traced `connect`/`onclose`/cleanup control flow
- Identified missing teardown guard for reconnect scheduling

**Learnings:**
- Close-on-unmount and reconnect logic need explicit separation of intentional vs accidental disconnects

### 2026-02-17 - Fix implemented

**By:** Codex

**Actions:**
- Added `shouldReconnectRef` in `frontend/src/hooks/useWebSocket.ts`
- Set reconnect guard true on mount and false during cleanup
- Prevented `onclose` from scheduling retries when teardown is intentional

**Learnings:**
- Reconnect behavior is more predictable when socket close intent is explicitly tracked

## Notes

- This is reliability-focused and should be fixed before broader UI expansion tasks.
