---
status: complete
priority: p2
issue_id: "007"
tags: [code-review, reliability, parser, quality]
dependencies: []
resolution: "Fixed idle timing: working (<=3s), idle (>=30s), removed 3s false-idle transition"
---

# Status Transition Violates 30 Second Idle Specification

## Problem Statement

Parser status logic marks windows idle after ~3 seconds, but spec requires `idle` only after 30+ seconds of no output.

## Findings

- Spec states: `working` within 3s, `idle` after 30s+ (`TASK-03-output-parser.md:157-160`).
- `internal/parser/parser.go:230` checks `> 3*time.Second`.
- `internal/parser/parser.go:232` sets `StatusIdle` at that threshold.
- This collapses `working` and `idle` semantics and removes intended intermediate behavior.

## Proposed Solutions

### Option 1: Align status thresholds with spec

**Approach:** Keep `working` for <=3s, introduce/maintain non-idle state for 3-30s, set `idle` only after >30s.

**Pros:**
- Matches task contract
- Produces stable UX semantics

**Cons:**
- Requires status logic adjustment and tests

**Effort:** 1-3 hours

**Risk:** Low

---

### Option 2: Amend spec to current behavior

**Approach:** Change task text to idle-after-3s.

**Pros:**
- No code changes

**Cons:**
- Reduces fidelity of presence/status model

**Effort:** 30 minutes

**Risk:** Medium

---

### Option 3: Keep as-is

**Approach:** No changes.

**Pros:**
- Zero effort

**Cons:**
- Spec non-compliance remains

**Effort:** 0

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `internal/parser/parser.go:230`
- `internal/parser/parser.go:232`
- `internal/parser/parser_test.go:321`

**Related components:**
- Window status indicators in UI

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Spec target:** `TASK-03-output-parser.md`

## Acceptance Criteria

- [ ] Idle is set only after 30+ seconds of no output
- [ ] Working semantics remain valid for recent output
- [ ] Unit tests explicitly verify 3s and 30s boundaries

## Work Log

### 2026-02-16 - Task 03 Implementation Review

**By:** Codex

**Actions:**
- Compared spec status definitions to current update logic
- Identified 3-second idle transition mismatch

**Learnings:**
- Time-threshold mismatches are easy to miss without boundary tests

## Notes

- Important behavior mismatch, not security-critical.
