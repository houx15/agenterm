---
status: complete
priority: p2
issue_id: "014"
tags: [code-review, architecture, lifecycle, parser]
dependencies: []
---

# Align lifecycle output capture with TASK-12 parser/gateway requirement

Current lifecycle monitoring captures terminal output by polling `tmux capture-pane` and diffing raw lines, instead of feeding per-event output through the parser pipeline.

## Problem Statement

`docs/TASK-12-session-lifecycle.md` Step 6 requires session output storage to be fed from Gateway events through Parser so status/output semantics stay consistent with terminal event processing. Current implementation bypasses that path and applies ad-hoc prompt checks on captured text.

## Findings

- `internal/session/monitor.go:106` reads output with `capturePaneFn` on each poll tick.
- `internal/session/monitor.go:127` stores raw diffed lines directly into the ring buffer.
- `internal/session/monitor.go:177` performs prompt detection against regexes on buffered raw lines rather than parser-classified messages.
- No integration from `hub` terminal stream to `session.Monitor` ring buffer is implemented in this commit range.

## Proposed Solutions

### Option 1: Wire monitor input from gateway/hub terminal events

**Approach:** Subscribe lifecycle manager/monitor to parsed terminal events, update ring buffer from those events, and use parser classifications for prompt detection.

**Pros:**
- Matches TASK-12 architecture exactly.
- Avoids polling drift and capture-pane diff edge cases.

**Cons:**
- Requires plumbing from hub/gateway to session manager.

**Effort:** Medium

**Risk:** Medium

### Option 2: Keep polling, but pass captured output through parser and clearly document deviation

**Approach:** Parse captured lines through parser before classification, and update task/spec docs to mark polling fallback behavior.

**Pros:**
- Smaller change.

**Cons:**
- Still deviates from gateway event feed requirement.

**Effort:** Small

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `internal/session/monitor.go`
- `internal/session/manager.go`
- `internal/hub/hub.go`

## Acceptance Criteria

- [x] Ring buffer entries originate from parser-processed terminal events.
- [x] Prompt/waiting detection uses parser classifications rather than raw regex-only heuristic.
- [x] Output retrieval remains stable for `GET /api/sessions/{id}/output`.

## Work Log

### 2026-02-17 - Implemented

**By:** Codex

**Actions:**
- Added `Manager.ObserveParsedOutput(...)` and wired parser message stream in `cmd/agenterm/main.go` to feed monitor ring buffers.
- Removed `capture-pane` output polling path from `Monitor.Run`.
- Updated prompt detection to depend on parser class `prompt` + shell prompt pattern.

## Resources

- `docs/TASK-12-session-lifecycle.md`
- `internal/session/monitor.go`
- Commits: `b29a91a`, `b2f2bb6`
