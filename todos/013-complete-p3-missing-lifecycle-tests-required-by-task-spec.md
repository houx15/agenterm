---
status: complete
priority: p3
issue_id: "013"
tags: [code-review, testing, lifecycle]
dependencies: []
---

# Add lifecycle tests required by TASK-12

The commit adds API-focused tests but does not add tests for several explicit lifecycle test requirements in the task spec.

## Problem Statement

TASK-12 requires tests for tmux session naming/creation, idle timeout behavior, session destruction killing tmux session, and takeover propagation. Current tests in the commit validate API request flows but not monitor timeout logic or destruction behavior end-to-end.

## Findings

- `internal/api/router_test.go` adds API endpoint tests but no monitor tests.
- `internal/session/` has no test files after this commit.
- Required scenarios in `docs/TASK-12-session-lifecycle.md` remain unverified by automated tests.

## Proposed Solutions

### Option 1: Add focused unit tests for Manager + Monitor

**Approach:** Introduce fake tmux manager/gateway and fake clock-friendly monitor checks to verify timeout, status transitions, and destroy behavior.

**Pros:**
- Fast and deterministic.
- Directly validates lifecycle logic.

**Cons:**
- Requires mock interfaces or small refactors for testability.

**Effort:** 4-6 hours

**Risk:** Low

---

### Option 2: Add integration tests gated by environment

**Approach:** Add optional tmux-backed tests enabled by env flag to validate real session creation/destroy flows.

**Pros:**
- Strong confidence in real environment behavior.

**Cons:**
- Slower and more brittle in CI without tmux.

**Effort:** 6-10 hours

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `internal/session/manager_test.go` (new)
- `internal/session/monitor_test.go` (new)
- `internal/api/router_test.go` (augment)

## Resources

- **Commit:** `b29a91a`
- **Task:** `docs/TASK-12-session-lifecycle.md`

## Acceptance Criteria

- [ ] Tests cover invalid/valid agent creation paths.
- [ ] Tests cover idle timeout transition.
- [ ] Tests cover destruction calling tmux kill + DB status update.
- [ ] Tests cover takeover propagation and hub status broadcast.

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Compared commit-added tests with TASK-12 testing checklist.
- Identified missing coverage in lifecycle internals.

**Learnings:**
- API tests are present, but monitor/manager behavior remains largely untested.

## Notes

- Keep unit tests as default; gate tmux integration tests behind env flags.
