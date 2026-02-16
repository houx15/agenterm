---
status: resolved
priority: p3
issue_id: "005"
tags: [code-review, quality, testing]
dependencies: []
resolution: "Added server_test.go smoke test requirement for root handler and /ws endpoint"
---

# Add Minimal Automated Test Deliverables

## Problem Statement

The task includes runtime verification requirements but does not require any committed automated tests, reducing long-term confidence in scaffold behavior.

## Findings

- `TASK-01-project-scaffold.md:93` lists testing requirements primarily as manual/runtime checks.
- No explicit requirement exists for unit tests or integration tests in this task's deliverables.
- Critical bootstrap behaviors (token generation, config persistence, graceful shutdown) are prone to regression.

## Proposed Solutions

### Option 1: Require smoke integration test only

**Approach:** Add one `go test` smoke case for server startup and root handler response.

**Pros:**
- Low overhead
- Immediate CI signal

**Cons:**
- Limited coverage depth

**Effort:** 1-2 hours

**Risk:** Low

---

### Option 2: Require targeted unit tests for config and shutdown

**Approach:** Add unit tests for token generation/persistence and shutdown path.

**Pros:**
- Covers highest-risk scaffolding behaviors

**Cons:**
- More setup than single smoke test

**Effort:** 2-4 hours

**Risk:** Low

---

### Option 3: Keep tests deferred to later tasks

**Approach:** Leave task 01 unchanged and rely on later feature tests.

**Pros:**
- Fastest short-term delivery

**Cons:**
- Raises regression risk and hidden technical debt

**Effort:** 0

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `TASK-01-project-scaffold.md:93`
- `TASK-01-project-scaffold.md:101`

**Related components:**
- CI quality gate expectations
- Scaffold stability baseline

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Review target:** `TASK-01-project-scaffold.md`

## Acceptance Criteria

- [ ] Task includes at least one mandatory automated test deliverable
- [ ] CI command for tests is explicitly documented
- [ ] Test scope maps to at least token/config/server lifecycle behavior

## Work Log

### 2026-02-16 - Initial Review Finding

**By:** Codex

**Actions:**
- Reviewed testing and acceptance sections for automation requirements
- Logged missing automated test expectations and options

**Learnings:**
- Early test contracts significantly reduce refactor friction across follow-up tasks

## Notes

- This is a quality improvement item, not a merge blocker by itself.
