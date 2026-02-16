---
status: complete
priority: p2
issue_id: "012"
tags: [code-review, frontend, ux, quality]
dependencies: []
resolution: "Fixed sendInput to show optimistic bubble with text stripped of trailing newline"
---

# User Input Is Not Rendered Optimistically For Command Sends

## Problem Statement

Spec requires local right-aligned user input bubbles on send, but current logic suppresses rendering for normal command sends because UI appends newline before dispatch.

## Findings

- Input handlers always call `sendInput(text + '\n')` (`web/index.html:899`, `web/index.html:914`).
- Optimistic render only occurs when text does not end with newline (`web/index.html:822`).
- Therefore standard Enter/send actions do not produce local user input bubbles.

## Proposed Solutions

### Option 1: Render using trimmed display text while sending newline payload

**Approach:** Always render optimistic user message for typed input, but send protocol keys with trailing newline.

**Pros:**
- Matches spec and user expectations
- Keeps backend command semantics unchanged

**Cons:**
- Need to avoid duplicate echo confusion in future

**Effort:** 30-60 minutes

**Risk:** Low

---

### Option 2: Add explicit `isAction` parameter

**Approach:** Differentiate prompt-button/control-key sends from typed commands; only suppress optimistic bubble for control-only actions.

**Pros:**
- Cleaner intent separation

**Cons:**
- Slightly broader refactor in send path

**Effort:** 1-2 hours

**Risk:** Low

---

### Option 3: Keep current behavior

**Approach:** No changes.

**Pros:**
- No implementation effort

**Cons:**
- Misses a stated Task-05 acceptance behavior

**Effort:** 0

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `web/index.html:899`
- `web/index.html:914`
- `web/index.html:822`

**Related components:**
- Chat UX feedback loop
- Input-send interaction model

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Spec target:** `TASK-05-frontend-ui.md`

## Acceptance Criteria

- [ ] Typed command sends show right-aligned optimistic user bubble
- [ ] Backend still receives newline-terminated keys
- [ ] Prompt action buttons still function correctly

## Work Log

### 2026-02-16 - Task 05 Implementation Review

**By:** Codex

**Actions:**
- Traced send-button/Enter key path to optimistic render guard
- Confirmed newline guard suppresses user bubble for normal sends

**Learnings:**
- Transport formatting and display formatting should be separated

## Notes

- Important task-compliance UX gap.
