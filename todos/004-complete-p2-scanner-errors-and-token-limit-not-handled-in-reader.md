---
status: complete
priority: p2
issue_id: "004"
tags: [code-review, reliability, performance, tmux]
dependencies: []
resolution: "Increased scanner buffer to 1MB; added error logging on scanner.Err()"
---

# Scanner Errors And Token Limit Are Not Handled In Reader

## Problem Statement

The tmux reader loop uses default `bufio.Scanner` limits and ignores scanner errors, which can silently terminate event streaming on long output lines.

## Findings

- `internal/tmux/gateway.go:75` creates scanner with default max token size.
- `internal/tmux/gateway.go:79` loops on `scanner.Scan()` but never checks `scanner.Err()` afterward.
- On oversized token, scan stops and channels are closed (`internal/tmux/gateway.go:109`) without propagating failure cause.

## Proposed Solutions

### Option 1: Configure scanner buffer and propagate errors

**Approach:** Set `scanner.Buffer(...)` to larger bound and report `scanner.Err()` via dedicated error channel/logging path.

**Pros:**
- Minimal change
- Improves observability and resilience

**Cons:**
- Still bounded by configured max

**Effort:** 1-2 hours

**Risk:** Low

---

### Option 2: Replace scanner with `bufio.Reader`

**Approach:** Implement line framing with `ReadString('\n')`/chunked handling to avoid scanner token-limit behavior.

**Pros:**
- Better control over large payload handling

**Cons:**
- More code complexity

**Effort:** 3-5 hours

**Risk:** Medium

---

### Option 3: Keep current behavior

**Approach:** No changes.

**Pros:**
- Zero effort

**Cons:**
- Silent stream failure risk remains

**Effort:** 0

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `internal/tmux/gateway.go:75`
- `internal/tmux/gateway.go:79`
- `internal/tmux/gateway.go:109`

**Related components:**
- Event pipeline reliability
- Long-running session stability

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Review scope:** Task-01/02 implementation code

## Acceptance Criteria

- [ ] Reader handles large tmux lines without silent shutdown
- [ ] Scanner/read errors are surfaced with actionable context
- [ ] Test case covers long-line behavior

## Work Log

### 2026-02-16 - Initial Code Review Finding

**By:** Codex

**Actions:**
- Inspected reader loop termination paths and error handling
- Identified unhandled scanner failure mode

**Learnings:**
- Long-running protocol readers must surface transport/parser failures explicitly

## Notes

- Important reliability issue for production-like output volume.
