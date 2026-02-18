---
status: complete
priority: p2
issue_id: "031"
tags: [code-review, quality, frontend, ux, spec-compliance]
dependencies: []
---

# Playbook Phases Editor Still Uses JSON Textarea

## Problem Statement

The playbook settings UI is expected to be form-first so users can enter values in blanks instead of editing raw config text. The current implementation still requires users to edit `phases` as a JSON array in a textarea, which is error-prone and inconsistent with that UX direction.

## Findings

- The playbook form uses regular inputs for top-level fields, but phases are edited through a raw textarea labeled `Phase Editor (JSON array)` in `frontend/src/pages/Settings.tsx:408`.
- Save flow depends on `JSON.parse(phasesEditor)` in `frontend/src/pages/Settings.tsx:193`, so malformed JSON blocks saving.
- This creates a mixed UI model (structured fields + raw JSON) and still exposes config-like editing for the most complex part of a playbook.

## Proposed Solutions

### Option 1: Structured phase rows (recommended)

**Approach:** Replace JSON textarea with a repeatable list of phase rows (`name`, `agent`, `role`, `description`) plus add/remove/reorder controls.

**Pros:**
- Aligns with blank-form UX direction.
- Eliminates JSON parse errors from normal usage.
- Easier validation and inline field-level feedback.

**Cons:**
- Requires more UI state management.

**Effort:** Medium

**Risk:** Low

---

### Option 2: Keep JSON as advanced mode only

**Approach:** Default to form rows and offer optional “Advanced JSON” toggle.

**Pros:**
- Preserves power-user editing path.
- Reduces migration risk.

**Cons:**
- More UI complexity.
- Still carries JSON parsing branch.

**Effort:** Medium

**Risk:** Medium

---

### Option 3: Auto-generated phase text helpers

**Approach:** Keep textarea but add template insertion and schema hints.

**Pros:**
- Smallest code change.

**Cons:**
- Still config-text editing, does not meet desired UX.

**Effort:** Small

**Risk:** Medium

## Recommended Action

Implement Option 1 and fully remove JSON textarea editing from the default playbook settings experience.

## Technical Details

**Affected files:**
- `frontend/src/pages/Settings.tsx:46`
- `frontend/src/pages/Settings.tsx:193`
- `frontend/src/pages/Settings.tsx:408`

## Resources

- Task spec: `docs/TASK-18-playbook-system.md`
- Prior related finding: `todos/028-complete-p2-settings-playbook-editor-not-yaml.md`

## Acceptance Criteria

- [x] Playbook phases can be added, edited, and removed through structured form controls.
- [x] Save path supports table mode without requiring free-form JSON editing.
- [x] Existing playbooks load into the structured phase editor without data loss.
- [x] API payload shape remains unchanged (`phases` array of objects).

## Work Log

### 2026-02-17 - Review finding created

**By:** Codex

**Actions:**
- Re-reviewed branch against TASK-18 and current product direction.
- Verified phases are still edited via JSON textarea and parsed at save time.
- Logged this as the remaining UX/spec-compliance gap.

**Learnings:**
- Backend and API are in good shape; primary remaining issue is frontend phase-editing UX.

### 2026-02-18 - Implemented table + raw dual-mode editor

**By:** Codex

**Actions:**
- Added a default table editor for playbook phases with row inputs and add/remove controls.
- Added editor mode toggle (`Table` / `Raw JSON`) so users can still edit raw payload when needed.
- Updated save logic to use structured phases in table mode and validated JSON only in raw mode.
- Added frontend styles for the phase table layout and mode switch area.

**Learnings:**
- Dual-mode editing gives safer default UX while retaining an escape hatch for advanced/manual edits.
