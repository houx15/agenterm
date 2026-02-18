---
status: complete
priority: p1
issue_id: "028"
tags: [code-review, automation, hub, concurrency]
dependencies: []
---

# Human takeover detaches too early with multiple clients

## Problem Statement

Human takeover is toggled per attach/detach callback, but the current implementation does not track attach reference counts across clients. If two clients are attached to the same session and one disconnects, takeover is cleared even though another human is still attached. This violates the automation pause contract and can resume coordinator/auto-commit unexpectedly.

## Findings

- `internal/hub/client.go:153` emits detach for every attached session on client disconnect, without global session-level client counting.
- `cmd/agenterm/main.go:464` immediately clears takeover and resumes automation on any detach callback.
- There is no session-level “active viewers” counter in `hub.Hub`, so detach semantics are non-idempotent across multiple clients.

## Proposed Solutions

### Option 1: Hub-level reference counting (recommended)

**Approach:** Track `sessionID -> attachedClientCount` in `hub.Hub`; only fire `onTerminalAttach` on transition `0->1` and `onTerminalDetach` on transition `1->0`.

**Pros:**
- Correct semantics for multi-client viewing.
- Keeps takeover logic centralized and deterministic.

**Cons:**
- Adds shared mutable state in hub.

**Effort:** 2-4 hours

**Risk:** Medium

---

### Option 2: Session manager idempotency guard

**Approach:** Keep a persistent attach counter in DB/session manager and ignore detach until counter reaches zero.

**Pros:**
- Survives process restarts.

**Cons:**
- Adds DB writes on websocket attach churn.
- More invasive than needed for this layer.

**Effort:** 4-8 hours

**Risk:** Medium

## Recommended Action

**To be filled during triage.**

## Technical Details

**Affected files:**
- `internal/hub/client.go:89`
- `internal/hub/client.go:153`
- `cmd/agenterm/main.go:447`
- `cmd/agenterm/main.go:464`

**Related components:**
- Human takeover state transitions
- Coordinator pause/resume
- AutoCommitter pause/resume

**Database changes (if any):**
- Migration needed? No

## Resources

- **Branch:** `feature/automation`
- **Spec:** `docs/TASK-16-automation.md`

## Acceptance Criteria

- [ ] With two websocket clients attached to one session, disconnecting one client does not clear takeover.
- [ ] Takeover clears only when the last attached client detaches.
- [ ] Coordinator/autocommit remain paused while at least one human is attached.
- [ ] Add regression tests for multi-client attach/detach semantics.

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed attach/detach lifecycle in hub and main wiring.
- Traced callback flow from client disconnect to takeover clear.
- Identified missing session-level reference counting.

**Learnings:**
- Current behavior is correct for single client but unsafe for concurrent viewers.

## Notes

- Severity is P1 because it can silently re-enable autonomous actions during active human takeover.
