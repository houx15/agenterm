---
status: complete
priority: p2
issue_id: "011"
tags: [code-review, frontend, ui, quality]
dependencies: []
resolution: "Added .message.normal CSS rule matching .message.output styling"
---

# Normal Class Messages Are Not Styled As Terminal Output

## Problem Statement

Parser emits `class: "normal"`, but frontend style mapping expects `output`; normal terminal output messages miss the intended bubble style.

## Findings

- Parser class contract uses `normal` (`internal/parser/types.go:8`).
- Frontend sets `className = msg.class || 'output'` (`web/index.html:774`).
- CSS styles define `.message.output` but not `.message.normal` (`web/index.html:296`).
- Result: normal output is rendered without the required terminal-output bubble styling.

## Proposed Solutions

### Option 1: Map `normal` to `output` in renderer

**Approach:** In render logic, translate `msg.class === 'normal'` to `output` class.

**Pros:**
- Minimal change
- Preserves current CSS naming

**Cons:**
- Adds small mapping branch in JS

**Effort:** 15-30 minutes

**Risk:** Low

---

### Option 2: Add `.message.normal` CSS alias

**Approach:** Duplicate/alias `.message.output` styles for `.message.normal`.

**Pros:**
- No JS change
- Explicit support for parser class values

**Cons:**
- Potential style duplication

**Effort:** 15-30 minutes

**Risk:** Low

---

### Option 3: Rename parser output class to `output`

**Approach:** Change backend class vocabulary.

**Pros:**
- Frontend alignment via backend change

**Cons:**
- Breaks existing parser/task contract and wider message semantics

**Effort:** 1-2 hours

**Risk:** Medium

## Recommended Action


## Technical Details

**Affected files:**
- `web/index.html:774`
- `web/index.html:296`
- `internal/parser/types.go:8`

**Related components:**
- Message bubble rendering fidelity

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Spec target:** `TASK-05-frontend-ui.md`

## Acceptance Criteria

- [ ] `ClassNormal` messages render with terminal-output bubble style
- [ ] Error/prompt/code/system styles remain unchanged

## Work Log

### 2026-02-16 - Task 05 Implementation Review

**By:** Codex

**Actions:**
- Compared parser class values with frontend class-to-style mapping
- Identified mismatch between `normal` and `.output`

**Learnings:**
- UI class vocab should be normalized at module boundaries

## Notes

- Important UX correctness issue.
