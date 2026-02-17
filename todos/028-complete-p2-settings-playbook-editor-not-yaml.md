---
status: complete
priority: p2
issue_id: "028"
tags: [code-review, quality, frontend, spec-compliance]
dependencies: []
---

# Settings Playbook Editor Is Not YAML

## Problem Statement

TASK-18 requires a YAML editor with syntax highlighting for advanced playbook editing, but the Settings UI currently exposes a JSON textarea for phases only. This creates a spec mismatch and makes it harder to round-trip full YAML playbook configs.

## Findings

- Spec explicitly calls for `YAML editor with syntax highlighting` in `docs/TASK-18-playbook-system.md:75`.
- UI label and behavior are JSON-based in `frontend/src/pages/Settings.tsx:408`.
- The current editor only supports `phases` as JSON text, not full YAML document editing.

## Proposed Solutions

### Option 1: Monaco YAML editor (recommended)

**Approach:** Add Monaco editor configured for YAML mode and edit a full playbook YAML document, with parse/validation on save.

**Pros:**
- Meets spec exactly (YAML + syntax highlighting)
- Best editing UX for advanced users

**Cons:**
- Adds dependency and bundle size
- Requires YAML<->API payload mapping

**Effort:** Medium

**Risk:** Medium

---

### Option 2: CodeMirror YAML editor

**Approach:** Use CodeMirror + YAML language package with a smaller footprint.

**Pros:**
- Syntax highlighting support
- Lighter than Monaco

**Cons:**
- Slightly more custom integration work
- Fewer built-in language services than Monaco

**Effort:** Medium

**Risk:** Medium

---

### Option 3: Minimal YAML textarea + server validation

**Approach:** Keep textarea but switch to full YAML text and validate server-side.

**Pros:**
- Minimal implementation cost
- Restores YAML round-trip

**Cons:**
- No syntax highlighting (still partially misses spec)
- Weaker UX

**Effort:** Small

**Risk:** Low

## Recommended Action

Keep the form-based editor approach (structured blanks) as the product direction for this project. Do not introduce YAML/JSON code-editor UX at this time.

## Technical Details

**Affected files:**
- `frontend/src/pages/Settings.tsx:408`
- `docs/TASK-18-playbook-system.md:75`

## Resources

- Branch: `feature/playbook-system`
- Task spec: `docs/TASK-18-playbook-system.md`

## Acceptance Criteria

- [x] Product direction confirmed: settings use structured form inputs instead of raw config-file editing
- [x] Existing create/update/delete behavior still works

## Work Log

### 2026-02-17 - Review finding created

**By:** Codex

**Actions:**
- Compared TASK-18 editor requirement to implemented UI.
- Verified editor label and behavior are JSON-based, not YAML.

**Learnings:**
- Core CRUD exists, but advanced editing requirement is not fully implemented.

### 2026-02-17 - Product decision recorded

**By:** Codex

**Actions:**
- User confirmed that YAML vs JSON is acceptable and preferred form-based blanks for frontend UX.
- Closed this finding as an accepted product-direction decision, not an implementation defect.

**Learnings:**
- This branch should prioritize guided form editing over raw configuration editing.

## Notes

- This is a spec-compliance gap, not a backend correctness failure.
