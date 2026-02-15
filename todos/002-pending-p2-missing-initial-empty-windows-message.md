---
status: pending
priority: p2
issue_id: "002"
tags: [code-review, websocket, protocol, quality]
dependencies: []
---

# Always Send Initial Windows Snapshot

## Problem Statement

The task requires sending an initial `WindowsMessage` when a client connects. Current code only sends this message when the list is non-empty, creating an ambiguous initial state for new clients.

## Findings

- `internal/hub/hub.go:110` gates initial snapshot with `if len(windows) > 0`.
- `TASK-04-websocket-hub.md` Step 3 states: send initial `WindowsMessage` with current window list.
- Impact: frontend cannot distinguish “no windows yet” from “snapshot never received”.

## Proposed Solutions

### Option 1: Remove Length Check and Always Send Snapshot

**Approach:** Marshal and queue `WindowsMessage{Type:"windows", List: windows}` even when `windows` is empty.

**Pros:**
- Fully matches task protocol contract.
- Simple and low-risk.

**Cons:**
- Slightly more traffic (single tiny frame per connection).

**Effort:** 15-30 minutes

**Risk:** Low

---

### Option 2: Add Explicit Ready Message for Empty State

**Approach:** Keep current behavior but send a separate `status`/`ready` message when window list is empty.

**Pros:**
- Can provide richer client semantics.

**Cons:**
- Expands protocol beyond current spec.

**Effort:** 1-2 hours

**Risk:** Medium

## Recommended Action

To be filled during triage.

## Technical Details

**Affected files:**
- `internal/hub/hub.go:110`
- `internal/hub/hub_test.go` (add assertion for empty-list initial message)

**Related components:**
- WebSocket initial sync handshake
- Frontend websocket state initialization

**Database changes (if any):**
- No

## Resources

- **Task spec:** `TASK-04-websocket-hub.md`
- **Implementation:** `internal/hub/hub.go`

## Acceptance Criteria

- [ ] New clients always receive one initial `windows` message immediately after connect.
- [ ] Empty-window case serializes as `{"type":"windows","list":[]}`.
- [ ] Integration test verifies empty-list initial snapshot behavior.

## Work Log

### 2026-02-15 - Initial Discovery

**By:** Codex

**Actions:**
- Compared handshake behavior against task requirements.
- Verified initial message is currently conditional on non-empty list.

**Learnings:**
- Handshake determinism matters for frontend bootstrapping and avoids subtle race-driven UI behavior.
