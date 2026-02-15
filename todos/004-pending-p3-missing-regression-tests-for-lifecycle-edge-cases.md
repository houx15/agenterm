---
status: pending
priority: p3
issue_id: "004"
tags: [code-review, tests, websocket, quality]
dependencies: []
---

# Add Lifecycle Edge-Case Regression Tests

## Problem Statement

Current tests cover happy-path behavior but miss lifecycle edge cases where hub startup/shutdown ordering and initial sync contract can fail.

## Findings

- `internal/hub/hub_test.go` has no test for websocket connection before `Run()` starts.
- No test asserts initial empty `WindowsMessage` is sent to newly connected clients.
- No test validates disconnect/shutdown behavior under high client counts.
- Impact: critical lifecycle regressions can pass CI undetected.

## Proposed Solutions

### Option 1: Extend Existing `hub_test.go` with 3 Targeted Tests

**Approach:** Add focused tests for pre-`Run()` connect, empty windows snapshot handshake, and high-count shutdown cleanup.

**Pros:**
- Fastest path.
- Keeps test surface close to implementation.

**Cons:**
- `hub_test.go` is already large and may become harder to navigate.

**Effort:** 1-3 hours

**Risk:** Low

---

### Option 2: Split by Concern into Separate Test Files

**Approach:** Move lifecycle tests into `lifecycle_test.go` and keep protocol/rate-limit tests separate.

**Pros:**
- Better long-term test organization.
- Easier targeted test runs.

**Cons:**
- Slightly higher immediate refactor effort.

**Effort:** 2-4 hours

**Risk:** Low

## Recommended Action

To be filled during triage.

## Technical Details

**Affected files:**
- `internal/hub/hub_test.go`
- Optional: new `internal/hub/lifecycle_test.go`

**Related components:**
- Hub lifecycle
- WebSocket handshake contract

**Database changes (if any):**
- No

## Resources

- **Current tests:** `internal/hub/hub_test.go`
- **Task spec:** `TASK-04-websocket-hub.md`

## Acceptance Criteria

- [ ] Tests cover connection behavior before/without `Run()`.
- [ ] Tests validate deterministic initial handshake message for empty windows.
- [ ] Tests cover shutdown with many clients and verify no blocked teardown path.
- [ ] New tests pass in CI and fail when edge-case protections are removed.

## Work Log

### 2026-02-15 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed current hub tests against task acceptance criteria.
- Enumerated uncovered lifecycle and edge-condition scenarios.

**Learnings:**
- Core behavior is well-covered for normal flow, but lifecycle-order edge cases are under-tested.
