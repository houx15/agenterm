---
status: complete
priority: p3
issue_id: "016"
tags: [code-review, reliability, lifecycle]
dependencies: []
---

# Harden tmux session uniqueness strategy per TASK-12 note

Current unique-name logic only retries a fixed set of numeric suffixes and can fail under repeated collisions.

## Problem Statement

TASK-12 notes call for UUID suffixing when needed to ensure tmux session name uniqueness. Current code retries `-01` to `-07` only, then fails, which can block session creation in collision-heavy environments.

## Findings

- `internal/session/manager.go:354` loops with at most 8 attempts.
- `internal/session/manager.go:357` appends deterministic two-digit suffixes.
- `internal/session/manager.go:371` returns failure after fixed retries instead of fallback UUID/random suffix.

## Proposed Solutions

### Option 1: Add UUID/random fallback after bounded retries

**Approach:** Keep short deterministic retries, then append short UUID segment and retry once.

**Pros:**
- Preserves readable names in common case.
- Robust under collisions.

**Cons:**
- Slightly less human-readable in fallback case.

**Effort:** Small

**Risk:** Low

### Option 2: Always suffix with short UUID

**Approach:** Always append stable short random token to generated base name.

**Pros:**
- Strong uniqueness guarantee.

**Cons:**
- Session names less predictable.

**Effort:** Small

**Risk:** Low

## Recommended Action


## Technical Details

**Affected files:**
- `internal/session/manager.go`

## Acceptance Criteria

- [x] Session creation does not fail due to predictable suffix exhaustion.
- [x] Naming remains within tmux length constraints.
- [x] Behavior is covered by unit tests.

## Work Log

### 2026-02-17 - Implemented

**By:** Codex

**Actions:**
- Updated `createTmuxSession` in `internal/session/manager.go` to use random ID suffix fallback (`db.NewID()` short segment) after deterministic retries.
- Preserved tmux length constraints while removing fixed-suffix exhaustion risk.

## Resources

- `docs/TASK-12-session-lifecycle.md`
- `internal/session/manager.go`
