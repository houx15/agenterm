---
status: complete
priority: p1
issue_id: "006"
tags: [code-review, architecture, parser, tmux, integration]
dependencies: []
resolution: "Added pane-to-window mapping in Gateway; lookup WindowID on output events"
---

# Output Events Lack WindowID Breaking Per-Window Parser Contract

## Problem Statement

`parser.Parser` is designed to accumulate output per window, but current tmux output events do not populate `Event.WindowID`, so parser messages are keyed to an empty/incorrect window ID.

## Findings

- `internal/tmux/protocol.go:94` parses `%output` into `PaneID` and `Data`, but does not set `WindowID`.
- `internal/tmux/gateway.go:160` only updates `windows` on lifecycle events; no pane→window mapping is maintained.
- `cmd/agenterm/main.go:99` feeds parser via `psr.Feed(event.WindowID, event.Data)`.
- Result: output messages can be emitted under empty `WindowID`, violating the task objective of per-window segmentation/classification.

## Proposed Solutions

### Option 1: Maintain pane-to-window mapping in gateway

**Approach:** Track pane ownership from tmux metadata and set `event.WindowID` before broadcasting parser input.

**Pros:**
- Correct end-to-end contract
- Supports multi-window behavior accurately

**Cons:**
- Requires additional tmux state synchronization

**Effort:** 4-8 hours

**Risk:** Medium

---

### Option 2: Switch parser feed to pane key temporarily

**Approach:** Feed parser with `PaneID` as key until full mapping is implemented.

**Pros:**
- Restores deterministic partitioning quickly

**Cons:**
- Diverges from window-oriented UI contract

**Effort:** 1-2 hours

**Risk:** Medium

---

### Option 3: Keep current integration

**Approach:** No changes.

**Pros:**
- Zero effort

**Cons:**
- Core parser objective is not satisfied

**Effort:** 0

**Risk:** High

## Recommended Action


## Technical Details

**Affected files:**
- `internal/tmux/protocol.go:94`
- `internal/tmux/gateway.go:160`
- `cmd/agenterm/main.go:99`

**Related components:**
- Parser per-window buffer model
- Hub/UI message routing

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Spec target:** `TASK-03-output-parser.md`

## Acceptance Criteria

- [ ] Output messages always carry a valid window identifier
- [ ] Parser buffers are partitioned by actual window, not empty key
- [ ] Integration test verifies output from two windows is separated correctly

## Work Log

### 2026-02-16 - Task 03 Implementation Review

**By:** Codex

**Actions:**
- Traced output event path from tmux parser to parser feed
- Verified missing `WindowID` propagation on output events

**Learnings:**
- Per-window parsing requires stable upstream window attribution, not just pane payload

## Notes

- Merge-blocking for claiming Task 03 end-to-end success.
