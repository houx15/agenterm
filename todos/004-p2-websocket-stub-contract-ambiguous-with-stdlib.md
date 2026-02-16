---
status: resolved
priority: p2
issue_id: "004"
tags: [code-review, architecture, quality]
dependencies: []
resolution: "Specified /ws returns 501 Not Implemented with JSON error message"
---

# Clarify WebSocket Stub Contract Under Stdlib-Only Constraint

## Problem Statement

The task requests a WebSocket upgrade endpoint while also enforcing stdlib-only dependencies, but does not define what "stub" behavior means under that constraint.

## Findings

- `TASK-01-project-scaffold.md:22` says "WebSocket upgrade endpoint (stub)".
- `TASK-01-project-scaffold.md:72` says "just accept and close for now".
- `TASK-01-project-scaffold.md:106` requires no external dependencies.
- Go stdlib has no first-class websocket server API; without a defined fallback behavior, contributors may implement incompatible stubs.

## Proposed Solutions

### Option 1: Redefine endpoint as plain HTTP placeholder

**Approach:** Specify `/ws` returns `501 Not Implemented` with clear JSON/text message in task 01.

**Pros:**
- Fully compatible with stdlib-only requirement
- Removes ambiguity

**Cons:**
- Not a real handshake stub

**Effort:** 15-30 minutes

**Risk:** Low

---

### Option 2: Allow one websocket dependency in task 01

**Approach:** Amend acceptance criteria to allow a minimal websocket library for handshake and close.

**Pros:**
- Real protocol behavior early

**Cons:**
- Violates current simplicity goal

**Effort:** 30-60 minutes

**Risk:** Medium

---

### Option 3: Split websocket work into task 02/04 only

**Approach:** Remove `/ws` implementation from task 01 and leave route planning note only.

**Pros:**
- Preserves scaffold scope
- Aligns with dependency constraints

**Cons:**
- Requires cross-task text edits

**Effort:** 15-30 minutes

**Risk:** Low

## Recommended Action


## Technical Details

**Affected files:**
- `TASK-01-project-scaffold.md:22`
- `TASK-01-project-scaffold.md:72`
- `TASK-01-project-scaffold.md:106`

**Related components:**
- HTTP routing contract
- Task 02/04 websocket roadmap

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Review target:** `TASK-01-project-scaffold.md`

## Acceptance Criteria

- [ ] Task defines exact `/ws` behavior for task 01 (status code/body or protocol behavior)
- [ ] Stdlib-only requirement and endpoint behavior are compatible
- [ ] Future websocket tasks reference this contract without conflict

## Work Log

### 2026-02-16 - Initial Review Finding

**By:** Codex

**Actions:**
- Reviewed websocket-related lines against dependency constraints
- Documented ambiguity and options

**Learnings:**
- Early protocol ambiguity causes avoidable branch divergence in greenfield implementations

## Notes

- Keep task 01 intentionally thin and explicit to reduce early churn.
