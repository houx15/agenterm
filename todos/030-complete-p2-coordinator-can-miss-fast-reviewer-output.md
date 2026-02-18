---
status: complete
priority: p2
issue_id: "030"
tags: [code-review, automation, coordinator, race-condition]
dependencies: []
---

# Coordinator can miss fast reviewer output due to timestamp race

## Problem Statement

The coordinator records `since := time.Now()` after sending the review prompt. If the reviewer emits output immediately after send but before `since` is captured, `GetOutputSince` can miss that output, causing delayed/failed feedback processing.

## Findings

- `internal/automation/coordinator.go:211` sends the prompt.
- `internal/automation/coordinator.go:215` captures `since` only afterward.
- `waitForReviewerDecision` depends exclusively on output entries at/after that timestamp.

## Proposed Solutions

### Option 1: Capture `since` before send (recommended)

**Approach:** Set `since` just before `sendCommand` and pass that into decision wait.

**Pros:**
- Minimal code change.
- Eliminates race window.

**Cons:**
- Includes potentially stale output unless session output is scoped by prompt marker.

**Effort:** 1 hour

**Risk:** Low

---

### Option 2: Prompt correlation marker

**Approach:** Include a unique marker in prompt and parse only output after the marker appears.

**Pros:**
- Strong association between prompt and reply.

**Cons:**
- More implementation complexity.

**Effort:** 2-4 hours

**Risk:** Medium

## Recommended Action

**To be filled during triage.**

## Technical Details

**Affected files:**
- `internal/automation/coordinator.go:211`
- `internal/automation/coordinator.go:215`

**Related components:**
- Session output polling
- Reviewer decision loop

**Database changes (if any):**
- Migration needed? No

## Resources

- **Branch:** `feature/automation`
- **Spec:** `docs/TASK-16-automation.md`

## Acceptance Criteria

- [ ] Reviewer responses produced immediately after prompt send are never dropped by `since` filtering.
- [ ] Add regression test that simulates near-instant reviewer output.
- [ ] Coordinator review iteration completes without unnecessary timeout on fast responses.

## Work Log

### 2026-02-17 - Initial Discovery

**By:** Codex

**Actions:**
- Traced control flow around prompt send and output polling.
- Identified ordering race between send and timestamp capture.

**Learnings:**
- The race is small but can intermittently break review-loop reliability.

## Notes

- Severity P2 due to intermittent behavior and automation loop instability.
