---
status: complete
priority: p2
issue_id: "009"
tags: [code-review, tests, tmux, hub, api]
dependencies: []
---

# Add Regression Tests For Manager-Backed Session Flows

## Problem Statement

TASK-10 testing requirements call out manager-based multi-session behavior and session routing. Current tests do not sufficiently cover manager-backed API creation and websocket subscription filtering behavior.

## Findings

- `internal/tmux/manager_test.go` covers integration create/attach/destroy (good), but only under `TMUX_INTEGRATION_TEST`.
- `internal/api/router_test.go` still instantiates router with `manager=nil`, so API manager path in `createSession` is untested.
- `internal/hub/client.go` added `subscribe` and `new_session` behavior; no dedicated test verifies subscription filtering semantics.

## Proposed Solutions

### Option 1: Add focused unit tests with test doubles (Recommended)

**Approach:**
- Add fake `sessionManager` to API tests and validate manager path (`CreateSession`, session naming, rollback).
- Add hub tests for subscription filtering (`subscribe` one session vs all).

**Pros:**
- Fast and deterministic.
- Prevents regressions in the most complex new paths.

**Cons:**
- Requires moderate test scaffolding.

**Effort:** Medium

**Risk:** Low

---

### Option 2: Expand integration tests only

**Approach:** Add more tmux/websocket integration tests with real runtime components.

**Pros:**
- Strong end-to-end confidence.

**Cons:**
- Slower and less CI-friendly.
- Environment dependent.

**Effort:** Large

**Risk:** Medium

## Recommended Action

Implemented a mixed strategy: added a hub unit test for subscription filtering and a tmux-gated API integration test for manager-backed session creation.

## Technical Details

Affected files:
- `internal/api/router_test.go`
- `internal/hub/hub_test.go`
- `internal/hub/client.go` (behavior under test)

## Resources

- `docs/TASK-10-multi-session-tmux.md`
- Review commits: `8d8d566`, `530e0c4`

## Acceptance Criteria

- [x] API tests validate manager-backed `createSession` path
- [x] Hub tests verify `subscribe` filtering for `session_id`
- [x] Tests verify fallback "subscribe all" behavior
- [x] Existing test suite remains green

## Work Log

### 2026-02-17 - Review Finding

**By:** Codex

**Actions:**
- Audited test coverage in `internal/api` and `internal/hub` against TASK-10 requirements.
- Identified untested multi-session pathways introduced in latest commits.

**Learnings:**
- Core functionality exists, but targeted regression tests are needed to lock behavior.

### 2026-02-17 - Resolution

**By:** Codex

**Actions:**
- Added `TestBroadcastToClientsRespectsSessionSubscription` in `internal/hub/hub_test.go`.
- Added `TestSessionCreationUsesManagerWhenAvailable` in `internal/api/router_test.go` (guarded by `TMUX_INTEGRATION_TEST`).
- Added `openAPIWithManager` test helper and verified manager-path creation avoids legacy `gw.NewWindow`.
- Ran targeted test commands for `internal/hub`, `internal/api`, and `internal/tmux`.

**Learnings:**
- A lightweight hub helper (`broadcastToClients`) enables deterministic subscription filtering tests without websocket/network setup.

## Notes
