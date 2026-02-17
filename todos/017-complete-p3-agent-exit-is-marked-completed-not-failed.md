---
status: complete
priority: p3
issue_id: "017"
tags: [code-review, lifecycle, reliability]
dependencies: []
---

# Distinguish abrupt agent exit from successful completion

When the tmux session disappears, monitor unconditionally marks the session as `completed`.

## Problem Statement

TASK-12 notes explicitly call out immediate agent exit detection and failure reporting. Current monitor behavior treats all tmux disappearance as successful completion, masking crashes and startup failures.

## Findings

- `internal/session/monitor.go:102` checks tmux session existence.
- `internal/session/monitor.go:103` persists status `completed` immediately when session is gone.
- No heuristic or state check distinguishes clean completion signals from abrupt termination.

## Proposed Solutions

### Option 1: Gate `completed` on explicit done signals

**Approach:** Mark `completed` only when marker file exists; otherwise use `failed` if tmux disappears unexpectedly.

**Pros:**
- Better status accuracy.

**Cons:**
- Requires clear definition of expected completion signals.

**Effort:** Small/Medium

**Risk:** Medium

### Option 2: Add terminal-exit reason capture from tmux/process logs

**Approach:** Inspect exit reason/status (when available) and map to `completed` vs `failed`.

**Pros:**
- Most accurate semantics.

**Cons:**
- More implementation complexity.

**Effort:** Medium

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `internal/session/monitor.go`

## Acceptance Criteria

- [x] Abrupt early agent exits can be surfaced as `failed`.
- [x] Expected completion paths still mark `completed`.
- [x] Added tests cover both abrupt exit and normal completion.

## Work Log

### 2026-02-17 - Implemented

**By:** Codex

**Actions:**
- Added `statusOnSessionExit()` in `internal/session/monitor.go`:
- `completed` when marker file exists.
- `waiting_review` when `[READY_FOR_REVIEW]` commit is present.
- `failed` for abrupt exits without completion signals.
- Added `TestMonitorRunMarksFailedWhenSessionDisappearsWithoutCompletionSignal`.

## Resources

- `docs/TASK-12-session-lifecycle.md`
- `internal/session/monitor.go`
