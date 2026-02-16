---
status: complete
priority: p1
issue_id: "001"
tags: [code-review, reliability, tmux, quality]
dependencies: []
resolution: "Added tmux has-session preflight check before attach-session"
---

# Gateway Start Does Not Fail Fast On Missing Session

## Problem Statement

`Gateway.Start` can return success even when the target tmux session does not exist, causing the application to continue with a dead gateway and delayed failures.

## Findings

- `internal/tmux/gateway.go:46` only checks `process.Start()` errors for session-not-found cases.
- In tmux, a non-existent session typically fails after process start (runtime stderr/exit), so `Start()` can still return `nil`.
- `internal/tmux/gateway.go:53` and `internal/tmux/gateway.go:64` only send `list-windows`; they do not verify a successful `%begin/%end` response before returning.
- `internal/tmux/gateway.go:109` closes event channel on reader exit, but no explicit startup failure propagates to caller.

## Proposed Solutions

### Option 1: Explicit startup handshake validation

**Approach:** During `Start`, wait for successful `list-windows` command completion (or timeout) and return a clear error if command fails/EOF occurs first.

**Pros:**
- Deterministic startup semantics
- Catches missing session immediately

**Cons:**
- Requires response-correlation plumbing

**Effort:** 3-5 hours

**Risk:** Low

---

### Option 2: Preflight session existence check

**Approach:** Run `tmux has-session -t <session>` before `attach-session`.

**Pros:**
- Simple and explicit
- Fast failure with clear message

**Cons:**
- Additional tmux process call
- Still need runtime handling if session disappears right after check

**Effort:** 1-2 hours

**Risk:** Low

---

### Option 3: Keep current behavior

**Approach:** Rely on downstream event absence/errors.

**Pros:**
- No code changes

**Cons:**
- Poor UX and hard-to-debug startup failures

**Effort:** 0

**Risk:** High

## Recommended Action


## Technical Details

**Affected files:**
- `internal/tmux/gateway.go:46`
- `internal/tmux/gateway.go:53`
- `internal/tmux/gateway.go:109`
- `cmd/agenterm/main.go:52`

**Related components:**
- Gateway lifecycle
- Application startup path

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Review scope:** Task-01/02 implementation code

## Acceptance Criteria

- [ ] `Start()` returns non-nil error for non-existent tmux session within bounded timeout
- [ ] Error message includes actionable remediation
- [ ] Integration test verifies fail-fast behavior without relying on delayed reader shutdown

## Work Log

### 2026-02-16 - Initial Code Review Finding

**By:** Codex

**Actions:**
- Reviewed gateway startup flow and error propagation paths
- Identified missing startup success/failure handshake

**Learnings:**
- Process start success is not equivalent to protocol/session readiness

## Notes

- This is merge-blocking for reliable startup behavior.
