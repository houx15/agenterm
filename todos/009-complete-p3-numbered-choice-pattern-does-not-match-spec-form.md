---
status: complete
priority: p3
issue_id: "009"
tags: [code-review, quality, parser]
dependencies: []
resolution: "Added PromptBracketedChoicePattern for [1-9] bracketed numeric prompts"
---

# Numbered Choice Prompt Pattern Diverges From Spec Form

## Problem Statement

Spec lists numbered-choice prompt detection as bracket form (`[1-9]`), but implementation instead detects multi-line menus (`1. ...`, `2) ...`) and may miss bracketed single-line prompts.

## Findings

- Spec includes `\[1-9\]` in prompt detection set (`TASK-03-output-parser.md:91`).
- `internal/parser/patterns.go:18` defines numbered choices as line-start numeric list items.
- `internal/parser/parser.go:142` requires at least two matching lines.
- Single-line prompts like `Select option [1-3]:` are not directly covered.

## Proposed Solutions

### Option 1: Add explicit bracketed-number prompt regex

**Approach:** Add detection for bracketed numeric ranges/choices and include it in prompt classification.

**Pros:**
- Aligns with spec
- Better menu coverage

**Cons:**
- Slightly broader prompt matching space

**Effort:** 30-90 minutes

**Risk:** Low

---

### Option 2: Update spec to current implementation

**Approach:** Clarify that only multi-line numbered menus are in scope.

**Pros:**
- No code change

**Cons:**
- Narrows intended UX feature set

**Effort:** 30 minutes

**Risk:** Low

---

### Option 3: Keep as-is

**Approach:** No changes.

**Pros:**
- Zero effort

**Cons:**
- Minor spec mismatch remains

**Effort:** 0

**Risk:** Low

## Recommended Action


## Technical Details

**Affected files:**
- `TASK-03-output-parser.md:91`
- `internal/parser/patterns.go:18`
- `internal/parser/parser.go:142`

**Related components:**
- Prompt quick-action generation

**Database changes (if any):**
- Migration needed? No
- New columns/tables? None

## Resources

- **Spec target:** `TASK-03-output-parser.md`

## Acceptance Criteria

- [ ] Bracketed numeric prompt forms are either supported or explicitly out of scope in spec
- [ ] Unit tests cover chosen numbered-choice behavior

## Work Log

### 2026-02-16 - Task 03 Implementation Review

**By:** Codex

**Actions:**
- Compared numbered-choice spec pattern against parser regex behavior
- Documented mismatch and options

**Learnings:**
- Pattern examples in spec should map 1:1 to executable tests

## Notes

- Nice-to-have alignment item.
