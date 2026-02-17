---
status: complete
priority: p2
issue_id: "008"
tags: [code-review, ui, websocket, tmux]
dependencies: []
---

# Group Sidebar Entries By Session

## Problem Statement

`docs/TASK-10-multi-session-tmux.md` Step 5 requires: "Sidebar groups windows by session". The current UI renders a flat list of window cards and does not visually group windows under each tmux session.

## Findings

- `web/index.html:905` iterates `state.windows.map(...)` and renders one card per window.
- No grouping structure (session header + nested windows) exists in sidebar rendering.
- This means multi-window sessions are visually mixed with other sessions instead of grouped.

## Proposed Solutions

### Option 1: Introduce grouped session sections (Recommended)

**Approach:** Build a map keyed by `session_id`, render one section per session with its windows underneath.

**Pros:**
- Matches TASK-10 spec directly.
- Scales to many windows per session.
- Enables per-session actions later.

**Cons:**
- Requires UI refactor for selection/unread state.

**Effort:** Medium

**Risk:** Low

---

### Option 2: Keep flat list but add session labels

**Approach:** Keep current card list and prepend each card with session label.

**Pros:**
- Minimal change.

**Cons:**
- Still not true grouping.
- Weaker UX for many windows.

**Effort:** Small

**Risk:** Low

## Recommended Action

Implemented Option 1: grouped sidebar sections keyed by `session_id`, with per-group header and preserved per-window actions.

## Technical Details

Affected files:
- `web/index.html`

## Resources

- `docs/TASK-10-multi-session-tmux.md`
- Review commits: `8d8d566`, `530e0c4`

## Acceptance Criteria

- [x] Sidebar renders windows grouped under their `session_id`
- [x] Active selection/unread counts still work with grouped rendering
- [x] `kill_window` and selection actions still resolve correct `session_id` + `window`
- [x] No regression in single-session mode

## Work Log

### 2026-02-17 - Review Finding

**By:** Codex

**Actions:**
- Compared TASK-10 Step 5 requirements with latest UI implementation.
- Verified current list rendering is flat and not grouped.

**Learnings:**
- Session-scoped keys are implemented, so grouping can be added without protocol changes.

### 2026-02-17 - Resolution

**By:** Codex

**Actions:**
- Added session-group container and header styles in `web/index.html`.
- Refactored sidebar rendering to group windows by `session_id`.
- Kept existing selection/unread/kill behavior by continuing to use `session_id::window` composite keys.

**Learnings:**
- Existing keying model was sufficient; only render-layer grouping changes were required.

## Notes
