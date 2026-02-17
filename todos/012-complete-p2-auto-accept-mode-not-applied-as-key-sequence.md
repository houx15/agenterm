---
status: complete
priority: p2
issue_id: "012"
tags: [code-review, quality, lifecycle]
dependencies: []
---

# Implement real auto-accept key handling

Session startup does not send a configurable key sequence for `auto_accept_mode`; it only sends newline when mode equals `supported`.

## Problem Statement

TASK-12 Step 2 requires sending auto-accept key sequence after startup when configured. Current implementation checks only `mode == "supported"` and sends `"\n"`, which is not a general key sequence mechanism.

## Findings

- `internal/session/manager.go:213` starts auto-accept branch for non-empty mode.
- `internal/session/manager.go:216` only handles literal `"supported"`.
- `internal/session/manager.go:217` sends only newline, no mapping for values like `shift+tab` described in spec.

## Proposed Solutions

### Option 1: Treat `auto_accept_mode` as explicit key sequence

**Approach:** Parse `auto_accept_mode` into tmux-sendable key sequence and send via `SendKeys`/`SendRaw` after delay.

**Pros:**
- Directly matches task spec intent.
- Future-proof for per-agent differences.

**Cons:**
- Requires key normalization/mapping logic.

**Effort:** 2-4 hours

**Risk:** Medium

---

### Option 2: Introduce enum + mapping table

**Approach:** Keep config values as semantic modes (`supported`, `optional`, etc.) and maintain internal mapping per agent/tool to actual sequence.

**Pros:**
- Cleaner config semantics.
- Centralized behavior control.

**Cons:**
- Additional maintenance and per-agent metadata needed.

**Effort:** 3-5 hours

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `internal/session/manager.go`
- `configs/agents/*.yaml`
- `docs/TASK-12-session-lifecycle.md` (clarify schema if needed)

## Resources

- **Commit:** `b29a91a`
- **Task:** `docs/TASK-12-session-lifecycle.md`

## Acceptance Criteria

- [ ] Agent with configured auto-accept emits expected key sequence after startup delay.
- [ ] Behavior covered by tests for at least two modes.
- [ ] Unknown mode handling is explicit and logged.

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Traced CreateSession startup path for auto-accept behavior.
- Verified no configurable key sequence support exists.

**Learnings:**
- Current behavior is binary and newline-only, which under-specifies intended automation.

## Notes

- Prefer deterministic mapping and avoid magic string comparisons in runtime logic.
