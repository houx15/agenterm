---
status: complete
priority: p1
issue_id: "003"
tags: [code-review, api, sessions, reliability]
dependencies: []
---

# Session Creation Leaves Orphan DB Rows

`POST /api/tasks/{id}/sessions` writes the DB record before tmux window creation and does not rollback when tmux creation fails.

## Problem Statement

Session creation should result in a usable session. When `NewWindow` fails, API returns error but already-persisted session rows remain. This creates inconsistent state and broken session listings.

## Findings

- DB row is created at `internal/api/sessions.go:85`.
- tmux window is created at `internal/api/sessions.go:92`.
- On failure at line 92, handler returns error without deleting the created session.
- Contract expectation in task spec implies successful create path should produce valid session + tmux linkage.

## Proposed Solutions

### Option 1: Create tmux first, persist DB after success

**Approach:** Allocate tmux window first, then insert session record with resolved `tmux_window_id`.

**Pros:**
- Avoids orphan sessions entirely
- Simple failure model

**Cons:**
- Need window naming strategy without DB ID dependency

**Effort:** Medium

**Risk:** Low

---

### Option 2: Keep order but add compensating rollback

**Approach:** If tmux creation fails, delete inserted session row in same request path.

**Pros:**
- Minimal structural change
- Preserves current ID-based naming flow

**Cons:**
- Compensation failures need additional handling/logging

**Effort:** Small

**Risk:** Medium

---

### Option 3: Wrap with DB transaction + retryable tmux step

**Approach:** Track creation lifecycle state and retry/finalize asynchronously.

**Pros:**
- Robust under transient tmux issues

**Cons:**
- Adds complexity and state machine overhead

**Effort:** Large

**Risk:** Medium

## Recommended Action


## Technical Details

Affected files:
- `internal/api/sessions.go:77`
- `internal/api/sessions.go:85`
- `internal/api/sessions.go:92`

## Resources

- Spec: `docs/TASK-08-rest-api.md`
- Commit: `8d62614664c2502ddf0c95a52b034d9ac20c4bfd`

## Acceptance Criteria

- [ ] No persisted session record exists when tmux window creation fails
- [ ] Error path is covered by API tests
- [ ] Session list/detail endpoints never return half-created sessions

## Work Log

### 2026-02-17 - Review Discovery

**By:** Codex

**Actions:**
- Traced create-session control flow and failure exits
- Confirmed missing compensation behavior
- Classified as P1 reliability/data-consistency issue

**Learnings:**
- Current ordering couples persistent state to external side effects without rollback.

## Notes

- Impacts both users and orchestrator retries.
