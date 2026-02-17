---
status: complete
priority: p2
issue_id: "006"
tags: [code-review, tests, api, quality]
dependencies: []
---

# API Test Coverage Misses Required Negative Cases

The new API test suite validates core happy paths but does not meet several explicit testing requirements from `docs/TASK-08-rest-api.md`.

## Problem Statement

The task spec requires broad valid/invalid endpoint coverage, including auth failure variants and error response paths. Current tests are limited and can miss regressions.

## Findings

- `internal/api/router_test.go` includes auth tests for missing token and valid token only (`internal/api/router_test.go:92`), but not wrong token case.
- No endpoint-level tests for many bad-input paths (e.g., malformed patch payloads, not-found for agents/worktrees/sessions).
- No explicit tests for `PATCH /api/sessions/{id}/takeover`, `/api/sessions/{id}/idle`, and `/api/sessions/{id}/send` error paths.

## Proposed Solutions

### Option 1: Expand table-driven endpoint tests in `router_test.go`

**Approach:** Add table-driven tests for each route covering success + key failure modes.

**Pros:**
- Directly aligns with spec
- Fast and maintainable

**Cons:**
- Test file grows substantially

**Effort:** Medium

**Risk:** Low

---

### Option 2: Split tests per handler file

**Approach:** Create `projects_test.go`, `sessions_test.go`, etc., each with focused happy/negative tests.

**Pros:**
- Better organization
- Easier ownership per module

**Cons:**
- More boilerplate setup

**Effort:** Medium

**Risk:** Low

## Recommended Action


## Technical Details

Affected files:
- `internal/api/router_test.go`
- `docs/TASK-08-rest-api.md`

## Resources

- Commit: `8d62614664c2502ddf0c95a52b034d9ac20c4bfd`

## Acceptance Criteria

- [ ] Wrong-token auth case is tested
- [ ] Every endpoint has at least one invalid-input test
- [ ] 404 paths are covered for missing resources
- [ ] Session sub-endpoints include negative-path tests

## Work Log

### 2026-02-17 - Review Discovery

**By:** Codex

**Actions:**
- Compared current tests with explicit test requirements in task doc
- Identified missing failure-path coverage areas

**Learnings:**
- Current suite verifies functionality but not full contract hardening.
