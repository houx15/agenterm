---
status: complete
priority: p2
issue_id: "032"
tags: [code-review, frontend, quality]
dependencies: []
---

# Guard Raw-to-Table Phase Conversion Against Invalid Items

## Problem Statement

The Playbook phases editor now supports `table` and `raw` modes, but the raw-to-table conversion only validates that parsed JSON is an array. It does not validate each array element shape. Invalid elements (for example `null`) can cause runtime crashes when the table renderer accesses phase fields.

## Findings

- `switchPhaseEditorMode('table')` in `frontend/src/pages/Settings.tsx:171` accepts any JSON array and calls `setDraftPhases(parsed)` without item-level validation.
- Table rendering reads `phase.name`, `phase.agent`, and other properties in `frontend/src/pages/Settings.tsx:492`. If an entry is `null`, this throws at render time.
- The issue is user-reachable from the UI by entering raw JSON such as `[null]` and switching back to table mode.

## Proposed Solutions

### Option 1: Strict Client-Side Validation Before Mode Switch

**Approach:** Validate each parsed item is an object with string fields (`name`, `agent`, `role`, `description`) before calling `setDraftPhases`.

**Pros:**
- Prevents runtime crashes at source.
- Gives immediate, actionable feedback in UI.
- Keeps table state type-safe.

**Cons:**
- Adds validation logic in the component.

**Effort:** Small

**Risk:** Low

---

### Option 2: Sanitize Parsed Items with Defaults

**Approach:** Convert each parsed item into a safe `PlaybookPhase`, coercing missing fields to `''` and dropping non-objects.

**Pros:**
- Very resilient to malformed user input.
- Minimizes user friction during manual edits.

**Cons:**
- Silent coercion can hide input mistakes.
- May produce unexpected edited output for advanced users.

**Effort:** Small

**Risk:** Medium

---

### Option 3: Keep Raw Mode Isolated

**Approach:** Disallow switching from raw to table unless parsing and validation pass; otherwise remain in raw mode with a clear error.

**Pros:**
- Clear contract between modes.
- Avoids partial/silent data transformations.

**Cons:**
- Requires complete validation pass.
- Slightly stricter UX for advanced editing.

**Effort:** Small

**Risk:** Low

## Recommended Action

Use Option 1 with explicit validation and error messaging.

## Technical Details

**Affected files:**
- `frontend/src/pages/Settings.tsx:171`
- `frontend/src/pages/Settings.tsx:492`

**Related components:**
- Settings Playbook editor (`table` and `raw` phase modes)

**Database changes (if any):**
- No

## Resources

- **Spec:** `docs/TASK-18-playbook-system.md`
- **Related todo:** `todos/031-complete-p2-playbook-phases-editor-still-json-textarea.md`

## Acceptance Criteria

- [ ] Raw-to-table switch rejects arrays containing invalid phase entries.
- [ ] UI shows a clear validation error instead of crashing.
- [ ] Table renderer only receives valid `PlaybookPhase[]` data.
- [ ] Manual test covers malformed raw input (`[null]`, `[1]`, `[{}]`).

## Work Log

### 2026-02-18 - Initial Discovery

**By:** Codex

**Actions:**
- Reviewed `feature/playbook-system` against `docs/TASK-18-playbook-system.md`.
- Inspected phase mode toggle and table render path.
- Identified user-triggerable crash path from malformed raw JSON array items.

**Learnings:**
- Array-level validation is insufficient for mode-switch safety.
- Table mode needs guaranteed shape-valid phase objects.

### 2026-02-18 - Resolution

**By:** Codex

**Actions:**
- Added `parseRawPhases` validation in `frontend/src/pages/Settings.tsx`.
- Enforced per-item validation (`name`, `agent`, `role`) before Raw -> Table switch.
- Reused the same validation in raw-mode save path for consistent behavior.
- Ran `npm --prefix frontend run build` successfully.

**Learnings:**
- Shared validation logic avoids drift between mode-switch and save behavior.

## Notes

- Keep validation behavior consistent with save-time playbook validation to avoid conflicting UX.
